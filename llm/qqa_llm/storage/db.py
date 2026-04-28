from __future__ import annotations

import json
import re
from datetime import datetime, timedelta, timezone
from typing import Any, Dict, Iterable, List, Optional

import numpy as np

from qqa_llm.core.errors import IndexingError
from qqa_llm.domain.models import KnowledgeChunk, KnowledgeDocument, ScopeRef

try:
    import psycopg
    from psycopg.rows import dict_row
except ImportError:  # pragma: no cover - exercised only when postgres backend is enabled
    psycopg = None
    dict_row = None


class DatabaseAdapter:
    """Postgres adapter for scope bundles, jobs, and traces.

    The design target is Postgres plus pgvector. In local environments where
    the pgvector extension is not installed, the adapter degrades to JSON
    embedding storage so we can still verify the real database path end to end.
    Health checks surface that degraded mode explicitly.
    """

    _SCHEMA_RE = re.compile(r"^[A-Za-z_][A-Za-z0-9_]*$")
    _TOKEN_RE = re.compile(r"[A-Za-z0-9_]+|[\u4e00-\u9fff]")

    def __init__(
        self,
        *,
        dsn: str,
        schema: str,
        connect_timeout: int,
        embedding_dim: int,
    ) -> None:
        self.dsn = dsn.strip()
        self.schema = self._validate_schema(schema)
        self.connect_timeout = max(connect_timeout, 1)
        self.embedding_dim = max(embedding_dim, 1)
        self._schema_ready = False
        self._vector_mode = "unknown"

    def is_configured(self) -> bool:
        return bool(self.dsn)

    def dependency_ready(self) -> bool:
        return psycopg is not None

    def health_status(self) -> Dict[str, object]:
        if not self.is_configured():
            return {
                "status": "not_configured",
                "message": "Postgres DSN is not configured.",
                "schema": self.schema,
                "vector_mode": self._vector_mode,
            }
        if not self.dependency_ready():
            return {
                "status": "missing_dependency",
                "message": "psycopg is not installed.",
                "schema": self.schema,
                "vector_mode": self._vector_mode,
            }
        try:
            self.ensure_schema()
            with self._connect() as conn:
                with conn.cursor() as cur:
                    cur.execute("SELECT current_database() AS db, current_user AS user_name")
                    row = cur.fetchone() or {}
            status = "ok" if self._vector_mode == "native" else "degraded"
            message = "Postgres connection is ready."
            if self._vector_mode != "native":
                message = "Postgres is ready, but pgvector is unavailable so JSON embedding fallback is active."
            return {
                "status": status,
                "message": message,
                "schema": self.schema,
                "database": row.get("db", ""),
                "user": row.get("user_name", ""),
                "vector_mode": self._vector_mode,
            }
        except Exception as exc:  # pragma: no cover - depends on external db
            return {
                "status": "error",
                "message": str(exc),
                "schema": self.schema,
                "vector_mode": self._vector_mode,
            }

    def ensure_schema(self) -> None:
        if self._schema_ready:
            return
        with self._connect() as conn:
            with conn.cursor() as cur:
                self._vector_mode = self._detect_vector_mode(cur)
                cur.execute(f"CREATE SCHEMA IF NOT EXISTS {self._schema_name()}")
                cur.execute(
                    f"""
                    CREATE TABLE IF NOT EXISTS {self._table_name("scopes")} (
                        user_id TEXT NOT NULL,
                        scope_id TEXT NOT NULL,
                        scope_type TEXT NOT NULL,
                        scope_external_id TEXT NOT NULL,
                        provider TEXT NOT NULL,
                        name TEXT NOT NULL,
                        status TEXT NOT NULL,
                        latest_index_version TEXT NOT NULL,
                        document_count INTEGER NOT NULL DEFAULT 0,
                        chunk_count INTEGER NOT NULL DEFAULT 0,
                        last_indexed_at TEXT NOT NULL DEFAULT '',
                        created_at TEXT NOT NULL DEFAULT '',
                        updated_at TEXT NOT NULL DEFAULT '',
                        compat_meta JSONB NOT NULL DEFAULT '{{}}'::jsonb,
                        PRIMARY KEY (user_id, scope_id)
                    )
                    """
                )
                cur.execute(
                    f"""
                    CREATE TABLE IF NOT EXISTS {self._table_name("documents")} (
                        id TEXT PRIMARY KEY,
                        user_id TEXT NOT NULL,
                        scope_id TEXT NOT NULL,
                        provider TEXT NOT NULL,
                        external_id TEXT NOT NULL,
                        repo TEXT NOT NULL,
                        title TEXT NOT NULL,
                        doc_ref TEXT NOT NULL,
                        source_url TEXT NOT NULL,
                        source_updated_at TEXT NOT NULL,
                        content_hash TEXT NOT NULL,
                        version_id TEXT NOT NULL,
                        raw_body TEXT NOT NULL,
                        normalized_body TEXT NOT NULL,
                        FOREIGN KEY (user_id, scope_id)
                            REFERENCES {self._table_name("scopes")} (user_id, scope_id)
                            ON DELETE CASCADE
                    )
                    """
                )
                cur.execute(
                    f'ALTER TABLE {self._table_name("documents")} ADD COLUMN IF NOT EXISTS latest_version_id TEXT NOT NULL DEFAULT \'\''
                )
                cur.execute(
                    f'ALTER TABLE {self._table_name("documents")} ADD COLUMN IF NOT EXISTS is_deleted BOOLEAN NOT NULL DEFAULT FALSE'
                )
                cur.execute(
                    f'ALTER TABLE {self._table_name("documents")} ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT \'active\''
                )
                cur.execute(
                    f'ALTER TABLE {self._table_name("documents")} ADD COLUMN IF NOT EXISTS created_at TEXT NOT NULL DEFAULT \'\''
                )
                cur.execute(
                    f'ALTER TABLE {self._table_name("documents")} ADD COLUMN IF NOT EXISTS updated_at TEXT NOT NULL DEFAULT \'\''
                )
                cur.execute(
                    f"""
                    CREATE INDEX IF NOT EXISTS {self._index_name("documents_scope_idx")}
                    ON {self._table_name("documents")} (user_id, scope_id)
                    """
                )
                cur.execute(
                    f"""
                    CREATE UNIQUE INDEX IF NOT EXISTS {self._index_name("documents_scope_external_uniq")}
                    ON {self._table_name("documents")} (scope_id, external_id)
                    """
                )
                cur.execute(
                    f"""
                    CREATE INDEX IF NOT EXISTS {self._index_name("documents_scope_updated_idx")}
                    ON {self._table_name("documents")} (scope_id, source_updated_at DESC)
                    """
                )
                cur.execute(
                    f"""
                    CREATE INDEX IF NOT EXISTS {self._index_name("documents_scope_deleted_idx")}
                    ON {self._table_name("documents")} (scope_id, is_deleted, updated_at DESC)
                    """
                )
                cur.execute(
                    f"""
                    CREATE TABLE IF NOT EXISTS {self._table_name("document_versions")} (
                        id TEXT PRIMARY KEY,
                        document_id TEXT NOT NULL,
                        user_id TEXT NOT NULL,
                        scope_id TEXT NOT NULL,
                        content_hash TEXT NOT NULL,
                        raw_body TEXT NOT NULL,
                        normalized_body TEXT NOT NULL,
                        token_count INTEGER NOT NULL DEFAULT 0,
                        created_at TEXT NOT NULL DEFAULT '',
                        updated_at TEXT NOT NULL DEFAULT '',
                        FOREIGN KEY (document_id)
                            REFERENCES {self._table_name("documents")} (id)
                            ON DELETE CASCADE
                    )
                    """
                )
                cur.execute(
                    f'ALTER TABLE {self._table_name("document_versions")} ADD COLUMN IF NOT EXISTS version_no INTEGER NOT NULL DEFAULT 0'
                )
                cur.execute(
                    f'ALTER TABLE {self._table_name("document_versions")} ADD COLUMN IF NOT EXISTS raw_body_ref TEXT NOT NULL DEFAULT \'\''
                )
                cur.execute(
                    f'ALTER TABLE {self._table_name("document_versions")} ADD COLUMN IF NOT EXISTS outline_json JSONB NOT NULL DEFAULT \'{{}}\'::jsonb'
                )
                cur.execute(
                    f'ALTER TABLE {self._table_name("document_versions")} ADD COLUMN IF NOT EXISTS body_hash TEXT NOT NULL DEFAULT \'\''
                )
                cur.execute(
                    f"""
                    CREATE INDEX IF NOT EXISTS {self._index_name("document_versions_scope_idx")}
                    ON {self._table_name("document_versions")} (user_id, scope_id, document_id)
                    """
                )
                cur.execute(
                    f"""
                    CREATE UNIQUE INDEX IF NOT EXISTS {self._index_name("document_versions_doc_version_uniq")}
                    ON {self._table_name("document_versions")} (document_id, version_no)
                    """
                )
                cur.execute(
                    f"""
                    CREATE UNIQUE INDEX IF NOT EXISTS {self._index_name("document_versions_doc_hash_uniq")}
                    ON {self._table_name("document_versions")} (document_id, body_hash)
                    """
                )
                cur.execute(
                    f"""
                    CREATE INDEX IF NOT EXISTS {self._index_name("document_versions_doc_created_idx")}
                    ON {self._table_name("document_versions")} (document_id, created_at DESC)
                    """
                )
                cur.execute(self._chunks_ddl())
                cur.execute(
                    f'ALTER TABLE {self._table_name("chunks")} ADD COLUMN IF NOT EXISTS lexical_terms TEXT NOT NULL DEFAULT \'\''
                )
                cur.execute(
                    f'ALTER TABLE {self._table_name("chunks")} ADD COLUMN IF NOT EXISTS tsv tsvector NOT NULL DEFAULT \'\'::tsvector'
                )
                cur.execute(
                    f"""
                    CREATE INDEX IF NOT EXISTS {self._index_name("chunks_scope_idx")}
                    ON {self._table_name("chunks")} (user_id, scope_id)
                    """
                )
                cur.execute(
                    f"""
                    CREATE INDEX IF NOT EXISTS {self._index_name("chunks_tsv_idx")}
                    ON {self._table_name("chunks")} USING GIN (tsv)
                    """
                )
                cur.execute(
                    f"""
                    CREATE TABLE IF NOT EXISTS {self._table_name("index_jobs")} (
                        job_id TEXT PRIMARY KEY,
                        payload JSONB NOT NULL DEFAULT '{{}}'::jsonb,
                        updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
                    )
                    """
                )
                cur.execute(
                    f'ALTER TABLE {self._table_name("index_jobs")} ADD COLUMN IF NOT EXISTS user_id TEXT NOT NULL DEFAULT \'\''
                )
                cur.execute(
                    f'ALTER TABLE {self._table_name("index_jobs")} ADD COLUMN IF NOT EXISTS scope_id TEXT NOT NULL DEFAULT \'\''
                )
                cur.execute(
                    f'ALTER TABLE {self._table_name("index_jobs")} ADD COLUMN IF NOT EXISTS job_type TEXT NOT NULL DEFAULT \'\''
                )
                cur.execute(
                    f'ALTER TABLE {self._table_name("index_jobs")} ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT \'\''
                )
                cur.execute(
                    f'ALTER TABLE {self._table_name("index_jobs")} ADD COLUMN IF NOT EXISTS document_count INTEGER NOT NULL DEFAULT 0'
                )
                cur.execute(
                    f'ALTER TABLE {self._table_name("index_jobs")} ADD COLUMN IF NOT EXISTS chunk_count INTEGER NOT NULL DEFAULT 0'
                )
                cur.execute(
                    f'ALTER TABLE {self._table_name("index_jobs")} ADD COLUMN IF NOT EXISTS error_code TEXT NOT NULL DEFAULT \'\''
                )
                cur.execute(
                    f'ALTER TABLE {self._table_name("index_jobs")} ADD COLUMN IF NOT EXISTS error_message TEXT NOT NULL DEFAULT \'\''
                )
                cur.execute(
                    f'ALTER TABLE {self._table_name("index_jobs")} ADD COLUMN IF NOT EXISTS started_at TIMESTAMPTZ NULL'
                )
                cur.execute(
                    f'ALTER TABLE {self._table_name("index_jobs")} ADD COLUMN IF NOT EXISTS finished_at TIMESTAMPTZ NULL'
                )
                cur.execute(
                    f'ALTER TABLE {self._table_name("index_jobs")} ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()'
                )
                cur.execute(
                    f"""
                    CREATE INDEX IF NOT EXISTS {self._index_name("index_jobs_scope_idx")}
                    ON {self._table_name("index_jobs")} (scope_id, created_at DESC)
                    """
                )
                cur.execute(
                    f"""
                    CREATE INDEX IF NOT EXISTS {self._index_name("index_jobs_status_idx")}
                    ON {self._table_name("index_jobs")} (status, created_at DESC)
                    """
                )
                cur.execute(
                    f"""
                    CREATE TABLE IF NOT EXISTS {self._table_name("skill_mirrors")} (
                        skill_id TEXT NOT NULL,
                        user_id TEXT NOT NULL,
                        scope TEXT NOT NULL DEFAULT 'global',
                        definition_id TEXT NOT NULL DEFAULT '',
                        revision_id TEXT NOT NULL DEFAULT '',
                        kind TEXT NOT NULL DEFAULT 'chat_profile',
                        slug TEXT NOT NULL,
                        title TEXT NOT NULL,
                        description TEXT NOT NULL DEFAULT '',
                        prompt TEXT NOT NULL DEFAULT '',
                        mode TEXT NOT NULL DEFAULT 'answer',
                        source TEXT NOT NULL DEFAULT 'manual',
                        enabled BOOLEAN NOT NULL DEFAULT TRUE,
                        repo_url TEXT NOT NULL DEFAULT '',
                        runtime_spec_json JSONB NOT NULL DEFAULT '{{}}'::jsonb,
                        created_at TEXT NOT NULL DEFAULT '',
                        updated_at TEXT NOT NULL DEFAULT '',
                        PRIMARY KEY (skill_id, user_id)
                    )
                    """
                )
                cur.execute(
                    f"""
                    CREATE INDEX IF NOT EXISTS {self._index_name("skill_mirrors_user_updated_idx")}
                    ON {self._table_name("skill_mirrors")} (user_id, updated_at DESC)
                    """
                )
                cur.execute(
                    f"""
                    CREATE INDEX IF NOT EXISTS {self._index_name("skill_mirrors_user_enabled_mode_idx")}
                    ON {self._table_name("skill_mirrors")} (user_id, enabled, mode)
                    """
                )
                cur.execute(
                    f"""
                    CREATE TABLE IF NOT EXISTS {self._table_name("model_profiles")} (
                        id TEXT PRIMARY KEY,
                        user_id TEXT NOT NULL DEFAULT '',
                        purpose TEXT NOT NULL,
                        provider TEXT NOT NULL,
                        name TEXT NOT NULL,
                        base_url TEXT NOT NULL DEFAULT '',
                        api_key_ref TEXT NOT NULL DEFAULT '',
                        model_name TEXT NOT NULL,
                        temperature DOUBLE PRECISION NOT NULL DEFAULT 0.2,
                        max_tokens INTEGER NOT NULL DEFAULT 4096,
                        is_default BOOLEAN NOT NULL DEFAULT FALSE,
                        updated_at TEXT NOT NULL DEFAULT ''
                    )
                    """
                )
                cur.execute(
                    f"""
                    CREATE UNIQUE INDEX IF NOT EXISTS {self._index_name("model_profiles_name_uniq")}
                    ON {self._table_name("model_profiles")} (user_id, purpose, lower(name))
                    """
                )
                cur.execute(
                    f"""
                    CREATE UNIQUE INDEX IF NOT EXISTS {self._index_name("model_profiles_default_uniq")}
                    ON {self._table_name("model_profiles")} (user_id, purpose)
                    WHERE is_default = TRUE
                    """
                )
                cur.execute(
                    f"""
                    CREATE TABLE IF NOT EXISTS {self._table_name("traces")} (
                        id BIGSERIAL PRIMARY KEY,
                        payload JSONB NOT NULL DEFAULT '{{}}'::jsonb,
                        created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
                    )
                    """
                )
                cur.execute(
                    f"""
                    CREATE TABLE IF NOT EXISTS {self._table_name("request_traces")} (
                        id TEXT PRIMARY KEY,
                        request_type TEXT NOT NULL,
                        user_id TEXT NOT NULL DEFAULT '',
                        run_id TEXT NOT NULL DEFAULT '',
                        message_id TEXT NOT NULL DEFAULT '',
                        skill_id TEXT NOT NULL DEFAULT '',
                        model_profile_id TEXT NOT NULL DEFAULT '',
                        scope_ids TEXT[] NOT NULL DEFAULT '{{}}',
                        query_text TEXT NOT NULL DEFAULT '',
                        mode TEXT NOT NULL DEFAULT '',
                        retrieved_chunk_ids TEXT[] NOT NULL DEFAULT '{{}}',
                        reranked_chunk_ids TEXT[] NOT NULL DEFAULT '{{}}',
                        used_chunk_ids TEXT[] NOT NULL DEFAULT '{{}}',
                        latency_retrieve_ms INTEGER NOT NULL DEFAULT 0,
                        latency_generate_ms INTEGER NOT NULL DEFAULT 0,
                        latency_total_ms INTEGER NOT NULL DEFAULT 0,
                        retrieval_backend TEXT NOT NULL DEFAULT '',
                        candidate_mode TEXT NOT NULL DEFAULT '',
                        output_preview TEXT NOT NULL DEFAULT '',
                        error_code TEXT NOT NULL DEFAULT '',
                        error_message TEXT NOT NULL DEFAULT '',
                        payload JSONB NOT NULL DEFAULT '{{}}'::jsonb,
                        created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
                    )
                    """
                )
                cur.execute(
                    f"""
                    CREATE INDEX IF NOT EXISTS {self._index_name("request_traces_user_created_idx")}
                    ON {self._table_name("request_traces")} (user_id, created_at DESC)
                    """
                )
                cur.execute(
                    f"""
                    CREATE INDEX IF NOT EXISTS {self._index_name("request_traces_type_created_idx")}
                    ON {self._table_name("request_traces")} (request_type, created_at DESC)
                    """
                )
                cur.execute(
                    f"""
                    CREATE INDEX IF NOT EXISTS {self._index_name("request_traces_run_created_idx")}
                    ON {self._table_name("request_traces")} (run_id, created_at DESC)
                    WHERE run_id <> ''
                    """
                )
            conn.commit()
        self._schema_ready = True

    def save_scope_bundle(
        self,
        scope: ScopeRef,
        documents: List[KnowledgeDocument],
        chunks: List[KnowledgeChunk],
        embeddings: np.ndarray,
        compat_meta: Dict[str, object],
    ) -> None:
        self.ensure_schema()
        if embeddings.shape[0] != len(chunks):
            raise IndexingError("embedding row count does not match chunk count")
        doc_token_counts: Dict[str, int] = {}
        for chunk in chunks:
            doc_token_counts[chunk.document_id] = doc_token_counts.get(chunk.document_id, 0) + int(chunk.token_count or 0)
        incoming_ids = [doc.id for doc in documents]
        chunk_indexes_by_doc: Dict[str, List[int]] = {}
        for index, chunk in enumerate(chunks):
            chunk_indexes_by_doc.setdefault(chunk.document_id, []).append(index)
        with self._connect() as conn:
            with conn.cursor() as cur:
                cur.execute(
                    f"""
                    SELECT id, content_hash, COALESCE(NULLIF(latest_version_id, ''), version_id) AS version_id, is_deleted
                    FROM {self._table_name("documents")}
                    WHERE user_id = %s AND scope_id = %s
                    """,
                    (scope.user_id, scope.scope_id),
                )
                existing_documents = {str(row["id"]): dict(row) for row in cur.fetchall()}
                changed_doc_ids = {
                    doc.id
                    for doc in documents
                    if doc.id not in existing_documents
                    or str(existing_documents[doc.id].get("content_hash") or "") != doc.content_hash
                    or str(existing_documents[doc.id].get("version_id") or "") != doc.version_id
                    or bool(existing_documents[doc.id].get("is_deleted"))
                }
                cur.execute(
                    f"""
                    INSERT INTO {self._table_name("scopes")} (
                        user_id, scope_id, scope_type, scope_external_id, provider, name, status,
                        latest_index_version, document_count, chunk_count, last_indexed_at,
                        created_at, updated_at, compat_meta
                    ) VALUES (
                        %s, %s, %s, %s, %s, %s, %s,
                        %s, %s, %s, %s,
                        %s, %s, %s::jsonb
                    )
                    ON CONFLICT (user_id, scope_id) DO UPDATE SET
                        scope_type = EXCLUDED.scope_type,
                        scope_external_id = EXCLUDED.scope_external_id,
                        provider = EXCLUDED.provider,
                        name = EXCLUDED.name,
                        status = EXCLUDED.status,
                        latest_index_version = EXCLUDED.latest_index_version,
                        document_count = EXCLUDED.document_count,
                        chunk_count = EXCLUDED.chunk_count,
                        last_indexed_at = EXCLUDED.last_indexed_at,
                        created_at = CASE
                            WHEN {self._table_name("scopes")}.created_at = '' THEN EXCLUDED.created_at
                            ELSE {self._table_name("scopes")}.created_at
                        END,
                        updated_at = EXCLUDED.updated_at,
                        compat_meta = EXCLUDED.compat_meta
                    """,
                    (
                        scope.user_id,
                        scope.scope_id,
                        scope.scope_type,
                        scope.scope_external_id,
                        scope.provider,
                        scope.name,
                        scope.status,
                        scope.latest_index_version,
                        scope.document_count,
                        scope.chunk_count,
                        scope.last_indexed_at,
                        scope.created_at,
                        scope.updated_at,
                        json.dumps(compat_meta, ensure_ascii=False),
                    ),
                )
                if incoming_ids:
                    cur.execute(
                        f"""
                        UPDATE {self._table_name("documents")}
                        SET is_deleted = TRUE,
                            status = 'deleted',
                            updated_at = %s
                        WHERE user_id = %s AND scope_id = %s AND NOT (id = ANY(%s))
                        """,
                        (scope.updated_at, scope.user_id, scope.scope_id, incoming_ids),
                    )
                else:
                    cur.execute(
                        f"""
                        UPDATE {self._table_name("documents")}
                        SET is_deleted = TRUE,
                            status = 'deleted',
                            updated_at = %s
                        WHERE user_id = %s AND scope_id = %s
                        """,
                        (scope.updated_at, scope.user_id, scope.scope_id),
                    )
                if incoming_ids:
                    cur.execute(
                        f"""
                        UPDATE {self._table_name("documents")}
                        SET is_deleted = FALSE
                        WHERE user_id = %s AND scope_id = %s AND id = ANY(%s)
                        """,
                        (scope.user_id, scope.scope_id, incoming_ids),
                    )
                if changed_doc_ids:
                    cur.execute(
                        f"""
                        DELETE FROM {self._table_name("chunks")}
                        WHERE user_id = %s AND scope_id = %s AND document_id = ANY(%s)
                        """,
                        (scope.user_id, scope.scope_id, list(changed_doc_ids)),
                    )
                cur.executemany(
                    f"""
                    INSERT INTO {self._table_name("documents")} (
                        id, user_id, scope_id, provider, external_id, repo, title, doc_ref,
                        source_url, source_updated_at, content_hash, version_id, latest_version_id,
                        is_deleted, status, created_at, updated_at, raw_body, normalized_body
                    ) VALUES (
                        %s, %s, %s, %s, %s, %s, %s, %s,
                        %s, %s, %s, %s, %s,
                        %s, %s, %s, %s, %s, %s
                    )
                    ON CONFLICT (id) DO UPDATE SET
                        user_id = EXCLUDED.user_id,
                        scope_id = EXCLUDED.scope_id,
                        provider = EXCLUDED.provider,
                        external_id = EXCLUDED.external_id,
                        repo = EXCLUDED.repo,
                        title = EXCLUDED.title,
                        doc_ref = EXCLUDED.doc_ref,
                        source_url = EXCLUDED.source_url,
                        source_updated_at = EXCLUDED.source_updated_at,
                        content_hash = EXCLUDED.content_hash,
                        version_id = EXCLUDED.version_id,
                        latest_version_id = EXCLUDED.latest_version_id,
                        is_deleted = EXCLUDED.is_deleted,
                        status = EXCLUDED.status,
                        created_at = CASE
                            WHEN {self._table_name("documents")}.created_at = '' THEN EXCLUDED.created_at
                            ELSE {self._table_name("documents")}.created_at
                        END,
                        updated_at = EXCLUDED.updated_at,
                        raw_body = EXCLUDED.raw_body,
                        normalized_body = EXCLUDED.normalized_body
                    """,
                    [
                        (
                            doc.id,
                            doc.user_id,
                            doc.scope_id,
                            doc.provider,
                            doc.external_id,
                            doc.repo,
                            doc.title,
                            doc.doc_ref,
                            doc.source_url,
                            doc.source_updated_at,
                            doc.content_hash,
                            doc.version_id,
                            doc.version_id,
                            False,
                            "active",
                            scope.created_at,
                            scope.updated_at,
                            doc.raw_body,
                            doc.normalized_body,
                        )
                        for doc in documents
                    ],
                )
                changed_documents = [doc for doc in documents if doc.id in changed_doc_ids]
                if changed_documents:
                    cur.execute(
                        f"""
                        SELECT id, document_id, version_no
                        FROM {self._table_name("document_versions")}
                        WHERE id = ANY(%s)
                        """,
                        ([doc.version_id for doc in changed_documents],),
                    )
                    existing_versions = {str(row["id"]): dict(row) for row in cur.fetchall()}
                    cur.execute(
                        f"""
                        SELECT document_id, COALESCE(MAX(version_no), 0) AS max_version_no
                        FROM {self._table_name("document_versions")}
                        WHERE document_id = ANY(%s)
                        GROUP BY document_id
                        """,
                        ([doc.id for doc in changed_documents],),
                    )
                    max_version_numbers = {
                        str(row["document_id"]): int(row.get("max_version_no") or 0)
                        for row in cur.fetchall()
                    }
                    cur.executemany(
                        f"""
                        INSERT INTO {self._table_name("document_versions")} (
                            id, document_id, user_id, scope_id, content_hash, raw_body,
                            normalized_body, token_count, version_no, raw_body_ref,
                            outline_json, body_hash, created_at, updated_at
                        ) VALUES (
                            %s, %s, %s, %s, %s, %s,
                            %s, %s, %s, %s,
                            %s::jsonb, %s, %s, %s
                        )
                        ON CONFLICT (id) DO UPDATE SET
                            content_hash = EXCLUDED.content_hash,
                            raw_body = EXCLUDED.raw_body,
                            normalized_body = EXCLUDED.normalized_body,
                            token_count = EXCLUDED.token_count,
                            version_no = EXCLUDED.version_no,
                            raw_body_ref = EXCLUDED.raw_body_ref,
                            outline_json = EXCLUDED.outline_json,
                            body_hash = EXCLUDED.body_hash,
                            updated_at = EXCLUDED.updated_at
                        """,
                        [
                            (
                                doc.version_id,
                                doc.id,
                                doc.user_id,
                                doc.scope_id,
                                doc.content_hash,
                                doc.raw_body,
                                doc.normalized_body,
                                doc_token_counts.get(doc.id, 0),
                                int(existing_versions.get(doc.version_id, {}).get("version_no") or (max_version_numbers.get(doc.id, 0) + 1)),
                                "",
                                json.dumps({}, ensure_ascii=False),
                                doc.content_hash,
                                scope.created_at,
                                scope.updated_at,
                            )
                            for doc in changed_documents
                        ],
                    )
                changed_chunk_indexes = [
                    index
                    for document_id in changed_doc_ids
                    for index in chunk_indexes_by_doc.get(document_id, [])
                ]
                if changed_chunk_indexes and self._vector_mode == "native":
                    cur.executemany(
                        f"""
                        INSERT INTO {self._table_name("chunks")} (
                            id, user_id, scope_id, document_id, version_id, provider, repo, title,
                            heading_path, source_url, updated_at, chunk_index, content, lexical_text,
                            lexical_terms, tsv, token_count, embedding
                        ) VALUES (
                            %s, %s, %s, %s, %s, %s, %s, %s,
                            %s, %s, %s, %s, %s, %s,
                            %s, to_tsvector('simple', %s), %s, %s::vector
                        )
                        """,
                        [
                            (
                                chunks[index].id,
                                chunks[index].user_id,
                                chunks[index].scope_id,
                                chunks[index].document_id,
                                chunks[index].version_id,
                                chunks[index].provider,
                                chunks[index].repo,
                                chunks[index].title,
                                chunks[index].heading_path,
                                chunks[index].source_url,
                                chunks[index].updated_at,
                                chunks[index].chunk_index,
                                chunks[index].content,
                                chunks[index].lexical_text,
                                self._lexical_terms(chunks[index].lexical_text),
                                self._lexical_terms(chunks[index].lexical_text),
                                chunks[index].token_count,
                                self._vector_literal(embeddings[index]),
                            )
                            for index in changed_chunk_indexes
                        ],
                    )
                elif changed_chunk_indexes:
                    cur.executemany(
                        f"""
                        INSERT INTO {self._table_name("chunks")} (
                            id, user_id, scope_id, document_id, version_id, provider, repo, title,
                            heading_path, source_url, updated_at, chunk_index, content, lexical_text,
                            lexical_terms, tsv, token_count, embedding_json
                        ) VALUES (
                            %s, %s, %s, %s, %s, %s, %s, %s,
                            %s, %s, %s, %s, %s, %s,
                            %s, to_tsvector('simple', %s), %s, %s::jsonb
                        )
                        """,
                        [
                            (
                                chunks[index].id,
                                chunks[index].user_id,
                                chunks[index].scope_id,
                                chunks[index].document_id,
                                chunks[index].version_id,
                                chunks[index].provider,
                                chunks[index].repo,
                                chunks[index].title,
                                chunks[index].heading_path,
                                chunks[index].source_url,
                                chunks[index].updated_at,
                                chunks[index].chunk_index,
                                chunks[index].content,
                                chunks[index].lexical_text,
                                self._lexical_terms(chunks[index].lexical_text),
                                self._lexical_terms(chunks[index].lexical_text),
                                chunks[index].token_count,
                                json.dumps([float(value) for value in embeddings[index]], ensure_ascii=False),
                            )
                            for index in changed_chunk_indexes
                        ],
                    )
            conn.commit()

    def load_scope_bundle(self, user_id: str, scope_id: str) -> Optional[Dict[str, object]]:
        self.ensure_schema()
        with self._connect() as conn:
            with conn.cursor() as cur:
                cur.execute(
                    f"""
                    SELECT user_id, scope_id, scope_type, scope_external_id, provider, name, status,
                           latest_index_version, document_count, chunk_count, last_indexed_at,
                           created_at, updated_at, compat_meta
                    FROM {self._table_name("scopes")}
                    WHERE user_id = %s AND scope_id = %s
                    """,
                    (user_id, scope_id),
                )
                scope_row = cur.fetchone()
                if not scope_row:
                    return None
                compat_meta = scope_row.pop("compat_meta", {}) or {}
                cur.execute(
                    f"""
                    SELECT d.id, d.scope_id, d.user_id, d.provider, d.external_id, d.repo, d.title, d.doc_ref,
                           d.source_url, d.source_updated_at, d.content_hash,
                           COALESCE(NULLIF(d.latest_version_id, ''), d.version_id) AS version_id,
                           COALESCE(v.raw_body, d.raw_body) AS raw_body,
                           COALESCE(v.normalized_body, d.normalized_body) AS normalized_body
                    FROM {self._table_name("documents")} d
                    LEFT JOIN {self._table_name("document_versions")} v
                      ON v.id = COALESCE(NULLIF(d.latest_version_id, ''), d.version_id)
                    WHERE d.user_id = %s AND d.scope_id = %s AND d.is_deleted = FALSE
                    ORDER BY d.title ASC, d.id ASC
                    """,
                    (user_id, scope_id),
                )
                document_rows = [dict(row) for row in cur.fetchall()]
                cur.execute(self._chunk_select_sql(), (user_id, scope_id))
                chunk_rows = []
                embedding_rows = []
                for row in cur.fetchall():
                    chunk_payload = dict(row)
                    if self._vector_mode == "native":
                        embedding_rows.append(self._parse_vector_text(str(chunk_payload.pop("embedding_text", ""))))
                    else:
                        embedding_rows.append(self._parse_embedding_json(chunk_payload.pop("embedding_json", [])))
                    chunk_rows.append(chunk_payload)
        embeddings = (
            np.asarray(embedding_rows, dtype=np.float32)
            if embedding_rows
            else np.zeros((0, self.embedding_dim), dtype=np.float32)
        )
        return {
            "scope": dict(scope_row),
            "documents": document_rows,
            "chunks": chunk_rows,
            "embeddings": embeddings,
            "meta": compat_meta,
        }

    def search_candidates(
        self,
        *,
        user_id: str,
        scope_ids: List[str],
        query: str,
        query_embedding: np.ndarray,
        lexical_top_k: int,
        vector_top_k: int,
    ) -> Optional[Dict[str, object]]:
        """Run coarse retrieval in Postgres before Python fusion/rerank.

        This is the main step that moves the postgres backend closer to the
        target architecture: we stop loading every chunk for every scope and
        instead let the database pre-select lexical and vector candidates.
        """

        self.ensure_schema()
        if not scope_ids:
            return None
        lexical_rows = self._search_lexical_candidates(
            user_id=user_id,
            scope_ids=scope_ids,
            query=query,
            limit=max(lexical_top_k, 1),
        )
        vector_rows = self._search_vector_candidates(
            user_id=user_id,
            scope_ids=scope_ids,
            query_embedding=query_embedding,
            limit=max(vector_top_k, 1),
        )
        chunk_map: Dict[str, Dict[str, object]] = {}
        lexical_ranking: List[Dict[str, object]] = []
        vector_ranking: List[Dict[str, object]] = []

        for row in lexical_rows:
            chunk_id = str(row.get("id") or "")
            if not chunk_id:
                continue
            chunk_map[chunk_id] = self._chunk_payload(row)
            lexical_ranking.append({"chunk_id": chunk_id, "score": float(row.get("score") or 0.0)})

        for row in vector_rows:
            chunk_id = str(row.get("id") or "")
            if not chunk_id:
                continue
            chunk_map.setdefault(chunk_id, self._chunk_payload(row))
            vector_ranking.append({"chunk_id": chunk_id, "score": float(row.get("score") or 0.0)})

        return {
            "chunks": list(chunk_map.values()),
            "lexical_ranking": lexical_ranking,
            "vector_ranking": vector_ranking,
            "meta": {"candidate_mode": "postgres_assisted", "vector_mode": self._vector_mode},
        }

    def delete_scope_bundle(self, user_id: str, scope_id: str) -> None:
        self.ensure_schema()
        with self._connect() as conn:
            with conn.cursor() as cur:
                cur.execute(
                    f"DELETE FROM {self._table_name('scopes')} WHERE user_id = %s AND scope_id = %s",
                    (user_id, scope_id),
                )
            conn.commit()

    def list_scope_ids(self, user_id: str) -> List[str]:
        self.ensure_schema()
        with self._connect() as conn:
            with conn.cursor() as cur:
                cur.execute(
                    f"SELECT scope_id FROM {self._table_name('scopes')} WHERE user_id = %s ORDER BY scope_id ASC",
                    (user_id,),
                )
                return [str(row["scope_id"]) for row in cur.fetchall()]

    def list_user_ids(self) -> List[str]:
        self.ensure_schema()
        with self._connect() as conn:
            with conn.cursor() as cur:
                cur.execute(f"SELECT DISTINCT user_id FROM {self._table_name('scopes')} ORDER BY user_id ASC")
                return [str(row["user_id"]) for row in cur.fetchall()]

    def upsert_index_job(self, job_id: str, payload: Dict[str, Any]) -> None:
        self.ensure_schema()
        job_type = str(payload.get("job_type") or payload.get("operation") or "").strip()
        status = str(payload.get("status") or "").strip()
        error_message = str(payload.get("error_message") or payload.get("error") or "").strip()
        with self._connect() as conn:
            with conn.cursor() as cur:
                cur.execute(
                    f"""
                    INSERT INTO {self._table_name("index_jobs")} (
                        job_id, payload, updated_at, user_id, scope_id, job_type, status,
                        document_count, chunk_count, error_code, error_message,
                        started_at, finished_at, created_at
                    ) VALUES (
                        %s, %s::jsonb, NOW(), %s, %s, %s, %s,
                        %s, %s, %s, %s,
                        %s, %s, COALESCE(%s, NOW())
                    )
                    ON CONFLICT (job_id) DO UPDATE SET
                        payload = EXCLUDED.payload,
                        updated_at = NOW(),
                        user_id = EXCLUDED.user_id,
                        scope_id = EXCLUDED.scope_id,
                        job_type = EXCLUDED.job_type,
                        status = EXCLUDED.status,
                        document_count = EXCLUDED.document_count,
                        chunk_count = EXCLUDED.chunk_count,
                        error_code = EXCLUDED.error_code,
                        error_message = EXCLUDED.error_message,
                        started_at = COALESCE(EXCLUDED.started_at, {self._table_name("index_jobs")}.started_at),
                        finished_at = EXCLUDED.finished_at,
                        created_at = {self._table_name("index_jobs")}.created_at
                    """,
                    (
                        job_id,
                        json.dumps(payload, ensure_ascii=False),
                        str(payload.get("user_id") or ""),
                        str(payload.get("scope_id") or ""),
                        job_type,
                        status,
                        int(payload.get("document_count") or 0),
                        int(payload.get("chunk_count") or 0),
                        str(payload.get("error_code") or ""),
                        error_message,
                        self._parse_timestamp(payload.get("started_at") or payload.get("created_at")),
                        self._parse_timestamp(payload.get("finished_at") or payload.get("updated_at")),
                        self._parse_timestamp(payload.get("created_at")),
                    ),
                )
            conn.commit()

    def get_index_job(self, job_id: str) -> Optional[Dict[str, Any]]:
        self.ensure_schema()
        with self._connect() as conn:
            with conn.cursor() as cur:
                cur.execute(
                    f"""
                    SELECT job_id, user_id, scope_id, job_type, status, document_count, chunk_count,
                           error_code, error_message, started_at, finished_at, created_at, updated_at, payload
                    FROM {self._table_name('index_jobs')}
                    WHERE job_id = %s
                    """,
                    (job_id,),
                )
                row = cur.fetchone()
                if not row:
                    return None
                payload = dict(row.get("payload") or {})
                payload.setdefault("job_id", str(row.get("job_id") or job_id))
                payload.setdefault("user_id", str(row.get("user_id") or ""))
                payload.setdefault("scope_id", str(row.get("scope_id") or ""))
                payload.setdefault("operation", str(row.get("job_type") or ""))
                payload.setdefault("status", str(row.get("status") or ""))
                payload.setdefault("document_count", int(row.get("document_count") or 0))
                payload.setdefault("chunk_count", int(row.get("chunk_count") or 0))
                payload.setdefault("error_code", str(row.get("error_code") or ""))
                payload.setdefault("error", str(row.get("error_message") or ""))
                payload.setdefault("started_at", self._iso_timestamp(row.get("started_at")))
                payload.setdefault("finished_at", self._iso_timestamp(row.get("finished_at")))
                payload.setdefault("created_at", self._iso_timestamp(row.get("created_at")))
                payload.setdefault("updated_at", self._iso_timestamp(row.get("updated_at")))
                return payload

    def list_index_jobs(
        self,
        *,
        statuses: List[str],
        user_id: str = "",
        scope_id: str = "",
        limit: int = 100,
    ) -> List[Dict[str, Any]]:
        self.ensure_schema()
        predicates = ["1=1"]
        params: List[Any] = []
        if statuses:
            predicates.append("status = ANY(%s)")
            params.append(list(statuses))
        if user_id:
            predicates.append("user_id = %s")
            params.append(user_id)
        if scope_id:
            predicates.append("scope_id = %s")
            params.append(scope_id)
        params.append(max(limit, 0))
        with self._connect() as conn:
            with conn.cursor() as cur:
                cur.execute(
                    f"""
                    SELECT job_id, user_id, scope_id, job_type, status, document_count, chunk_count,
                           error_code, error_message, started_at, finished_at, created_at, updated_at, payload
                    FROM {self._table_name('index_jobs')}
                    WHERE {' AND '.join(predicates)}
                    ORDER BY created_at ASC
                    LIMIT %s
                    """,
                    tuple(params),
                )
                rows = cur.fetchall()
        items: List[Dict[str, Any]] = []
        for row in rows:
            payload = dict(row.get("payload") or {})
            payload.setdefault("job_id", str(row.get("job_id") or ""))
            payload.setdefault("user_id", str(row.get("user_id") or ""))
            payload.setdefault("scope_id", str(row.get("scope_id") or ""))
            payload.setdefault("operation", str(row.get("job_type") or ""))
            payload.setdefault("status", str(row.get("status") or ""))
            payload.setdefault("document_count", int(row.get("document_count") or 0))
            payload.setdefault("chunk_count", int(row.get("chunk_count") or 0))
            payload.setdefault("error_code", str(row.get("error_code") or ""))
            payload.setdefault("error", str(row.get("error_message") or ""))
            payload.setdefault("started_at", self._iso_timestamp(row.get("started_at")))
            payload.setdefault("finished_at", self._iso_timestamp(row.get("finished_at")))
            payload.setdefault("created_at", self._iso_timestamp(row.get("created_at")))
            payload.setdefault("updated_at", self._iso_timestamp(row.get("updated_at")))
            items.append(payload)
        return items

    def append_trace(self, payload: Dict[str, Any]) -> None:
        self.ensure_schema()
        trace_id = str(payload.get("trace_id") or payload.get("id") or "")
        with self._connect() as conn:
            with conn.cursor() as cur:
                cur.execute(
                    f"INSERT INTO {self._table_name('traces')} (payload) VALUES (%s::jsonb)",
                    (json.dumps(payload, ensure_ascii=False),),
                )
                if trace_id:
                    cur.execute(
                        f"""
                        INSERT INTO {self._table_name("request_traces")} (
                            id, request_type, user_id, run_id, message_id, skill_id, model_profile_id,
                            scope_ids, query_text, mode, retrieved_chunk_ids, reranked_chunk_ids, used_chunk_ids,
                            latency_retrieve_ms, latency_generate_ms, latency_total_ms, retrieval_backend,
                            candidate_mode, output_preview, error_code, error_message, payload, created_at
                        ) VALUES (
                            %s, %s, %s, %s, %s, %s, %s,
                            %s, %s, %s, %s, %s, %s,
                            %s, %s, %s, %s,
                            %s, %s, %s, %s, %s::jsonb, NOW()
                        )
                        ON CONFLICT (id) DO UPDATE SET
                            request_type = EXCLUDED.request_type,
                            user_id = EXCLUDED.user_id,
                            run_id = EXCLUDED.run_id,
                            message_id = EXCLUDED.message_id,
                            skill_id = EXCLUDED.skill_id,
                            model_profile_id = EXCLUDED.model_profile_id,
                            scope_ids = EXCLUDED.scope_ids,
                            query_text = EXCLUDED.query_text,
                            mode = EXCLUDED.mode,
                            retrieved_chunk_ids = EXCLUDED.retrieved_chunk_ids,
                            reranked_chunk_ids = EXCLUDED.reranked_chunk_ids,
                            used_chunk_ids = EXCLUDED.used_chunk_ids,
                            latency_retrieve_ms = EXCLUDED.latency_retrieve_ms,
                            latency_generate_ms = EXCLUDED.latency_generate_ms,
                            latency_total_ms = EXCLUDED.latency_total_ms,
                            retrieval_backend = EXCLUDED.retrieval_backend,
                            candidate_mode = EXCLUDED.candidate_mode,
                            output_preview = EXCLUDED.output_preview,
                            error_code = EXCLUDED.error_code,
                            error_message = EXCLUDED.error_message,
                            payload = EXCLUDED.payload
                        """,
                        (
                            trace_id,
                            str(payload.get("request_type") or ""),
                            str(payload.get("user_id") or ""),
                            str(payload.get("run_id") or ""),
                            str(payload.get("message_id") or ""),
                            str(payload.get("skill_id") or ""),
                            str(payload.get("model_profile_id") or ""),
                            list(payload.get("scope_ids") or []),
                            str(payload.get("query_text") or ""),
                            str(payload.get("mode") or ""),
                            list(payload.get("retrieved_chunk_ids") or []),
                            list(payload.get("reranked_chunk_ids") or []),
                            list(payload.get("used_chunk_ids") or []),
                            int(payload.get("latency_retrieve_ms") or 0),
                            int(payload.get("latency_generate_ms") or 0),
                            int(payload.get("latency_total_ms") or 0),
                            str(payload.get("retrieval_backend") or ""),
                            str(payload.get("candidate_mode") or ""),
                            str(payload.get("answer_preview") or payload.get("summary") or payload.get("output_preview") or ""),
                            str(payload.get("error_code") or ""),
                            str(payload.get("error_message") or ""),
                            json.dumps(payload, ensure_ascii=False),
                        ),
                    )
            conn.commit()

    def get_request_trace(self, trace_id: str) -> Optional[Dict[str, Any]]:
        self.ensure_schema()
        with self._connect() as conn:
            with conn.cursor() as cur:
                cur.execute(
                    f"""
                    SELECT id, request_type, user_id, run_id, message_id, skill_id, model_profile_id,
                           scope_ids, query_text, mode, retrieved_chunk_ids, reranked_chunk_ids,
                           used_chunk_ids, latency_retrieve_ms, latency_generate_ms, latency_total_ms,
                           retrieval_backend, candidate_mode, output_preview, error_code, error_message,
                           created_at, payload
                    FROM {self._table_name("request_traces")}
                    WHERE id = %s
                    """,
                    (trace_id,),
                )
                row = cur.fetchone()
                return self._request_trace_payload(row) if row else None

    def list_request_traces(
        self,
        *,
        user_id: str,
        limit: int = 20,
        request_type: str = "",
        run_id: str = "",
        message_id: str = "",
        skill_id: str = "",
        model_profile_id: str = "",
    ) -> List[Dict[str, Any]]:
        self.ensure_schema()
        limit = max(1, min(limit, 200))
        with self._connect() as conn:
            with conn.cursor() as cur:
                conditions = ["user_id = %s"]
                params: List[Any] = [user_id]
                if request_type:
                    conditions.append("request_type = %s")
                    params.append(request_type)
                if run_id:
                    conditions.append("run_id = %s")
                    params.append(run_id)
                if message_id:
                    conditions.append("message_id = %s")
                    params.append(message_id)
                if skill_id:
                    conditions.append("skill_id = %s")
                    params.append(skill_id)
                if model_profile_id:
                    conditions.append("model_profile_id = %s")
                    params.append(model_profile_id)
                params.append(limit)
                cur.execute(
                    f"""
                    SELECT id, request_type, user_id, run_id, message_id, skill_id, model_profile_id,
                           scope_ids, query_text, mode, retrieved_chunk_ids, reranked_chunk_ids,
                           used_chunk_ids, latency_retrieve_ms, latency_generate_ms, latency_total_ms,
                           retrieval_backend, candidate_mode, output_preview, error_code, error_message,
                           created_at, payload
                    FROM {self._table_name("request_traces")}
                    WHERE {' AND '.join(conditions)}
                    ORDER BY created_at DESC
                    LIMIT %s
                    """,
                    tuple(params),
                )
                return [self._request_trace_payload(row) for row in cur.fetchall()]

    def cleanup_request_traces(
        self,
        *,
        trace_retention_days: int,
        report_trace_retention_days: int,
    ) -> Dict[str, int]:
        self.ensure_schema()
        now = datetime.now(timezone.utc)
        default_cutoff = now - timedelta(days=max(trace_retention_days, 0))
        report_cutoff = now - timedelta(days=max(report_trace_retention_days, 0))
        with self._connect() as conn:
            with conn.cursor() as cur:
                cur.execute(
                    f"""
                    DELETE FROM {self._table_name("request_traces")}
                    WHERE (
                        request_type IN ('report', 'quality_benchmark')
                        AND created_at < %s
                    ) OR (
                        request_type NOT IN ('report', 'quality_benchmark')
                        AND created_at < %s
                    )
                    """,
                    (report_cutoff, default_cutoff),
                )
                request_removed = cur.rowcount or 0
                cur.execute(
                    f"""
                    DELETE FROM {self._table_name("traces")}
                    WHERE (
                        COALESCE(payload->>'request_type', '') IN ('report', 'quality_benchmark')
                        AND created_at < %s
                    ) OR (
                        COALESCE(payload->>'request_type', '') NOT IN ('report', 'quality_benchmark')
                        AND created_at < %s
                    )
                    """,
                    (report_cutoff, default_cutoff),
                )
                raw_removed = cur.rowcount or 0
            conn.commit()
        return {"removed": int(request_removed + raw_removed), "kept": 0}

    def upsert_skill_mirror(self, payload: Dict[str, Any]) -> None:
        self.ensure_schema()
        with self._connect() as conn:
            with conn.cursor() as cur:
                cur.execute(
                    f"""
                    INSERT INTO {self._table_name("skill_mirrors")} (
                        skill_id, user_id, scope, definition_id, revision_id, kind, slug, title,
                        description, prompt, mode, source, enabled, repo_url, runtime_spec_json,
                        created_at, updated_at
                    ) VALUES (
                        %s, %s, %s, %s, %s, %s, %s, %s,
                        %s, %s, %s, %s, %s, %s, %s::jsonb,
                        %s, %s
                    )
                    ON CONFLICT (skill_id, user_id) DO UPDATE SET
                        scope = EXCLUDED.scope,
                        definition_id = EXCLUDED.definition_id,
                        revision_id = EXCLUDED.revision_id,
                        kind = EXCLUDED.kind,
                        slug = EXCLUDED.slug,
                        title = EXCLUDED.title,
                        description = EXCLUDED.description,
                        prompt = EXCLUDED.prompt,
                        mode = EXCLUDED.mode,
                        source = EXCLUDED.source,
                        enabled = EXCLUDED.enabled,
                        repo_url = EXCLUDED.repo_url,
                        runtime_spec_json = EXCLUDED.runtime_spec_json,
                        created_at = EXCLUDED.created_at,
                        updated_at = EXCLUDED.updated_at
                    """,
                    (
                        str(payload.get("id") or ""),
                        str(payload.get("user_id") or ""),
                        str(payload.get("scope") or "global"),
                        str(payload.get("definition_id") or ""),
                        str(payload.get("revision_id") or ""),
                        str(payload.get("kind") or "chat_profile"),
                        str(payload.get("slug") or ""),
                        str(payload.get("title") or ""),
                        str(payload.get("description") or ""),
                        str(payload.get("prompt") or ""),
                        str(payload.get("mode") or "answer"),
                        str(payload.get("source") or "manual"),
                        bool(payload.get("enabled", True)),
                        str(payload.get("repo_url") or ""),
                        json.dumps(payload.get("runtime_spec") or {}, ensure_ascii=False),
                        str(payload.get("created_at") or ""),
                        str(payload.get("updated_at") or ""),
                    ),
                )
            conn.commit()

    def delete_skill_mirror(self, user_id: str, skill_id: str) -> None:
        self.ensure_schema()
        with self._connect() as conn:
            with conn.cursor() as cur:
                cur.execute(
                    f"DELETE FROM {self._table_name('skill_mirrors')} WHERE user_id = %s AND skill_id = %s",
                    (user_id, skill_id),
                )
            conn.commit()

    def get_skill_mirror(self, user_id: str, skill_id: str) -> Optional[Dict[str, Any]]:
        self.ensure_schema()
        with self._connect() as conn:
            with conn.cursor() as cur:
                cur.execute(
                    f"""
                    SELECT skill_id, user_id, scope, definition_id, revision_id, kind, slug, title,
                           description, prompt, mode, source, enabled, repo_url, runtime_spec_json,
                           created_at, updated_at
                    FROM {self._table_name("skill_mirrors")}
                    WHERE user_id = %s AND skill_id = %s
                    """,
                    (user_id, skill_id),
                )
                row = cur.fetchone()
                if not row:
                    return None
                payload = dict(row)
                payload["id"] = payload.pop("skill_id")
                payload["runtime_spec"] = payload.pop("runtime_spec_json", {}) or {}
                return payload

    def upsert_model_profile(self, payload: Dict[str, Any]) -> None:
        self.ensure_schema()
        with self._connect() as conn:
            with conn.cursor() as cur:
                if bool(payload.get("is_default", False)):
                    cur.execute(
                        f"""
                        UPDATE {self._table_name("model_profiles")}
                        SET is_default = FALSE
                        WHERE user_id = %s AND purpose = %s AND id <> %s
                        """,
                        (
                            str(payload.get("user_id") or ""),
                            str(payload.get("purpose") or ""),
                            str(payload.get("id") or ""),
                        ),
                    )
                cur.execute(
                    f"""
                    INSERT INTO {self._table_name("model_profiles")} (
                        id, user_id, purpose, provider, name, base_url, api_key_ref, model_name,
                        temperature, max_tokens, is_default, updated_at
                    ) VALUES (
                        %s, %s, %s, %s, %s, %s, %s, %s,
                        %s, %s, %s, %s
                    )
                    ON CONFLICT (id) DO UPDATE SET
                        user_id = EXCLUDED.user_id,
                        purpose = EXCLUDED.purpose,
                        provider = EXCLUDED.provider,
                        name = EXCLUDED.name,
                        base_url = EXCLUDED.base_url,
                        api_key_ref = EXCLUDED.api_key_ref,
                        model_name = EXCLUDED.model_name,
                        temperature = EXCLUDED.temperature,
                        max_tokens = EXCLUDED.max_tokens,
                        is_default = EXCLUDED.is_default,
                        updated_at = EXCLUDED.updated_at
                    """,
                    (
                        str(payload.get("id") or ""),
                        str(payload.get("user_id") or ""),
                        str(payload.get("purpose") or ""),
                        str(payload.get("provider") or ""),
                        str(payload.get("name") or ""),
                        str(payload.get("base_url") or ""),
                        str(payload.get("api_key_ref") or ""),
                        str(payload.get("model_name") or ""),
                        float(payload.get("temperature") or 0.2),
                        int(payload.get("max_tokens") or 4096),
                        bool(payload.get("is_default", False)),
                        str(payload.get("updated_at") or ""),
                    ),
                )
            conn.commit()

    def get_model_profile(self, user_id: str, profile_id: str) -> Optional[Dict[str, Any]]:
        self.ensure_schema()
        with self._connect() as conn:
            with conn.cursor() as cur:
                cur.execute(
                    f"""
                    SELECT id, user_id, purpose, provider, name, base_url, api_key_ref, model_name,
                           temperature, max_tokens, is_default, updated_at
                    FROM {self._table_name("model_profiles")}
                    WHERE id = %s AND user_id IN (%s, '')
                    ORDER BY CASE WHEN user_id = %s THEN 0 ELSE 1 END
                    LIMIT 1
                    """,
                    (profile_id, user_id, user_id),
                )
                row = cur.fetchone()
                return dict(row) if row else None

    def find_default_model_profile(self, user_id: str, purpose: str) -> Optional[Dict[str, Any]]:
        self.ensure_schema()
        with self._connect() as conn:
            with conn.cursor() as cur:
                cur.execute(
                    f"""
                    SELECT id, user_id, purpose, provider, name, base_url, api_key_ref, model_name,
                           temperature, max_tokens, is_default, updated_at
                    FROM {self._table_name("model_profiles")}
                    WHERE purpose = %s AND is_default = TRUE AND user_id IN (%s, '')
                    ORDER BY CASE WHEN user_id = %s THEN 0 ELSE 1 END
                    LIMIT 1
                    """,
                    (purpose, user_id, user_id),
                )
                row = cur.fetchone()
                return dict(row) if row else None

    def list_model_profiles(self, user_id: str) -> List[Dict[str, Any]]:
        self.ensure_schema()
        with self._connect() as conn:
            with conn.cursor() as cur:
                cur.execute(
                    f"""
                    SELECT id, user_id, purpose, provider, name, base_url, api_key_ref, model_name,
                           temperature, max_tokens, is_default, updated_at
                    FROM {self._table_name("model_profiles")}
                    WHERE user_id IN (%s, '')
                    ORDER BY purpose ASC, name ASC
                    """,
                    (user_id,),
                )
                return [dict(row) for row in cur.fetchall()]

    def delete_model_profile(self, user_id: str, profile_id: str) -> None:
        self.ensure_schema()
        with self._connect() as conn:
            with conn.cursor() as cur:
                cur.execute(
                    f"""
                    DELETE FROM {self._table_name("model_profiles")}
                    WHERE id = %s AND user_id = %s
                    """,
                    (profile_id, user_id),
                )
            conn.commit()

    def _connect(self):
        if not self.is_configured():
            raise IndexingError("postgres backend requires QQA_POSTGRES_DSN")
        if not self.dependency_ready():
            raise IndexingError("psycopg is required for the postgres backend")
        return psycopg.connect(self.dsn, connect_timeout=self.connect_timeout, row_factory=dict_row)

    def _schema_name(self) -> str:
        return f'"{self.schema}"'

    def _table_name(self, table: str) -> str:
        return f'{self._schema_name()}."{table}"'

    def _index_name(self, index: str) -> str:
        return f'"{self.schema}_{index}"'

    def _chunks_ddl(self) -> str:
        if self._vector_mode == "native":
            return f"""
                CREATE TABLE IF NOT EXISTS {self._table_name("chunks")} (
                    id TEXT PRIMARY KEY,
                    user_id TEXT NOT NULL,
                    scope_id TEXT NOT NULL,
                    document_id TEXT NOT NULL,
                    version_id TEXT NOT NULL,
                    provider TEXT NOT NULL,
                    repo TEXT NOT NULL,
                    title TEXT NOT NULL,
                    heading_path TEXT NOT NULL,
                    source_url TEXT NOT NULL,
                    updated_at TEXT NOT NULL,
                    chunk_index INTEGER NOT NULL,
                    content TEXT NOT NULL,
                    lexical_text TEXT NOT NULL,
                    token_count INTEGER NOT NULL DEFAULT 0,
                    embedding VECTOR({self.embedding_dim}) NOT NULL,
                    FOREIGN KEY (document_id)
                        REFERENCES {self._table_name("documents")} (id)
                        ON DELETE CASCADE
                )
            """
        return f"""
            CREATE TABLE IF NOT EXISTS {self._table_name("chunks")} (
                id TEXT PRIMARY KEY,
                user_id TEXT NOT NULL,
                scope_id TEXT NOT NULL,
                document_id TEXT NOT NULL,
                version_id TEXT NOT NULL,
                provider TEXT NOT NULL,
                repo TEXT NOT NULL,
                title TEXT NOT NULL,
                heading_path TEXT NOT NULL,
                source_url TEXT NOT NULL,
                updated_at TEXT NOT NULL,
                chunk_index INTEGER NOT NULL,
                content TEXT NOT NULL,
                lexical_text TEXT NOT NULL,
                token_count INTEGER NOT NULL DEFAULT 0,
                embedding_json JSONB NOT NULL DEFAULT '[]'::jsonb,
                FOREIGN KEY (document_id)
                    REFERENCES {self._table_name("documents")} (id)
                    ON DELETE CASCADE
            )
        """

    def _chunk_select_sql(self) -> str:
        if self._vector_mode == "native":
            return f"""
                SELECT id, scope_id, user_id, document_id, version_id, provider, repo, title,
                       heading_path, source_url, updated_at, chunk_index, content, lexical_text,
                       token_count, embedding::text AS embedding_text
                FROM {self._table_name("chunks")}
                WHERE user_id = %s AND scope_id = %s
                ORDER BY document_id ASC, chunk_index ASC, id ASC
            """
        return f"""
            SELECT id, scope_id, user_id, document_id, version_id, provider, repo, title,
                   heading_path, source_url, updated_at, chunk_index, content, lexical_text,
                   token_count, embedding_json
            FROM {self._table_name("chunks")}
            WHERE user_id = %s AND scope_id = %s
            ORDER BY document_id ASC, chunk_index ASC, id ASC
        """

    def _search_lexical_candidates(
        self,
        *,
        user_id: str,
        scope_ids: List[str],
        query: str,
        limit: int,
    ) -> List[Dict[str, object]]:
        tokens = self._tokenize_query(query)
        if not tokens:
            return []
        tsquery = self._tsquery_string(tokens)
        if tsquery:
            sql = f"""
                SELECT {self._candidate_columns()}, ts_rank_cd(c.tsv, to_tsquery('simple', %s)) AS score
                FROM {self._table_name("chunks")} c
                WHERE c.user_id = %s
                  AND c.scope_id = ANY(%s)
                  AND c.tsv @@ to_tsquery('simple', %s)
                ORDER BY score DESC, c.updated_at DESC, c.id ASC
                LIMIT %s
            """
            with self._connect() as conn:
                with conn.cursor() as cur:
                    cur.execute(sql, (tsquery, user_id, scope_ids, tsquery, limit))
                    rows = [dict(row) for row in cur.fetchall()]
            if rows:
                return rows
        return self._search_lexical_candidates_fallback(
            user_id=user_id,
            scope_ids=scope_ids,
            tokens=tokens,
            limit=limit,
        )

    def _search_vector_candidates(
        self,
        *,
        user_id: str,
        scope_ids: List[str],
        query_embedding: np.ndarray,
        limit: int,
    ) -> List[Dict[str, object]]:
        if query_embedding.size == 0:
            return []
        if self._vector_mode == "native":
            vector_literal = self._vector_literal(query_embedding[0])
            sql = f"""
                SELECT {self._candidate_columns()}, (1 - (c.embedding <=> %s::vector)) AS score
                FROM {self._table_name("chunks")} c
                WHERE c.user_id = %s
                  AND c.scope_id = ANY(%s)
                ORDER BY c.embedding <=> %s::vector ASC, c.id ASC
                LIMIT %s
            """
            with self._connect() as conn:
                with conn.cursor() as cur:
                    cur.execute(sql, (vector_literal, user_id, scope_ids, vector_literal, limit))
                    return [dict(row) for row in cur.fetchall()]

        # When pgvector is unavailable we still verify the postgres path by
        # reading only scoped chunk embeddings from the database, then doing the
        # coarse vector ranking in Python.
        sql = f"""
            SELECT {self._candidate_columns()}, embedding_json
            FROM {self._table_name("chunks")} c
            WHERE c.user_id = %s
              AND c.scope_id = ANY(%s)
            ORDER BY c.id ASC
        """
        with self._connect() as conn:
            with conn.cursor() as cur:
                cur.execute(sql, (user_id, scope_ids))
                rows = [dict(row) for row in cur.fetchall()]
        if not rows:
            return []
        query_vector = query_embedding[0]
        scored_rows: List[Dict[str, object]] = []
        for row in rows:
            vector = np.asarray(self._parse_embedding_json(row.pop("embedding_json", [])), dtype=np.float32)
            score = float(vector @ query_vector)
            scored_rows.append({**row, "score": score})
        scored_rows.sort(key=lambda item: float(item.get("score") or 0.0), reverse=True)
        return scored_rows[:limit]

    def _candidate_columns(self) -> str:
        return (
            "c.id, c.scope_id, c.user_id, c.document_id, c.version_id, c.provider, "
            "c.repo, c.title, c.heading_path, c.source_url, c.updated_at, c.chunk_index, "
            "c.content, c.lexical_text, c.token_count"
        )

    def _chunk_payload(self, row: Dict[str, object]) -> Dict[str, object]:
        return {
            "id": row.get("id"),
            "scope_id": row.get("scope_id"),
            "user_id": row.get("user_id"),
            "document_id": row.get("document_id"),
            "version_id": row.get("version_id"),
            "provider": row.get("provider"),
            "repo": row.get("repo"),
            "title": row.get("title"),
            "heading_path": row.get("heading_path"),
            "source_url": row.get("source_url"),
            "updated_at": row.get("updated_at"),
            "chunk_index": row.get("chunk_index"),
            "content": row.get("content"),
            "lexical_text": row.get("lexical_text"),
            "token_count": row.get("token_count"),
        }

    def _search_lexical_candidates_fallback(
        self,
        *,
        user_id: str,
        scope_ids: List[str],
        tokens: List[str],
        limit: int,
    ) -> List[Dict[str, object]]:
        score_terms = []
        where_terms = []
        score_params: List[object] = []
        where_params: List[object] = []
        for token in tokens:
            like = f"%{token}%"
            score_terms.append(
                "(CASE WHEN lower(c.title) LIKE %s THEN 3.0 ELSE 0 END"
                " + CASE WHEN lower(c.repo) LIKE %s THEN 1.5 ELSE 0 END"
                " + CASE WHEN lower(c.lexical_text) LIKE %s THEN 1.0 ELSE 0 END)"
            )
            where_terms.append(
                "(lower(c.title) LIKE %s OR lower(c.repo) LIKE %s OR lower(c.lexical_text) LIKE %s)"
            )
            score_params.extend([like, like, like])
            where_params.extend([like, like, like])
        sql = f"""
            SELECT {self._candidate_columns()}, ({' + '.join(score_terms)}) AS score
            FROM {self._table_name("chunks")} c
            WHERE c.user_id = %s
              AND c.scope_id = ANY(%s)
              AND ({' OR '.join(where_terms)})
            ORDER BY score DESC, c.updated_at DESC, c.id ASC
            LIMIT %s
        """
        params = score_params + [user_id, scope_ids] + where_params + [limit]
        with self._connect() as conn:
            with conn.cursor() as cur:
                cur.execute(sql, params)
                return [dict(row) for row in cur.fetchall()]

    def _validate_schema(self, schema: str) -> str:
        candidate = (schema or "").strip()
        if not candidate:
            raise IndexingError("postgres schema name is required")
        if not self._SCHEMA_RE.fullmatch(candidate):
            raise IndexingError("postgres schema name contains invalid characters")
        return candidate

    def _vector_literal(self, values: Iterable[float]) -> str:
        serializable = [f"{float(value):.8f}" for value in values]
        return "[" + ",".join(serializable) + "]"

    def _parse_vector_text(self, raw: str) -> List[float]:
        cleaned = raw.strip().lstrip("[").rstrip("]")
        if not cleaned:
            return [0.0] * self.embedding_dim
        values = [float(item.strip()) for item in cleaned.split(",") if item.strip()]
        if len(values) < self.embedding_dim:
            values.extend([0.0] * (self.embedding_dim - len(values)))
        return values[: self.embedding_dim]

    def _parse_embedding_json(self, payload: Any) -> List[float]:
        if isinstance(payload, list):
            values = [float(item) for item in payload]
        else:
            values = []
        if len(values) < self.embedding_dim:
            values.extend([0.0] * (self.embedding_dim - len(values)))
        return values[: self.embedding_dim]

    def _parse_timestamp(self, value: Any):
        if value in (None, ""):
            return None
        if hasattr(value, "isoformat"):
            return value
        try:
            text = str(value).strip()
            if not text:
                return None
            if text.endswith("Z"):
                text = text[:-1] + "+00:00"
            return datetime.fromisoformat(text)
        except Exception:
            return None

    def _iso_timestamp(self, value: Any) -> str:
        if value is None:
            return ""
        if hasattr(value, "isoformat"):
            return value.isoformat()
        return str(value)

    def _request_trace_payload(self, row: Dict[str, Any]) -> Dict[str, Any]:
        payload = dict(row.get("payload") or {})
        payload.setdefault("trace_id", str(row.get("id") or ""))
        payload.setdefault("id", str(row.get("id") or ""))
        payload.setdefault("request_type", str(row.get("request_type") or ""))
        payload.setdefault("user_id", str(row.get("user_id") or ""))
        payload.setdefault("run_id", str(row.get("run_id") or ""))
        payload.setdefault("message_id", str(row.get("message_id") or ""))
        payload.setdefault("skill_id", str(row.get("skill_id") or ""))
        payload.setdefault("model_profile_id", str(row.get("model_profile_id") or ""))
        payload.setdefault("scope_ids", list(row.get("scope_ids") or []))
        payload.setdefault("query_text", str(row.get("query_text") or ""))
        payload.setdefault("mode", str(row.get("mode") or ""))
        payload.setdefault("retrieved_chunk_ids", list(row.get("retrieved_chunk_ids") or []))
        payload.setdefault("reranked_chunk_ids", list(row.get("reranked_chunk_ids") or []))
        payload.setdefault("used_chunk_ids", list(row.get("used_chunk_ids") or []))
        payload.setdefault("latency_retrieve_ms", int(row.get("latency_retrieve_ms") or 0))
        payload.setdefault("latency_generate_ms", int(row.get("latency_generate_ms") or 0))
        payload.setdefault("latency_total_ms", int(row.get("latency_total_ms") or 0))
        payload.setdefault("retrieval_backend", str(row.get("retrieval_backend") or ""))
        payload.setdefault("candidate_mode", str(row.get("candidate_mode") or ""))
        payload.setdefault("output_preview", str(row.get("output_preview") or ""))
        payload.setdefault("error_code", str(row.get("error_code") or ""))
        payload.setdefault("error_message", str(row.get("error_message") or ""))
        payload.setdefault("created_at", self._iso_timestamp(row.get("created_at")))
        return payload

    def _tokenize_query(self, query: str) -> List[str]:
        return [token.lower() for token in self._TOKEN_RE.findall(query or "")]

    def _lexical_terms(self, text: str) -> str:
        return " ".join(self._tokenize_query(text))

    def _tsquery_string(self, tokens: List[str]) -> str:
        unique_tokens = []
        seen = set()
        for token in tokens:
            if token in seen:
                continue
            seen.add(token)
            unique_tokens.append(token)
        return " | ".join(unique_tokens)

    def _detect_vector_mode(self, cur) -> str:
        try:
            cur.execute("CREATE EXTENSION IF NOT EXISTS vector")
            return "native"
        except Exception:
            cur.connection.rollback()
            return "json_fallback"
