from __future__ import annotations

from typing import Dict, List

from qqa_llm.domain.models import KnowledgeChunk, KnowledgeDocument, ScopeRef
from qqa_llm.index.base import IndexBackend
from qqa_llm.storage.scope_repo import ScopeRepository


class FileIndexBackend(IndexBackend):
    def __init__(self, scope_repo: ScopeRepository) -> None:
        self.scope_repo = scope_repo

    def save_scope(
        self,
        scope: ScopeRef,
        documents: List[KnowledgeDocument],
        chunks: List[KnowledgeChunk],
        embeddings,
        compat_meta: Dict[str, object],
    ) -> None:
        self.scope_repo.save_scope(scope, documents, chunks, embeddings, compat_meta)

    def load_scope(self, user_id: str, scope_id: str):
        return self.scope_repo.load_scope(user_id, scope_id)

    def delete_scope(self, user_id: str, scope_id: str) -> None:
        self.scope_repo.delete_scope(user_id, scope_id)

    def list_scope_ids(self, user_id: str) -> List[str]:
        return self.scope_repo.list_scope_ids(user_id)

    def list_user_ids(self) -> List[str]:
        return self.scope_repo.list_user_ids()

    def backend_name(self) -> str:
        return "file"

    def health_status(self) -> Dict[str, object]:
        return {
            "backend": "file",
            "status": "ok",
            "message": "File-backed scope bundles are available.",
        }

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
        # File storage has no lower-level search primitive, so callers should
        # fall back to loading full scope bundles and running retrieval in Python.
        return None
