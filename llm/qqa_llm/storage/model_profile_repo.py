from __future__ import annotations

from pathlib import Path
from typing import Dict, List, Optional

from qqa_llm.domain.models import ModelProfile
from qqa_llm.storage.db import DatabaseAdapter
from qqa_llm.storage.file_repo import FileStore


class ModelProfileRepository:
    def __init__(self, file_store: FileStore, profiles_path: Path) -> None:
        self.file_store = file_store
        self.profiles_path = profiles_path

    def upsert(self, profile: ModelProfile) -> None:
        payload = self.file_store.read_json(self.profiles_path, {})
        user_profiles = payload.setdefault(profile.user_id or "__system__", {})
        user_profiles[profile.id] = profile.to_dict()
        self.file_store.write_json(self.profiles_path, payload)

    def get(self, user_id: str, profile_id: str) -> Optional[Dict[str, object]]:
        payload = self.file_store.read_json(self.profiles_path, {})
        for bucket in (user_id or "__system__", "__system__"):
            profile = (payload.get(bucket) or {}).get(profile_id)
            if profile:
                return profile
        return None

    def find_default(self, user_id: str, purpose: str) -> Optional[Dict[str, object]]:
        payload = self.file_store.read_json(self.profiles_path, {})
        for bucket in (user_id or "__system__", "__system__"):
            for profile in (payload.get(bucket) or {}).values():
                if profile.get("purpose") == purpose and bool(profile.get("is_default", False)):
                    return profile
        return None

    def list(self, user_id: str) -> List[Dict[str, object]]:
        payload = self.file_store.read_json(self.profiles_path, {})
        items = list((payload.get("__system__") or {}).values())
        if user_id:
            items.extend((payload.get(user_id) or {}).values())
        return items

    def delete(self, user_id: str, profile_id: str) -> None:
        payload = self.file_store.read_json(self.profiles_path, {})
        bucket = user_id or "__system__"
        profiles = dict(payload.get(bucket) or {})
        if profile_id in profiles:
            del profiles[profile_id]
        if profiles:
            payload[bucket] = profiles
        elif bucket in payload:
            del payload[bucket]
        self.file_store.write_json(self.profiles_path, payload)


class DatabaseModelProfileRepository:
    def __init__(self, db: DatabaseAdapter) -> None:
        self.db = db

    def upsert(self, profile: ModelProfile) -> None:
        self.db.upsert_model_profile(profile.to_dict())

    def get(self, user_id: str, profile_id: str) -> Optional[Dict[str, object]]:
        return self.db.get_model_profile(user_id, profile_id)

    def find_default(self, user_id: str, purpose: str) -> Optional[Dict[str, object]]:
        return self.db.find_default_model_profile(user_id, purpose)

    def list(self, user_id: str) -> List[Dict[str, object]]:
        return self.db.list_model_profiles(user_id)

    def delete(self, user_id: str, profile_id: str) -> None:
        self.db.delete_model_profile(user_id, profile_id)
