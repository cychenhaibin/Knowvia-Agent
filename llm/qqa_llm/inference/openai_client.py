from __future__ import annotations

from typing import Iterator, Optional, Union

from openai import OpenAI

from qqa_llm.core.errors import GenerationError
from qqa_llm.domain.models import ChatPrompt, ReportPrompt
from qqa_llm.inference.types import GenerationStreamChunk, GenerationUsage


class OpenAIGenerationClient:
    def __init__(
        self,
        *,
        model: str,
        api_base: str,
        api_key: str,
        temperature: Optional[float] = None,
        max_tokens: int = 4096,
        enable_search: bool = False,
    ) -> None:
        self.model = model
        self.client = OpenAI(api_key=api_key, base_url=api_base)
        self.temperature = temperature
        self.max_tokens = max_tokens
        self.enable_search = enable_search

    def generate_stream(self, prompt: Union[ChatPrompt, ReportPrompt]) -> Iterator[GenerationStreamChunk]:
        request = {
            "model": self.model,
            "messages": [
                {"role": "system", "content": prompt.system_prompt},
                {"role": "user", "content": prompt.user_prompt},
            ],
            "max_tokens": self.max_tokens,
            "stream": True,
            "stream_options": {"include_usage": True},
        }
        if self.temperature is not None:
            request["temperature"] = self.temperature
        if self.enable_search:
            request["extra_body"] = {"enable_search": True}
        stream = self.client.chat.completions.create(**request)
        for chunk in stream:
            usage = getattr(chunk, "usage", None)
            if usage is not None:
                yield GenerationStreamChunk(
                    usage=GenerationUsage(
                        prompt_tokens=int(getattr(usage, "prompt_tokens", 0) or 0),
                        completion_tokens=int(getattr(usage, "completion_tokens", 0) or 0),
                        total_tokens=int(getattr(usage, "total_tokens", 0) or 0),
                    )
                )
                continue
            if not chunk.choices:
                continue
            delta = chunk.choices[0].delta.content
            if delta:
                yield GenerationStreamChunk(content=delta)

    def generate(self, prompt: Union[ChatPrompt, ReportPrompt]) -> str:
        request = {
            "model": self.model,
            "messages": [
                {"role": "system", "content": prompt.system_prompt},
                {"role": "user", "content": prompt.user_prompt},
            ],
            "max_tokens": self.max_tokens,
        }
        if self.temperature is not None:
            request["temperature"] = self.temperature
        response = self.client.chat.completions.create(**request)
        content = response.choices[0].message.content
        if not content:
            raise GenerationError("openai generation returned empty content")
        return content.strip()
