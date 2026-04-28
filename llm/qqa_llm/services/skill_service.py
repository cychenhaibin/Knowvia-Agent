from __future__ import annotations

from typing import Any, Dict, Tuple

from qqa_llm.storage.skill_repo import SkillRepository


class SkillService:
    def __init__(self, skill_repo: SkillRepository) -> None:
        self.skill_repo = skill_repo

    def upsert(self, payload: Dict[str, Any]) -> None:
        from qqa_llm.domain.models import SkillMirror

        self.skill_repo.upsert(SkillMirror(**payload))

    def delete(self, user_id: str, skill_id: str) -> None:
        self.skill_repo.delete(user_id, skill_id)

    def resolve(
        self,
        *,
        user_id: str,
        skill_id: str,
        requested_mode: str,
        requested_prompt: str,
        skill_snapshot: Dict[str, Any],
    ) -> Tuple[str, str]:
        snapshot_mode = ""
        snapshot_prompt = ""
        runtime_spec = {}
        if isinstance(skill_snapshot, dict):
            runtime_spec = skill_snapshot.get("runtime_spec") or {}
            if isinstance(runtime_spec, dict):
                snapshot_mode = str(
                    runtime_spec.get("responseMode") or runtime_spec.get("response_mode") or ""
                ).strip()
                snapshot_prompt = str(runtime_spec.get("instructions") or "").strip()
            if not snapshot_mode:
                snapshot_mode = str(skill_snapshot.get("mode") or "").strip()
            if not snapshot_prompt:
                snapshot_prompt = str(skill_snapshot.get("prompt") or "").strip()

        skill = self.skill_repo.get(user_id, skill_id) if skill_id else None
        if skill and not bool(skill.get("enabled", True)):
            skill = None
        mode = snapshot_mode or requested_mode or str((skill or {}).get("mode") or "answer")
        prompt = snapshot_prompt or requested_prompt or str((skill or {}).get("prompt") or "")
        return self._normalize_mode(mode), prompt.strip()

    def _normalize_mode(self, mode: str) -> str:
        value = (mode or "").strip().lower()
        if value in {"summary", "actions", "report"}:
            return value
        return "answer"
