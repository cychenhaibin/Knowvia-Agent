from __future__ import annotations

from datetime import datetime, timezone
import hashlib
from typing import Any, Dict, Optional

from qqa_llm.core.config import Settings
from qqa_llm.domain.models import ModelProfile, ResolvedModelProfile


class ModelProfileService:
    def __init__(self, settings: Settings, profile_repo) -> None:
        self.settings = settings
        self.profile_repo = profile_repo
        self._ensure_default_profiles()

    def resolve(
        self,
        *,
        user_id: str,
        purpose: str,
        requested_profile: Optional[Dict[str, Any]] = None,
        legacy_model: str = "",
        legacy_api_base: str = "",
        legacy_api_key: str = "",
    ) -> ResolvedModelProfile:
        requested_profile = dict(requested_profile or {})
        effective_purpose = str(
            requested_profile.get("purpose")
            or requested_profile.get("response_purpose")
            or requested_profile.get("responsePurpose")
            or purpose
            or "chat_fast"
        ).strip() or "chat_fast"
        profile_id = str(
            requested_profile.get("profile_id")
            or requested_profile.get("profileId")
            or ""
        ).strip()
        explicit_provider = str(requested_profile.get("provider") or "").strip().lower()
        explicit_model = str(
            requested_profile.get("model_name")
            or requested_profile.get("modelName")
            or legacy_model
            or ""
        ).strip()
        explicit_base_url = str(
            requested_profile.get("base_url")
            or requested_profile.get("baseUrl")
            or legacy_api_base
            or ""
        ).strip()
        explicit_api_key = str(
            requested_profile.get("api_key")
            or requested_profile.get("apiKey")
            or legacy_api_key
            or ""
        ).strip()

        stored = None
        if profile_id:
            stored = self.profile_repo.get(user_id, profile_id)
        if stored is None and not any([explicit_provider, explicit_model, explicit_base_url, explicit_api_key]):
            stored = self.profile_repo.find_default(user_id, effective_purpose)

        if stored is not None:
            return self._resolved_from_stored(stored, explicit_api_key=explicit_api_key)

        provider = explicit_provider or self._default_provider(explicit_base_url, explicit_api_key)
        model_name = explicit_model or self._default_model_name(provider, effective_purpose)
        profile_key = profile_id or f"adhoc_{effective_purpose}_{provider}"
        name = str(requested_profile.get("name") or requested_profile.get("title") or profile_key).strip() or profile_key
        requested_temperature = requested_profile.get("temperature")
        return ResolvedModelProfile(
            id=profile_key,
            purpose=effective_purpose,
            provider=provider,
            name=name,
            base_url=explicit_base_url or self._default_base_url(provider),
            api_key=explicit_api_key or self._default_api_key(provider),
            model_name=model_name,
            temperature=float(requested_temperature) if requested_temperature is not None else self._default_temperature(effective_purpose),
            max_tokens=int(requested_profile.get("max_tokens") or requested_profile.get("maxTokens") or 4096),
        )

    def upsert(self, user_id: str, payload: Dict[str, Any]) -> Dict[str, Any]:
        now = datetime.now(timezone.utc).isoformat()
        purpose = str(payload.get("purpose") or "").strip() or "chat_fast"
        provider = str(payload.get("provider") or "").strip().lower() or "fallback"
        name = str(payload.get("name") or payload.get("title") or "").strip() or f"{purpose}-{provider}"
        profile_id = str(payload.get("id") or payload.get("profile_id") or payload.get("profileId") or "").strip()
        if not profile_id:
            digest = hashlib.sha1(f"{user_id}:{purpose}:{name}".encode("utf-8")).hexdigest()[:12]
            profile_id = f"profile_{purpose}_{digest}"
        profile = ModelProfile(
            id=profile_id,
            user_id=user_id,
            purpose=purpose,
            provider=provider,
            name=name,
            base_url=str(payload.get("base_url") or payload.get("baseUrl") or self._default_base_url(provider)).strip(),
            api_key_ref=str(payload.get("api_key_ref") or payload.get("apiKeyRef") or "").strip(),
            model_name=str(payload.get("model_name") or payload.get("modelName") or self._default_model_name(provider, purpose)).strip(),
            temperature=float(payload["temperature"]) if payload.get("temperature") is not None else self._default_temperature(purpose),
            max_tokens=int(payload.get("max_tokens") or payload.get("maxTokens") or 4096),
            is_default=bool(payload.get("is_default", payload.get("isDefault", False))),
            updated_at=str(payload.get("updated_at") or payload.get("updatedAt") or now),
        )
        self.profile_repo.upsert(profile)
        return profile.to_dict()

    def list(self, user_id: str) -> list[Dict[str, Any]]:
        return [dict(item) for item in self.profile_repo.list(user_id)]

    def get(self, user_id: str, profile_id: str) -> Optional[Dict[str, Any]]:
        payload = self.profile_repo.get(user_id, profile_id)
        return dict(payload) if payload is not None else None

    def delete(self, user_id: str, profile_id: str) -> bool:
        existing = self.profile_repo.get(user_id, profile_id)
        if existing is None:
            return False
        self.profile_repo.delete(user_id, profile_id)
        return True

    def profile_statuses(self, user_id: str) -> Dict[str, Dict[str, Any]]:
        statuses: Dict[str, Dict[str, Any]] = {}
        for purpose in ("chat_fast", "chat_quality", "report_writer"):
            resolved = self.resolve(user_id=user_id, purpose=purpose)
            statuses[purpose] = {
                "profile_id": resolved.id,
                "purpose": resolved.purpose,
                "provider": resolved.provider,
                "name": resolved.name,
                "model_name": resolved.model_name,
                "base_url": resolved.base_url,
                "is_default": bool((self.profile_repo.get(user_id, resolved.id) or {}).get("is_default", False)),
            }
        return statuses

    def _resolved_from_stored(self, payload: Dict[str, Any], *, explicit_api_key: str) -> ResolvedModelProfile:
        provider = str(payload.get("provider") or "fallback").strip() or "fallback"
        return ResolvedModelProfile(
            id=str(payload.get("id") or ""),
            purpose=str(payload.get("purpose") or "chat_fast"),
            provider=provider,
            name=str(payload.get("name") or payload.get("id") or "profile"),
            base_url=str(payload.get("base_url") or ""),
            api_key=explicit_api_key or self._resolve_api_key_ref(str(payload.get("api_key_ref") or ""), provider=provider),
            model_name=str(payload.get("model_name") or self._default_model_name(provider, str(payload.get("purpose") or ""))),
            temperature=float(payload["temperature"]) if payload.get("temperature") is not None else self._default_temperature(str(payload.get("purpose") or "")),
            max_tokens=int(payload.get("max_tokens") or 4096),
        )

    def _ensure_default_profiles(self) -> None:
        now = datetime.now(timezone.utc).isoformat()
        defaults = [
            ModelProfile(
                id=self.settings.default_chat_profile_id,
                user_id="",
                purpose="chat_fast",
                provider=self._default_provider(self.settings.openai_api_base, self.settings.openai_api_key),
                name="Default Chat Profile",
                base_url=self._default_base_url(self._default_provider(self.settings.openai_api_base, self.settings.openai_api_key)),
                api_key_ref="env:OPENAI_API_KEY" if self.settings.openai_api_key else "",
                model_name=self._default_model_name(self._default_provider(self.settings.openai_api_base, self.settings.openai_api_key), "chat_fast"),
                temperature=self.settings.default_chat_temperature,
                max_tokens=4096,
                is_default=True,
                updated_at=now,
            ),
            ModelProfile(
                id=self.settings.default_chat_quality_profile_id,
                user_id="",
                purpose="chat_quality",
                provider=self._default_provider(self.settings.openai_api_base, self.settings.openai_api_key),
                name="Default Chat Quality Profile",
                base_url=self._default_base_url(self._default_provider(self.settings.openai_api_base, self.settings.openai_api_key)),
                api_key_ref="env:OPENAI_API_KEY" if self.settings.openai_api_key else "",
                model_name=self._default_model_name(self._default_provider(self.settings.openai_api_base, self.settings.openai_api_key), "chat_quality"),
                temperature=self.settings.default_chat_quality_temperature,
                max_tokens=4096,
                is_default=True,
                updated_at=now,
            ),
            ModelProfile(
                id=self.settings.default_report_profile_id,
                user_id="",
                purpose="report_writer",
                provider=self._default_provider(self.settings.openai_api_base, self.settings.openai_api_key),
                name="Default Report Profile",
                base_url=self._default_base_url(self._default_provider(self.settings.openai_api_base, self.settings.openai_api_key)),
                api_key_ref="env:OPENAI_API_KEY" if self.settings.openai_api_key else "",
                model_name=self._default_model_name(self._default_provider(self.settings.openai_api_base, self.settings.openai_api_key), "report_writer"),
                temperature=self.settings.default_report_temperature,
                max_tokens=4096,
                is_default=True,
                updated_at=now,
            ),
        ]
        for profile in defaults:
            self.profile_repo.upsert(profile)

    def _resolve_api_key_ref(self, value: str, *, provider: str) -> str:
        ref = (value or "").strip()
        if ref.startswith("env:"):
            import os

            return os.getenv(ref[4:], "").strip()
        return ref or self._default_api_key(provider)

    def _default_provider(self, api_base: str, api_key: str) -> str:
        if api_base and api_key:
            return "openai_compatible"
        if self.settings.generator_backend == "ollama":
            return "ollama"
        return "fallback"

    def _default_model_name(self, provider: str, purpose: str) -> str:
        if provider == "ollama":
            return self.settings.ollama_model
        if provider == "openai_compatible":
            normalized_purpose = (purpose or "").strip().lower()
            if normalized_purpose == "chat_quality":
                return self.settings.default_chat_quality_model
            if normalized_purpose == "report_writer":
                return self.settings.default_report_model
            return self.settings.default_chat_model
        return "fallback"

    def _default_base_url(self, provider: str) -> str:
        if provider == "ollama":
            return self.settings.ollama_base_url
        if provider == "openai_compatible":
            return self.settings.openai_api_base
        return ""

    def _default_api_key(self, provider: str) -> str:
        if provider == "openai_compatible":
            return self.settings.openai_api_key
        return ""

    def _default_temperature(self, purpose: str) -> float:
        normalized_purpose = (purpose or "").strip().lower()
        if normalized_purpose == "chat_quality":
            return self.settings.default_chat_quality_temperature
        if normalized_purpose == "report_writer":
            return self.settings.default_report_temperature
        return self.settings.default_chat_temperature
