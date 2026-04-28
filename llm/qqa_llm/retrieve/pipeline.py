from __future__ import annotations

from collections import defaultdict
from time import perf_counter
from typing import Dict, List

from qqa_llm.core.config import Settings
from qqa_llm.domain.models import RetrievalDiagnostics, RetrievalResult, RetrievedPassage
from qqa_llm.inference.embeddings import EmbeddingClient
from qqa_llm.inference.reranker import RerankClient
from qqa_llm.retrieve.fusion import reciprocal_rank_fusion
from qqa_llm.retrieve.lexical import LexicalRetriever
from qqa_llm.retrieve.rerank import RerankStage
from qqa_llm.retrieve.vector import VectorRetriever


class RetrievalPipeline:
    def __init__(
        self,
        settings: Settings,
        embedding_client: EmbeddingClient,
        rerank_client: RerankClient,
    ) -> None:
        self.settings = settings
        self.lexical = LexicalRetriever()
        self.vector = VectorRetriever(embedding_client)
        self.rerank = RerankStage(rerank_client)

    def embed_query(self, query: str):
        return self.vector.embedding_client.embed_texts([query])

    def search(self, *, query: str, bundles: List[Dict[str, object]], top_k: int = 0) -> RetrievalResult:
        # Each scope bundle already contains the normalized chunks and their
        # stored embeddings. Retrieval works completely inside the llm service
        # so Go only needs to pass user/scope boundaries and the query.
        chunk_by_id: Dict[str, Dict[str, object]] = {}
        rankings: List[List[Dict[str, object]]] = []
        lexical_candidates: List[str] = []
        vector_candidates: List[str] = []
        lexical_elapsed = 0.0
        vector_elapsed = 0.0

        for bundle in bundles:
            chunks = list(bundle.get("chunks") or [])
            embeddings = bundle.get("embeddings")
            for chunk in chunks:
                chunk_by_id[str(chunk.get("id") or "")] = chunk
            lexical_started = perf_counter()
            lexical_ranking = self.lexical.score(query, chunks)[: self.settings.lexical_top_k]
            lexical_elapsed += perf_counter() - lexical_started
            lexical_candidates.extend(str(item.get("chunk_id") or "") for item in lexical_ranking if str(item.get("chunk_id") or ""))
            vector_started = perf_counter()
            vector_ranking = self.vector.score(query, chunks, embeddings)[: self.settings.vector_top_k]
            vector_elapsed += perf_counter() - vector_started
            vector_candidates.extend(str(item.get("chunk_id") or "") for item in vector_ranking if str(item.get("chunk_id") or ""))
            rankings.extend([lexical_ranking, vector_ranking])

        return self._search_from_rankings(
            query=query,
            chunk_by_id=chunk_by_id,
            rankings=rankings,
            top_k=top_k,
            diagnostics=RetrievalDiagnostics(
                candidate_mode="python_full_scan",
                lexical_candidate_ids=lexical_candidates,
                vector_candidate_ids=vector_candidates,
                latency_lexical_ms=int(lexical_elapsed * 1000),
                latency_vector_ms=int(vector_elapsed * 1000),
            ),
        )

    def search_prefetched(self, *, query: str, candidate_payload: Dict[str, object], top_k: int = 0) -> RetrievalResult:
        chunk_by_id = {
            str(chunk.get("id") or ""): chunk
            for chunk in list(candidate_payload.get("chunks") or [])
            if str(chunk.get("id") or "")
        }
        rankings = [
            list(candidate_payload.get("lexical_ranking") or []),
            list(candidate_payload.get("vector_ranking") or []),
        ]
        meta = dict(candidate_payload.get("meta") or {})
        return self._search_from_rankings(
            query=query,
            chunk_by_id=chunk_by_id,
            rankings=rankings,
            top_k=top_k,
            diagnostics=RetrievalDiagnostics(
                candidate_mode=str(meta.get("candidate_mode") or "prefetched"),
                lexical_candidate_ids=[
                    str(item.get("chunk_id") or "") for item in rankings[0] if str(item.get("chunk_id") or "")
                ],
                vector_candidate_ids=[
                    str(item.get("chunk_id") or "") for item in rankings[1] if str(item.get("chunk_id") or "")
                ],
                latency_lexical_ms=int(meta.get("latency_lexical_ms") or 0),
                latency_vector_ms=int(meta.get("latency_vector_ms") or 0),
            ),
        )

    def _search_from_rankings(
        self,
        *,
        query: str,
        chunk_by_id: Dict[str, Dict[str, object]],
        rankings: List[List[Dict[str, object]]],
        top_k: int,
        diagnostics: RetrievalDiagnostics,
    ) -> RetrievalResult:
        fusion_started = perf_counter()
        fused = reciprocal_rank_fusion(rankings)[: self.settings.fusion_top_k]
        diagnostics.latency_fusion_ms = int((perf_counter() - fusion_started) * 1000)
        diagnostics.fused_chunk_ids = [
            str(item.get("chunk_id") or "") for item in fused if str(item.get("chunk_id") or "")
        ]
        candidates: List[Dict[str, object]] = []
        for item in fused:
            chunk = chunk_by_id.get(str(item.get("chunk_id") or ""))
            if not chunk:
                continue
            candidates.append(
                {
                    "chunk_id": chunk.get("id"),
                    "document_id": chunk.get("document_id"),
                    "provider": chunk.get("provider"),
                    "title": chunk.get("title"),
                    "url": chunk.get("source_url"),
                    "repo": chunk.get("repo"),
                    "scope_id": chunk.get("scope_id"),
                    "text": chunk.get("content"),
                    "score": item.get("score", 0.0),
                }
            )

        rerank_started = perf_counter()
        reranked = self.rerank.rerank(query, candidates)[: self.settings.rerank_top_k]
        diagnostics.latency_rerank_ms = int((perf_counter() - rerank_started) * 1000)
        diagnostics.reranked_chunk_ids = [
            str(item.get("chunk_id") or "") for item in reranked if str(item.get("chunk_id") or "")
        ]
        passages = self._to_passages(query, reranked, top_k=top_k)
        diagnostics.used_chunk_ids = [passage.chunk_id for passage in passages if passage.chunk_id]
        diagnostics.chunk_catalog = self._build_chunk_catalog(
            chunk_by_id=chunk_by_id,
            chunk_ids=
            diagnostics.lexical_candidate_ids
            + diagnostics.vector_candidate_ids
            + diagnostics.fused_chunk_ids
            + diagnostics.reranked_chunk_ids
            + diagnostics.used_chunk_ids,
        )
        return RetrievalResult(query=query, passages=passages, diagnostics=diagnostics)

    def _to_passages(self, query: str, items: List[Dict[str, object]], *, top_k: int) -> List[RetrievedPassage]:
        by_document = defaultdict(int)
        passages: List[RetrievedPassage] = []
        max_sources = top_k if top_k > 0 else self.settings.max_sources
        for item in items:
            document_id = str(item.get("document_id") or "")
            if by_document[document_id] >= 2:
                continue
            by_document[document_id] += 1
            content = str(item.get("text") or "")
            passages.append(
                RetrievedPassage(
                    chunk_id=str(item.get("chunk_id") or ""),
                    scope_id=str(item.get("scope_id") or ""),
                    connection_id=str(item.get("scope_id") or ""),
                    document_id=document_id,
                    provider=str(item.get("provider") or "knowledge"),
                    title=str(item.get("title") or "Untitled"),
                    url=str(item.get("url") or ""),
                    repo=str(item.get("repo") or ""),
                    snippet=self._snippet(content, query, self.settings.max_snippet_chars),
                    content=content,
                    score=float(item.get("rerank_score") or item.get("score") or 0.0),
                )
            )
            if len(passages) >= max_sources:
                break
        return passages

    def _snippet(self, content: str, query: str, limit: int) -> str:
        content = (content or "").strip()
        if len(content) <= limit:
            return content
        lowered = content.lower()
        needle = (query or "").strip().lower()
        start = lowered.find(needle) if needle else -1
        if start < 0:
            return content[:limit] + "..."
        snippet_start = max(0, start - limit // 4)
        snippet_end = min(len(content), snippet_start + limit)
        prefix = "..." if snippet_start > 0 else ""
        suffix = "..." if snippet_end < len(content) else ""
        return prefix + content[snippet_start:snippet_end] + suffix

    def _build_chunk_catalog(
        self,
        *,
        chunk_by_id: Dict[str, Dict[str, object]],
        chunk_ids: List[str],
    ) -> Dict[str, Dict[str, str]]:
        catalog: Dict[str, Dict[str, str]] = {}
        for chunk_id in chunk_ids:
            normalized_id = str(chunk_id or "").strip()
            if not normalized_id or normalized_id in catalog:
                continue
            chunk = chunk_by_id.get(normalized_id) or {}
            catalog[normalized_id] = {
                "title": str(chunk.get("title") or "").strip(),
                "repo": str(chunk.get("repo") or "").strip(),
                "provider": str(chunk.get("provider") or "").strip(),
                "url": str(chunk.get("source_url") or "").strip(),
                "scope_id": str(chunk.get("scope_id") or "").strip(),
            }
        return catalog
