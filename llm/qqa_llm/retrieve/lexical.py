from __future__ import annotations

from typing import Dict, Iterable, List

from qqa_llm.retrieve.query_normalizer import tokenize


class LexicalRetriever:
    def score(self, query: str, chunks: Iterable[Dict[str, object]]) -> List[Dict[str, object]]:
        query_tokens = tokenize(query)
        scored: List[Dict[str, object]] = []
        for chunk in chunks:
            title_tokens = tokenize(str(chunk.get("title") or ""))
            repo_tokens = tokenize(str(chunk.get("repo") or ""))
            text_tokens = tokenize(str(chunk.get("lexical_text") or ""))
            score = 0.0
            for token in query_tokens:
                if token in title_tokens:
                    score += 3.0
                if token in repo_tokens:
                    score += 1.5
                if token in text_tokens:
                    score += 1.0
            if score > 0:
                scored.append({"chunk_id": chunk.get("id"), "score": score})
        scored.sort(key=lambda item: float(item["score"]), reverse=True)
        return scored
