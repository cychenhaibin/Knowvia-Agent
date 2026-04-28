from __future__ import annotations

from typing import Dict, List, Protocol

from qqa_llm.domain.models import KnowledgeChunk, KnowledgeDocument, ScopeRef


class IndexBackend(Protocol):
    def save_scope(
        self,
        scope: ScopeRef,
        documents: List[KnowledgeDocument],
        chunks: List[KnowledgeChunk],
        embeddings,
        compat_meta: Dict[str, object],
    ) -> None:
        ...

    def load_scope(self, user_id: str, scope_id: str):
        ...

    def delete_scope(self, user_id: str, scope_id: str) -> None:
        ...

    def list_scope_ids(self, user_id: str) -> List[str]:
        ...

    def list_user_ids(self) -> List[str]:
        ...

    def backend_name(self) -> str:
        ...

    def health_status(self) -> Dict[str, object]:
        ...

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
        ...
