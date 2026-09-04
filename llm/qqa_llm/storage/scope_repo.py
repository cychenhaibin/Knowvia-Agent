from __future__ import annotations

from dataclasses import asdict
from pathlib import Path
from typing import Dict, List, Optional

import numpy as np

from qqa_llm.domain.models import KnowledgeChunk, KnowledgeDocument, ScopeRef
from qqa_llm.storage.file_repo import FileStore


class ScopeRepository:
    def __init__(self, file_store: FileStore, base_dir: Path) -> None:
        self.file_store = file_store
        self.base_dir = base_dir
        self.base_dir.mkdir(parents=True, exist_ok=True)

    def list_scope_ids(self, user_id: str) -> List[str]:
        user_dir = self._safe_path(user_id)
        if not user_dir.exists():
            return []
        return sorted(path.name for path in user_dir.iterdir() if path.is_dir())

    def list_user_ids(self) -> List[str]:
        if not self.base_dir.exists():
            return []
        return sorted(path.name for path in self.base_dir.iterdir() if path.is_dir())

    def delete_scope(self, user_id: str, scope_id: str) -> None:
        scope_dir = self.scope_dir(user_id, scope_id)
        if not scope_dir.exists():
            return
        for path in sorted(scope_dir.rglob("*"), reverse=True):
            if path.is_file() or path.is_symlink():
                path.unlink()
            elif path.is_dir():
                path.rmdir()
        if scope_dir.exists():
            scope_dir.rmdir()

    def save_scope(
        self,
        scope: ScopeRef,
        documents: List[KnowledgeDocument],
        chunks: List[KnowledgeChunk],
        embeddings: np.ndarray,
        compat_meta: Dict[str, object],
    ) -> None:
        scope_dir = self.scope_dir(scope.user_id, scope.scope_id)
        scope_dir.mkdir(parents=True, exist_ok=True)
        self.file_store.write_json(scope_dir / "scope.json", scope.to_dict())
        self.file_store.write_json(scope_dir / "documents.json", [doc.to_dict() for doc in documents])
        self.file_store.write_json(scope_dir / "chunks.json", [chunk.to_dict() for chunk in chunks])
        np.save(scope_dir / "embeddings.npy", embeddings)
        self.file_store.write_json(scope_dir / "meta.json", compat_meta)

    def load_scope(self, user_id: str, scope_id: str) -> Optional[Dict[str, object]]:
        scope_dir = self.scope_dir(user_id, scope_id)
        if not scope_dir.exists():
            return None
        scope_payload = self.file_store.read_json(scope_dir / "scope.json", None)
        if scope_payload is None:
            return None
        documents = self.file_store.read_json(scope_dir / "documents.json", [])
        chunks = self.file_store.read_json(scope_dir / "chunks.json", [])
        embeddings_path = scope_dir / "embeddings.npy"
        embeddings = np.load(embeddings_path) if embeddings_path.exists() else np.zeros((0, 0), dtype=np.float32)
        return {
            "scope": scope_payload,
            "documents": documents,
            "chunks": chunks,
            "embeddings": embeddings,
            "meta": self.file_store.read_json(scope_dir / "meta.json", {}),
        }

    def scope_dir(self, user_id: str, scope_id: str) -> Path:
        return self._safe_path(user_id, scope_id)

    def _safe_path(self, *identifiers: str) -> Path:
        for identifier in identifiers:
            value = str(identifier).strip()
            if (
                not value
                or value in {".", ".."}
                or Path(value).is_absolute()
                or Path(value).name != value
                or "/" in value
                or "\\" in value
            ):
                raise ValueError("scope identifiers must be single path components")

        base = self.base_dir.resolve()
        candidate = base.joinpath(*identifiers).resolve()
        try:
            candidate.relative_to(base)
        except ValueError as exc:
            raise ValueError("scope path escapes the configured base directory") from exc
        if candidate == base:
            raise ValueError("scope path must be below the configured base directory")
        return candidate
