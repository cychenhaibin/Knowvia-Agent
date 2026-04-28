from __future__ import annotations

import re
from typing import List


TOKEN_RE = re.compile(r"[A-Za-z0-9_]+|[\u4e00-\u9fff]")


def tokenize(text: str) -> List[str]:
    return [token.lower() for token in TOKEN_RE.findall(text or "")]


class QueryNormalizer:
    def normalize(self, query: str) -> List[str]:
        return tokenize(query)
