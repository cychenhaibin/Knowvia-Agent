from __future__ import annotations

from pathlib import Path
from typing import Dict, Optional

from qqa_llm.storage.db import DatabaseAdapter
from qqa_llm.domain.models import SkillMirror
from qqa_llm.storage.file_repo import FileStore


class SkillRepository:
    def __init__(self, file_store: FileStore, skills_path: Path) -> None:
        self.file_store = file_store
        self.skills_path = skills_path

    def upsert(self, skill: SkillMirror) -> None:
        data = self.file_store.read_json(self.skills_path, {})
        user_skills = data.setdefault(skill.user_id, {})
        user_skills[skill.id] = skill.to_dict()
        self.file_store.write_json(self.skills_path, data)

    def delete(self, user_id: str, skill_id: str) -> None:
        data = self.file_store.read_json(self.skills_path, {})
        user_skills = data.get(user_id) or {}
        if skill_id in user_skills:
            del user_skills[skill_id]
        if user_skills:
            data[user_id] = user_skills
        elif user_id in data:
            del data[user_id]
        self.file_store.write_json(self.skills_path, data)

    def get(self, user_id: str, skill_id: str) -> Optional[Dict[str, object]]:
        data = self.file_store.read_json(self.skills_path, {})
        return (data.get(user_id) or {}).get(skill_id)


class DatabaseSkillRepository:
    def __init__(self, db: DatabaseAdapter) -> None:
        self.db = db

    def upsert(self, skill: SkillMirror) -> None:
        self.db.upsert_skill_mirror(skill.to_dict())

    def delete(self, user_id: str, skill_id: str) -> None:
        self.db.delete_skill_mirror(user_id, skill_id)

    def get(self, user_id: str, skill_id: str) -> Optional[Dict[str, object]]:
        return self.db.get_skill_mirror(user_id, skill_id)
