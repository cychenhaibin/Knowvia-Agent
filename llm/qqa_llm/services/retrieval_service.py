from __future__ import annotations

from time import perf_counter
from typing import List

from qqa_llm.domain.models import RetrievalResult
from qqa_llm.index.base import IndexBackend
from qqa_llm.retrieve.pipeline import RetrievalPipeline


class RetrievalService:
    def __init__(self, index_backend: IndexBackend, retrieval_pipeline: RetrievalPipeline) -> None:
        self.index_backend = index_backend
        self.retrieval_pipeline = retrieval_pipeline

    def retrieve(self, *, user_id: str, scope_ids: List[str], query: str, top_k: int = 0) -> RetrievalResult:
        # An empty scope list means "search every scope owned by this user".
        # This keeps the API ergonomic for chat/report callers while still
        # enforcing user-level isolation at the index backend boundary.
        started = perf_counter()
        resolved_scope_ids = scope_ids or self.index_backend.list_scope_ids(user_id)
        query_embedding = self.retrieval_pipeline.embed_query(query)
        prefetched = self.index_backend.candidate_search(
            user_id=user_id,
            scope_ids=resolved_scope_ids,
            query=query,
            query_embedding=query_embedding,
            lexical_top_k=self.retrieval_pipeline.settings.lexical_top_k,
            vector_top_k=self.retrieval_pipeline.settings.vector_top_k,
        )
        if prefetched is not None:
            result = self.retrieval_pipeline.search_prefetched(query=query, candidate_payload=prefetched, top_k=top_k)
            result.diagnostics.backend = self.index_backend.backend_name()
            result.diagnostics.latency_total_ms = int((perf_counter() - started) * 1000)
            return result
        bundles = []
        for scope_id in resolved_scope_ids:
            bundle = self.index_backend.load_scope(user_id, scope_id)
            if bundle is not None:
                bundles.append(bundle)
        result = self.retrieval_pipeline.search(query=query, bundles=bundles, top_k=top_k)
        result.diagnostics.backend = self.index_backend.backend_name()
        result.diagnostics.latency_total_ms = int((perf_counter() - started) * 1000)
        return result
