from __future__ import annotations

import json
from typing import Iterator, Optional, Union

import requests

from qqa_llm.core.errors import GenerationError
from qqa_llm.domain.models import ChatPrompt, ReportPrompt
from qqa_llm.inference.types import GenerationStreamChunk


class OllamaGenerationClient:
    def __init__(self, *, model: str, base_url: str, temperature: Optional[float] = None, max_tokens: int = 4096) -> None:
        self.model = model
        self.base_url = self._normalize_base_url(base_url)
        self.temperature = temperature
        self.max_tokens = max_tokens

    def generate_stream(self, prompt: Union[ChatPrompt, ReportPrompt]) -> Iterator[GenerationStreamChunk]:
        options = {"num_predict": self.max_tokens}
        if self.temperature is not None:
            options["temperature"] = self.temperature
        response = requests.post(
            f"{self.base_url}/api/chat",
            json={
                "model": self.model,
                "stream": True,
                "options": options,
                "messages": [
                    {"role": "system", "content": prompt.system_prompt},
                    {"role": "user", "content": prompt.user_prompt},
                ],
            },
            timeout=180,
            stream=True,
        )
        response.raise_for_status()
        for line in response.iter_lines():
            if not line:
                continue
            payload = json.loads(line)
            message = (payload.get("message") or {}).get("content") or ""
            if message:
                yield GenerationStreamChunk(content=message)

    def generate(self, prompt: Union[ChatPrompt, ReportPrompt]) -> str:
        options = {"num_predict": self.max_tokens}
        if self.temperature is not None:
            options["temperature"] = self.temperature
        response = requests.post(
            f"{self.base_url}/api/chat",
            json={
                "model": self.model,
                "stream": False,
                "options": options,
                "messages": [
                    {"role": "system", "content": prompt.system_prompt},
                    {"role": "user", "content": prompt.user_prompt},
                ],
            },
            timeout=180,
        )
        response.raise_for_status()
        payload = response.json()
        content = (payload.get("message") or {}).get("content") or ""
        if not content.strip():
            raise GenerationError("ollama generation returned empty content")
        return content.strip()

    @staticmethod
    def _normalize_base_url(base_url: str) -> str:
        normalized = base_url.rstrip("/")
        # The app stores local chat models with an OpenAI-compatible URL such
        # as http://127.0.0.1:11434/v1. Ollama's native API hangs off the
        # server root, so keeping /v1 would incorrectly produce /v1/api/chat.
        if normalized.endswith("/v1"):
            return normalized[:-3]
        return normalized
