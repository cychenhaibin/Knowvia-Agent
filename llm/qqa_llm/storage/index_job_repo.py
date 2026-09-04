from __future__ import annotations

from pathlib import Path
import threading
from typing import Any, Dict, List, Optional

from qqa_llm.storage.db import DatabaseAdapter
from qqa_llm.storage.file_repo import FileStore


class IndexJobRepository:
    def __init__(self, file_store: FileStore, job_path: Path) -> None:
        self.file_store = file_store
        self.job_path = job_path
        self._lock = threading.RLock()

    def upsert(self, job_id: str, payload: Dict[str, Any]) -> None:
        with self._lock:
            jobs = self.file_store.read_json(self.job_path, {})
            jobs[job_id] = payload
            self.file_store.write_json(self.job_path, jobs)

    def get(self, job_id: str) -> Optional[Dict[str, Any]]:
        with self._lock:
            jobs = self.file_store.read_json(self.job_path, {})
            return jobs.get(job_id)

    def list(
        self,
        *,
        statuses: Optional[List[str]] = None,
        user_id: str = "",
        scope_id: str = "",
        limit: int = 100,
    ) -> List[Dict[str, Any]]:
        with self._lock:
            jobs = self.file_store.read_json(self.job_path, {})
            wanted_statuses = {str(item).strip() for item in (statuses or []) if str(item).strip()}
            items: List[Dict[str, Any]] = []
            for payload in jobs.values():
                if wanted_statuses and str(payload.get("status") or "") not in wanted_statuses:
                    continue
                if user_id and str(payload.get("user_id") or "") != user_id:
                    continue
                if scope_id and str(payload.get("scope_id") or "") != scope_id:
                    continue
                items.append(dict(payload))
            items.sort(key=lambda item: (str(item.get("created_at") or ""), str(item.get("job_id") or "")))
            return items[: max(limit, 0)]


class DatabaseIndexJobRepository:
    """Persists index job snapshots in Postgres for operational visibility."""

    def __init__(self, db: DatabaseAdapter) -> None:
        self.db = db

    def upsert(self, job_id: str, payload: Dict[str, Any]) -> None:
        self.db.upsert_index_job(job_id, payload)

    def get(self, job_id: str) -> Optional[Dict[str, Any]]:
        return self.db.get_index_job(job_id)

    def list(
        self,
        *,
        statuses: Optional[List[str]] = None,
        user_id: str = "",
        scope_id: str = "",
        limit: int = 100,
    ) -> List[Dict[str, Any]]:
        return self.db.list_index_jobs(
            statuses=statuses or [],
            user_id=user_id,
            scope_id=scope_id,
            limit=limit,
        )
