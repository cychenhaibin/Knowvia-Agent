from __future__ import annotations

from typing import Dict, List

from qqa_llm.domain.models import KnowledgeChunk, KnowledgeDocument, ScopeRef
from qqa_llm.index.base import IndexBackend
from qqa_llm.storage.db import DatabaseAdapter


class PGVectorIndexBackend(IndexBackend):
    """Postgres-backed index storage that preserves the existing bundle API.

    The retrieval layer still consumes a scope bundle with chunk rows and a
    dense embedding matrix. This backend persists those structures in Postgres,
    then reconstructs the same shape on load so we can move storage forward
    without forcing a wider retrieval rewrite in the same pass.
    """

    def __init__(self, db: DatabaseAdapter) -> None:
        self.db = db

    def save_scope(
        self,
        scope: ScopeRef,
        documents: List[KnowledgeDocument],
        chunks: List[KnowledgeChunk],
        embeddings,
        compat_meta: Dict[str, object],
    ) -> None:
        self.db.save_scope_bundle(scope, documents, chunks, embeddings, compat_meta)

    def load_scope(self, user_id: str, scope_id: str):
        return self.db.load_scope_bundle(user_id, scope_id)

    def delete_scope(self, user_id: str, scope_id: str) -> None:
        self.db.delete_scope_bundle(user_id, scope_id)

    def list_scope_ids(self, user_id: str) -> List[str]:
        return self.db.list_scope_ids(user_id)

    def list_user_ids(self) -> List[str]:
        return self.db.list_user_ids()

    def backend_name(self) -> str:
        return "postgres_pgvector"

    def health_status(self) -> Dict[str, object]:
        status = self.db.health_status()
        status["backend"] = self.backend_name()
        return status

    def candidate_search(
        self,
        *,
        user_id: str,
        scope_ids: List[str],
        query: str,
        query_embedding,
        lexical_top_k: int,
        vector_top_k: int,
    ):
        # Postgres can cheaply pre-select lexical and vector candidates. The
        # higher-level fusion and rerank stages still stay in Python so chat and
        # report chains keep one consistent decision policy.
        return self.db.search_candidates(
            user_id=user_id,
            scope_ids=scope_ids,
            query=query,
            query_embedding=query_embedding,
            lexical_top_k=lexical_top_k,
            vector_top_k=vector_top_k,
        )
