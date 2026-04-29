from __future__ import annotations

from dataclasses import dataclass
from typing import Optional


@dataclass
class GenerationUsage:
    prompt_tokens: int = 0
    completion_tokens: int = 0
    total_tokens: int = 0


@dataclass
class GenerationStreamChunk:
    content: str = ""
    usage: Optional[GenerationUsage] = None
