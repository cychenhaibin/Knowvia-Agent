from __future__ import annotations

from qqa_llm.index.base import IndexBackend


class ScopeService:
    def __init__(self, index_backend: IndexBackend) -> None:
        self.index_backend = index_backend

    def list_scope_ids(self, user_id: str) -> list[str]:
        return self.index_backend.list_scope_ids(user_id)

    def list_user_ids(self) -> list[str]:
        return self.index_backend.list_user_ids()

    def delete_scope(self, user_id: str, scope_id: str) -> None:
        self.index_backend.delete_scope(user_id, scope_id)
