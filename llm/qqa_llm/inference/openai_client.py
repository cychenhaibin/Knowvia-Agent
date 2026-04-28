from __future__ import annotations

from typing import Iterator, Union

from openai import OpenAI

from qqa_llm.core.errors import GenerationError
from qqa_llm.domain.models import ChatPrompt, ReportPrompt


class OpenAIGenerationClient:
    def __init__(self, *, model: str, api_base: str, api_key: str, temperature: float = 0.2, max_tokens: int = 4096) -> None:
        self.model = model
        self.client = OpenAI(api_key=api_key, base_url=api_base)
        self.temperature = temperature
        self.max_tokens = max_tokens

    def generate_stream(self, prompt: Union[ChatPrompt, ReportPrompt]) -> Iterator[str]:
        stream = self.client.chat.completions.create(
            model=self.model,
            messages=[
                {"role": "system", "content": prompt.system_prompt},
                {"role": "user", "content": prompt.user_prompt},
            ],
            temperature=self.temperature,
            max_tokens=self.max_tokens,
            stream=True,
        )
        for chunk in stream:
            delta = chunk.choices[0].delta.content
            if delta:
                yield delta

    def generate(self, prompt: Union[ChatPrompt, ReportPrompt]) -> str:
        response = self.client.chat.completions.create(
            model=self.model,
            messages=[
                {"role": "system", "content": prompt.system_prompt},
                {"role": "user", "content": prompt.user_prompt},
            ],
            temperature=self.temperature,
            max_tokens=self.max_tokens,
        )
        content = response.choices[0].message.content
        if not content:
            raise GenerationError("openai generation returned empty content")
        return content.strip()
