from __future__ import annotations

import hashlib
import threading
import uuid
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional

import numpy as np

from qqa_llm.core.config import Settings
from qqa_llm.core.errors import ValidationError
from qqa_llm.domain.models import IndexingResult, KnowledgeChunk, KnowledgeDocument, ScopeRef
from qqa_llm.index.base import IndexBackend
from qqa_llm.ingest.chunk import ChunkBuilder
from qqa_llm.ingest.normalize import DocumentNormalizer
from qqa_llm.ingest.providers.feishu import FeishuPayloadParser
from qqa_llm.ingest.providers.yuque import YuquePayloadParser
from qqa_llm.inference.embeddings import EmbeddingClient


class IndexingService:
    """Build retrieval-ready scope bundles from canonical document snapshots."""

    def __init__(
        self,
        settings: Settings,
        *,
        index_backend: IndexBackend,
        index_job_repo,
        normalizer: DocumentNormalizer,
        chunk_builder: ChunkBuilder,
        embedding_client: EmbeddingClient,
    ) -> None:
        self.settings = settings
        self.index_backend = index_backend
        self.index_job_repo = index_job_repo
        self.normalizer = normalizer
        self.chunk_builder = chunk_builder
        self.embedding_client = embedding_client
        self.yuque_parser = YuquePayloadParser()
        self.feishu_parser = FeishuPayloadParser()
        self._active_scope_jobs: set[tuple[str, str]] = set()
        self._active_scope_lock = threading.Lock()
        self._active_scope_workers: set[tuple[str, str]] = set()
        self._active_worker_lock = threading.Lock()

    def upsert_documents(
        self,
        *,
        user_id: str,
        scope_id: str,
        scope_name: str,
        scope_type: str,
        provider: str,
        sync_mode: str = "replace",
        run_async: bool = False,
        documents: List[Dict[str, Any]],
    ) -> IndexingResult:
        if not user_id or not scope_id:
            raise ValidationError("user_id and scope_id are required")
        sync_mode = (sync_mode or "replace").strip().lower() or "replace"
        if sync_mode not in {"replace", "merge"}:
            raise ValidationError("sync_mode must be 'replace' or 'merge'")

        created_at = datetime.now(timezone.utc).isoformat()
        job_id = str(uuid.uuid4())
        if run_async:
            payload = self._job_payload(
                job_id=job_id,
                operation="upsert_scope",
                job_type="upsert_batch",
                status="queued",
                user_id=user_id,
                scope_id=scope_id,
                scope_name=scope_name,
                scope_type=scope_type,
                provider=provider,
                sync_mode=sync_mode,
                created_at=created_at,
                updated_at=created_at,
            )
            payload["task_kind"] = "upsert_documents"
            payload["task_payload"] = {
                "user_id": user_id,
                "scope_id": scope_id,
                "scope_name": scope_name,
                "scope_type": scope_type,
                "provider": provider,
                "sync_mode": sync_mode,
                "documents": list(documents),
                "created_at": created_at,
            }
            self.index_job_repo.upsert(job_id, payload)
            self._start_scope_worker(user_id=user_id, scope_id=scope_id)
            return IndexingResult(
                document_count=0,
                chunk_count=0,
                index_version="",
                job_id=job_id,
                documents=[],
                changed_documents={"added": 0, "updated": 0, "deleted": 0, "unchanged": 0},
                status="queued",
                execution_mode="async",
            )
        return self._run_upsert_job(
            job_id=job_id,
            user_id=user_id,
            scope_id=scope_id,
            scope_name=scope_name,
            scope_type=scope_type,
            provider=provider,
            sync_mode=sync_mode,
            documents=documents,
            created_at=created_at,
        )

    def sync_source(
        self,
        *,
        user_id: str,
        scope_id: str,
        name: str,
        provider: str,
        run_async: bool = False,
        raw_payload: Dict[str, Any],
    ) -> IndexingResult:
        # Go still owns fetching opaque provider payloads. Python only parses
        # them and routes everything through the same indexing pipeline.
        provider = (provider or "yuque").strip().lower() or "yuque"
        if provider == "yuque":
            documents = self.yuque_parser.parse(raw_payload)
        elif provider == "feishu":
            documents = self.feishu_parser.parse(
                raw_payload,
                entry_type=str(raw_payload.get("entry_type") or "docx"),
                entry_token=str(raw_payload.get("entry_token") or ""),
            )
        else:
            raise ValidationError(f"unsupported provider: {provider}")
        return self.upsert_documents(
            user_id=user_id,
            scope_id=scope_id,
            scope_name=name or scope_id,
            scope_type="knowledge_connection",
            provider=provider,
            sync_mode="replace",
            run_async=run_async,
            documents=documents,
        )

    def delete_scope(self, *, user_id: str, scope_id: str) -> None:
        self.index_backend.delete_scope(user_id, scope_id)

    def resume_pending_jobs(self) -> None:
        """Resume async indexing work that was persisted before a restart."""

        pending_jobs = self.index_job_repo.list(statuses=["queued", "running"], limit=500)
        scope_keys: set[tuple[str, str]] = set()
        for payload in pending_jobs:
            user_id = str(payload.get("user_id") or "").strip()
            scope_id = str(payload.get("scope_id") or "").strip()
            job_id = str(payload.get("job_id") or "").strip()
            if not user_id or not scope_id or not job_id:
                continue
            if str(payload.get("status") or "") == "running":
                recovered = dict(payload)
                recovered["status"] = "queued"
                recovered["updated_at"] = datetime.now(timezone.utc).isoformat()
                recovered["finished_at"] = ""
                recovered["recovered_from_status"] = "running"
                self.index_job_repo.upsert(job_id, recovered)
            scope_keys.add((user_id, scope_id))
        for user_id, scope_id in scope_keys:
            self._start_scope_worker(user_id=user_id, scope_id=scope_id)

    def _run_upsert_job(
        self,
        *,
        job_id: str,
        user_id: str,
        scope_id: str,
        scope_name: str,
        scope_type: str,
        provider: str,
        sync_mode: str,
        documents: List[Dict[str, Any]],
        created_at: str,
    ) -> IndexingResult:
        started_at = datetime.now(timezone.utc).isoformat()
        scope_key = (user_id, scope_id)
        with self._active_scope_lock:
            if scope_key in self._active_scope_jobs:
                self.index_job_repo.upsert(
                    job_id,
                    self._job_payload(
                        job_id=job_id,
                        operation="upsert_scope",
                        job_type="upsert_batch",
                        status="failed",
                        user_id=user_id,
                        scope_id=scope_id,
                        scope_name=scope_name,
                        scope_type=scope_type,
                        provider=provider,
                        sync_mode=sync_mode,
                        created_at=created_at,
                        updated_at=started_at,
                        started_at=started_at,
                        finished_at=started_at,
                        error_code="SCOPE_BUSY",
                        error_message="another indexing job is already running for this scope",
                    ),
                )
                raise ValidationError("another indexing job is already running for this scope")
            self._active_scope_jobs.add(scope_key)

        self.index_job_repo.upsert(
            job_id,
            self._job_payload(
                job_id=job_id,
                operation="upsert_scope",
                job_type="upsert_batch",
                status="running",
                user_id=user_id,
                scope_id=scope_id,
                scope_name=scope_name,
                scope_type=scope_type,
                provider=provider,
                sync_mode=sync_mode,
                created_at=created_at,
                updated_at=started_at,
                started_at=started_at,
            ),
        )

        normalized_docs: List[KnowledgeDocument] = []
        chunk_records: List[KnowledgeChunk] = []
        doc_summaries: List[Dict[str, Any]] = []
        existing_bundle = self.index_backend.load_scope(user_id, scope_id)
        existing_docs_by_external: Dict[str, Dict[str, Any]] = {}
        existing_chunks_by_doc: Dict[str, List[Any]] = {}
        change_counts = {"added": 0, "updated": 0, "deleted": 0, "unchanged": 0}
        if existing_bundle is not None:
            for doc in list(existing_bundle.get("documents") or []):
                external_id = str(doc.get("external_id") or "").strip()
                if external_id:
                    existing_docs_by_external[external_id] = dict(doc)
            existing_chunks = list(existing_bundle.get("chunks") or [])
            existing_embeddings = existing_bundle.get("embeddings")
            if existing_embeddings is None:
                existing_embeddings = np.zeros((0, self.settings.embedding_dim), dtype=np.float32)
            for index, chunk in enumerate(existing_chunks):
                document_id = str(chunk.get("document_id") or "").strip()
                if not document_id:
                    continue
                if index < len(existing_embeddings):
                    embedding_row = existing_embeddings[index]
                else:
                    embedding_row = np.zeros((self.settings.embedding_dim,), dtype=np.float32)
                existing_chunks_by_doc.setdefault(document_id, []).append((dict(chunk), embedding_row))

        rebuilt_chunks: List[KnowledgeChunk] = []
        preserved_chunks: List[KnowledgeChunk] = []
        preserved_embedding_rows: List[np.ndarray] = []
        incoming_external_ids = set()
        final_external_ids = set()

        try:
            for raw in documents:
                raw_body = str(raw.get("raw_body") or "").strip()
                provided_normalized_body = str(raw.get("normalized_body") or "").strip()
                normalized_body = provided_normalized_body or self.normalizer.normalize_body(raw_body)
                if not normalized_body:
                    continue
                external_id = str(raw.get("doc_id") or raw.get("external_id") or raw.get("doc_ref") or "").strip()
                if not external_id:
                    continue
                incoming_external_ids.add(external_id)
                content_hash = hashlib.sha256(normalized_body.encode("utf-8")).hexdigest()
                document_id = hashlib.sha256(f"{scope_id}:{external_id}".encode("utf-8")).hexdigest()
                version_id = hashlib.sha256(f"{document_id}:{content_hash}".encode("utf-8")).hexdigest()
                existing_doc = existing_docs_by_external.get(external_id)
                is_unchanged = (
                    existing_doc is not None
                    and str(existing_doc.get("content_hash") or "") == content_hash
                    and str(existing_doc.get("version_id") or "") != ""
                )
                if is_unchanged:
                    version_id = str(existing_doc.get("version_id") or version_id)
                    change_counts["unchanged"] += 1
                elif existing_doc is None:
                    change_counts["added"] += 1
                else:
                    change_counts["updated"] += 1

                document = KnowledgeDocument(
                    id=document_id,
                    scope_id=scope_id,
                    user_id=user_id,
                    provider=str(raw.get("provider") or provider).strip() or provider,
                    external_id=external_id,
                    repo=str(raw.get("repo") or scope_name).strip() or scope_name,
                    title=str(raw.get("title") or external_id).strip() or external_id,
                    doc_ref=str(raw.get("doc_ref") or external_id).strip() or external_id,
                    source_url=str(raw.get("source_url") or "").strip(),
                    source_updated_at=self.normalizer.normalize_updated_at(str(raw.get("updated_at") or "").strip()),
                    content_hash=content_hash,
                    version_id=version_id,
                    raw_body=raw_body,
                    normalized_body=normalized_body,
                )
                normalized_docs.append(document)
                final_external_ids.add(external_id)

                if is_unchanged:
                    preserved = existing_chunks_by_doc.get(document.id) or []
                    for chunk_payload, embedding_row in preserved:
                        preserved_chunks.append(self._chunk_from_payload(chunk_payload))
                        preserved_embedding_rows.append(np.asarray(embedding_row, dtype=np.float32))
                    doc_summaries.append(self._document_summary(document, len(preserved)))
                    continue

                chunks = self.chunk_builder.build_chunks(document)
                if not chunks:
                    continue
                rebuilt_chunks.extend(chunks)
                doc_summaries.append(self._document_summary(document, len(chunks)))

            if sync_mode == "merge":
                for external_id, payload in existing_docs_by_external.items():
                    if external_id in incoming_external_ids:
                        continue
                    carried_document = self._document_from_payload(payload)
                    normalized_docs.append(carried_document)
                    final_external_ids.add(external_id)
                    change_counts["unchanged"] += 1
                    preserved = existing_chunks_by_doc.get(carried_document.id) or []
                    for chunk_payload, embedding_row in preserved:
                        preserved_chunks.append(self._chunk_from_payload(chunk_payload))
                        preserved_embedding_rows.append(np.asarray(embedding_row, dtype=np.float32))
                    doc_summaries.append(self._document_summary(carried_document, len(preserved)))
            else:
                change_counts["deleted"] = sum(
                    1 for external_id in existing_docs_by_external if external_id not in final_external_ids
                )

            rebuilt_embeddings = (
                self.embedding_client.embed_texts([chunk.lexical_text for chunk in rebuilt_chunks])
                if rebuilt_chunks
                else np.zeros((0, self.settings.embedding_dim), dtype=np.float32)
            )
            chunk_records = preserved_chunks + rebuilt_chunks
            if preserved_embedding_rows and rebuilt_embeddings.size:
                embeddings = np.vstack([np.asarray(preserved_embedding_rows, dtype=np.float32), rebuilt_embeddings])
            elif preserved_embedding_rows:
                embeddings = np.asarray(preserved_embedding_rows, dtype=np.float32)
            else:
                embeddings = rebuilt_embeddings

            indexed_at = datetime.now(timezone.utc).isoformat()
            scope = ScopeRef(
                user_id=user_id,
                scope_id=scope_id,
                scope_type=scope_type,
                scope_external_id=scope_id,
                provider=provider,
                name=scope_name or scope_id,
                latest_index_version=indexed_at,
                document_count=len(normalized_docs),
                chunk_count=len(chunk_records),
                last_indexed_at=indexed_at,
                created_at=indexed_at,
                updated_at=indexed_at,
            )
            compat_meta = {
                "user_id": user_id,
                "connection_id": scope_id,
                "connection_meta": {"id": scope_id, "name": scope_name, "provider": provider},
                "sync_mode": sync_mode,
                "changed_documents": dict(change_counts),
                "document_count": len(normalized_docs),
                "chunk_count": len(chunk_records),
                "index_version": indexed_at,
                "documents": doc_summaries,
                "updated_at": indexed_at,
            }
            self.index_backend.save_scope(scope, normalized_docs, chunk_records, embeddings, compat_meta)
            self.index_job_repo.upsert(
                job_id,
                self._job_payload(
                    job_id=job_id,
                    operation="upsert_scope",
                    job_type="upsert_batch",
                    status="completed",
                    user_id=user_id,
                    scope_id=scope_id,
                    scope_name=scope_name,
                    scope_type=scope_type,
                    provider=provider,
                    sync_mode=sync_mode,
                    document_count=len(normalized_docs),
                    chunk_count=len(chunk_records),
                    index_version=indexed_at,
                    changed_documents=dict(change_counts),
                    created_at=created_at,
                    updated_at=datetime.now(timezone.utc).isoformat(),
                    started_at=started_at,
                    finished_at=datetime.now(timezone.utc).isoformat(),
                ),
            )
            return IndexingResult(
                document_count=len(normalized_docs),
                chunk_count=len(chunk_records),
                index_version=indexed_at,
                job_id=job_id,
                documents=doc_summaries,
                changed_documents=change_counts,
                status="completed",
                execution_mode="sync",
            )
        except Exception as exc:
            self.index_job_repo.upsert(
                job_id,
                self._job_payload(
                    job_id=job_id,
                    operation="upsert_scope",
                    job_type="upsert_batch",
                    status="failed",
                    user_id=user_id,
                    scope_id=scope_id,
                    scope_name=scope_name,
                    scope_type=scope_type,
                    provider=provider,
                    sync_mode=sync_mode,
                    document_count=len(normalized_docs),
                    chunk_count=len(chunk_records),
                    changed_documents=dict(change_counts),
                    created_at=created_at,
                    updated_at=datetime.now(timezone.utc).isoformat(),
                    started_at=started_at,
                    finished_at=datetime.now(timezone.utc).isoformat(),
                    error_code="INDEXING_ERROR",
                    error_message=str(exc),
                ),
            )
            raise
        finally:
            with self._active_scope_lock:
                self._active_scope_jobs.discard(scope_key)

    def _start_scope_worker(self, *, user_id: str, scope_id: str) -> None:
        scope_key = (user_id, scope_id)
        with self._active_worker_lock:
            if scope_key in self._active_scope_workers:
                return
            self._active_scope_workers.add(scope_key)
        worker = threading.Thread(
            target=self._scope_worker_loop,
            kwargs={"user_id": user_id, "scope_id": scope_id},
            daemon=True,
        )
        worker.start()

    def _scope_worker_loop(self, *, user_id: str, scope_id: str) -> None:
        scope_key = (user_id, scope_id)
        try:
            while True:
                payload = self._claim_next_scope_job(user_id=user_id, scope_id=scope_id)
                if payload is None:
                    break
                self._execute_claimed_job(payload)
        finally:
            with self._active_worker_lock:
                self._active_scope_workers.discard(scope_key)
            if self.index_job_repo.list(statuses=["queued"], user_id=user_id, scope_id=scope_id, limit=1):
                self._start_scope_worker(user_id=user_id, scope_id=scope_id)

    def _claim_next_scope_job(self, *, user_id: str, scope_id: str) -> Optional[Dict[str, Any]]:
        queued_jobs = self.index_job_repo.list(statuses=["queued"], user_id=user_id, scope_id=scope_id, limit=1)
        if not queued_jobs:
            return None
        payload = dict(queued_jobs[0])
        now = datetime.now(timezone.utc).isoformat()
        payload["status"] = "running"
        payload["updated_at"] = now
        payload["started_at"] = str(payload.get("started_at") or now)
        self.index_job_repo.upsert(str(payload.get("job_id") or ""), payload)
        return payload

    def _execute_claimed_job(self, payload: Dict[str, Any]) -> None:
        task_kind = str(payload.get("task_kind") or "").strip()
        task_payload = dict(payload.get("task_payload") or {})
        if task_kind != "upsert_documents":
            failed = dict(payload)
            failed["status"] = "failed"
            failed["updated_at"] = datetime.now(timezone.utc).isoformat()
            failed["finished_at"] = failed["updated_at"]
            failed["error_code"] = "UNSUPPORTED_TASK"
            failed["error_message"] = f"unsupported indexing task kind: {task_kind or 'unknown'}"
            self.index_job_repo.upsert(str(payload.get("job_id") or ""), failed)
            return
        self._run_upsert_job(
            job_id=str(payload.get("job_id") or ""),
            user_id=str(task_payload.get("user_id") or ""),
            scope_id=str(task_payload.get("scope_id") or ""),
            scope_name=str(task_payload.get("scope_name") or ""),
            scope_type=str(task_payload.get("scope_type") or ""),
            provider=str(task_payload.get("provider") or ""),
            sync_mode=str(task_payload.get("sync_mode") or "replace"),
            documents=list(task_payload.get("documents") or []),
            created_at=str(
                task_payload.get("created_at")
                or payload.get("created_at")
                or datetime.now(timezone.utc).isoformat()
            ),
        )

    def _chunk_from_payload(self, payload: Dict[str, Any]) -> KnowledgeChunk:
        return KnowledgeChunk(
            id=str(payload.get("id") or ""),
            scope_id=str(payload.get("scope_id") or ""),
            user_id=str(payload.get("user_id") or ""),
            document_id=str(payload.get("document_id") or ""),
            version_id=str(payload.get("version_id") or ""),
            provider=str(payload.get("provider") or ""),
            repo=str(payload.get("repo") or ""),
            title=str(payload.get("title") or ""),
            heading_path=str(payload.get("heading_path") or ""),
            source_url=str(payload.get("source_url") or ""),
            updated_at=str(payload.get("updated_at") or ""),
            chunk_index=int(payload.get("chunk_index") or 0),
            content=str(payload.get("content") or ""),
            lexical_text=str(payload.get("lexical_text") or ""),
            token_count=int(payload.get("token_count") or 0),
        )

    def _document_from_payload(self, payload: Dict[str, Any]) -> KnowledgeDocument:
        return KnowledgeDocument(
            id=str(payload.get("id") or ""),
            scope_id=str(payload.get("scope_id") or ""),
            user_id=str(payload.get("user_id") or ""),
            provider=str(payload.get("provider") or ""),
            external_id=str(payload.get("external_id") or payload.get("doc_id") or payload.get("doc_ref") or ""),
            repo=str(payload.get("repo") or ""),
            title=str(payload.get("title") or ""),
            doc_ref=str(payload.get("doc_ref") or payload.get("external_id") or ""),
            source_url=str(payload.get("source_url") or ""),
            source_updated_at=str(payload.get("source_updated_at") or payload.get("updated_at") or ""),
            content_hash=str(payload.get("content_hash") or ""),
            version_id=str(payload.get("version_id") or ""),
            raw_body=str(payload.get("raw_body") or ""),
            normalized_body=str(payload.get("normalized_body") or ""),
        )

    def _document_summary(self, document: KnowledgeDocument, chunk_count: int) -> Dict[str, Any]:
        return {
            "doc_id": document.external_id,
            "title": document.title,
            "repo": document.repo,
            "doc_ref": document.doc_ref,
            "source_url": document.source_url,
            "updated_at": document.source_updated_at,
            "chunk_count": chunk_count,
        }

    def _job_payload(
        self,
        *,
        job_id: str,
        operation: str,
        job_type: str,
        status: str,
        user_id: str,
        scope_id: str,
        scope_name: str,
        scope_type: str,
        provider: str,
        sync_mode: str,
        created_at: str,
        updated_at: str,
        document_count: int = 0,
        chunk_count: int = 0,
        index_version: str = "",
        changed_documents: Dict[str, int] | None = None,
        started_at: str = "",
        finished_at: str = "",
        error_code: str = "",
        error_message: str = "",
    ) -> Dict[str, Any]:
        return {
            "job_id": job_id,
            "operation": operation,
            "job_type": job_type,
            "status": status,
            "user_id": user_id,
            "scope_id": scope_id,
            "scope_name": scope_name,
            "scope_type": scope_type,
            "provider": provider,
            "sync_mode": sync_mode,
            "document_count": document_count,
            "chunk_count": chunk_count,
            "index_version": index_version,
            "changed_documents": changed_documents or {"added": 0, "updated": 0, "deleted": 0, "unchanged": 0},
            "created_at": created_at,
            "updated_at": updated_at,
            "started_at": started_at,
            "finished_at": finished_at,
            "error_code": error_code,
            "error_message": error_message,
            "error": error_message,
        }
