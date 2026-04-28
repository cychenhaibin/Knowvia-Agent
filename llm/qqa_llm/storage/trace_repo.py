from __future__ import annotations

import json
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Any, Dict, List, Optional

from qqa_llm.storage.db import DatabaseAdapter
from qqa_llm.storage.file_repo import FileStore


class TraceRepository:
    def __init__(self, file_store: FileStore, trace_path: Path) -> None:
        self.file_store = file_store
        self.trace_path = trace_path

    def append(self, payload: Dict[str, Any]) -> None:
        self.file_store.append_jsonl(self.trace_path, payload)

    def create_trace(self, payload: Dict[str, Any]) -> Dict[str, Any]:
        self.append(payload)
        return payload

    def get(self, trace_id: str) -> Optional[Dict[str, Any]]:
        if not self.trace_path.exists():
            return None
        for line in reversed(self.trace_path.read_text(encoding="utf-8").splitlines()):
            if not line.strip():
                continue
            payload = json.loads(line)
            if str(payload.get("trace_id") or payload.get("id") or "") == trace_id:
                return payload
        return None

    def list(
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
        if not self.trace_path.exists():
            return []
        items: List[Dict[str, Any]] = []
        seen_trace_ids = set()
        for line in reversed(self.trace_path.read_text(encoding="utf-8").splitlines()):
            if not line.strip():
                continue
            payload = json.loads(line)
            trace_id = str(payload.get("trace_id") or payload.get("id") or "").strip()
            if not trace_id or trace_id in seen_trace_ids:
                continue
            if str(payload.get("user_id") or "") != user_id:
                continue
            if request_type and str(payload.get("request_type") or "") != request_type:
                continue
            if run_id and str(payload.get("run_id") or "") != run_id:
                continue
            if message_id and str(payload.get("message_id") or "") != message_id:
                continue
            if skill_id and str(payload.get("skill_id") or "") != skill_id:
                continue
            if model_profile_id and str(payload.get("model_profile_id") or "") != model_profile_id:
                continue
            seen_trace_ids.add(trace_id)
            items.append(payload)
            if len(items) >= limit:
                break
        return items

    def update_trace_metrics(self, trace_ref: str, /, **updates: Any) -> Dict[str, Any]:
        payload = self.get(trace_ref) or {"id": trace_ref, "trace_id": trace_ref}
        payload.update({key: value for key, value in updates.items() if value is not None})
        self.append(payload)
        return payload

    def mark_trace_error(
        self,
        trace_ref: str,
        *,
        code: str,
        message: str,
        updates: Optional[Dict[str, Any]] = None,
    ) -> Dict[str, Any]:
        payload = self.get(trace_ref) or {"id": trace_ref, "trace_id": trace_ref}
        if updates:
            payload.update(updates)
        payload["error_code"] = code
        payload["error_message"] = message
        self.append(payload)
        return payload

    def cleanup_retention(
        self,
        *,
        trace_retention_days: int,
        report_trace_retention_days: int,
    ) -> Dict[str, int]:
        if not self.trace_path.exists():
            return {"removed": 0, "kept": 0}
        now = datetime.now(timezone.utc)
        default_cutoff = now - timedelta(days=max(trace_retention_days, 0))
        report_cutoff = now - timedelta(days=max(report_trace_retention_days, 0))
        kept_lines: List[str] = []
        removed = 0
        for raw_line in self.trace_path.read_text(encoding="utf-8").splitlines():
            if not raw_line.strip():
                continue
            payload = json.loads(raw_line)
            created_at = self._parse_created_at(str(payload.get("created_at") or ""))
            if created_at is None:
                kept_lines.append(raw_line)
                continue
            request_type = str(payload.get("request_type") or "").strip()
            cutoff = report_cutoff if request_type in {"report", "quality_benchmark"} else default_cutoff
            if created_at < cutoff:
                removed += 1
                continue
            kept_lines.append(raw_line)
        rewritten = "\n".join(kept_lines)
        if rewritten:
            rewritten += "\n"
        self.trace_path.write_text(rewritten, encoding="utf-8")
        return {"removed": removed, "kept": len(kept_lines)}

    def _parse_created_at(self, raw: str) -> Optional[datetime]:
        value = raw.strip()
        if not value:
            return None
        try:
            parsed = datetime.fromisoformat(value.replace("Z", "+00:00"))
        except ValueError:
            return None
        if parsed.tzinfo is None:
            return parsed.replace(tzinfo=timezone.utc)
        return parsed.astimezone(timezone.utc)


class DatabaseTraceRepository:
    """Writes llm traces to Postgres without changing trace payload shape."""

    def __init__(self, db: DatabaseAdapter) -> None:
        self.db = db

    def append(self, payload: Dict[str, Any]) -> None:
        self.db.append_trace(payload)

    def create_trace(self, payload: Dict[str, Any]) -> Dict[str, Any]:
        self.append(payload)
        return payload

    def get(self, trace_id: str) -> Optional[Dict[str, Any]]:
        return self.db.get_request_trace(trace_id)

    def list(
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
        return self.db.list_request_traces(
            user_id=user_id,
            limit=limit,
            request_type=request_type,
            run_id=run_id,
            message_id=message_id,
            skill_id=skill_id,
            model_profile_id=model_profile_id,
        )

    def update_trace_metrics(self, trace_ref: str, /, **updates: Any) -> Dict[str, Any]:
        payload = self.get(trace_ref) or {"id": trace_ref, "trace_id": trace_ref}
        payload.update({key: value for key, value in updates.items() if value is not None})
        self.append(payload)
        return payload

    def mark_trace_error(
        self,
        trace_ref: str,
        *,
        code: str,
        message: str,
        updates: Optional[Dict[str, Any]] = None,
    ) -> Dict[str, Any]:
        payload = self.get(trace_ref) or {"id": trace_ref, "trace_id": trace_ref}
        if updates:
            payload.update(updates)
        payload["error_code"] = code
        payload["error_message"] = message
        self.append(payload)
        return payload

    def cleanup_retention(
        self,
        *,
        trace_retention_days: int,
        report_trace_retention_days: int,
    ) -> Dict[str, int]:
        return self.db.cleanup_request_traces(
            trace_retention_days=trace_retention_days,
            report_trace_retention_days=report_trace_retention_days,
        )
