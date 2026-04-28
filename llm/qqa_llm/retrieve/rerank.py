from __future__ import annotations

from typing import Dict, List

from qqa_llm.inference.reranker import RerankClient


class RerankStage:
    def __init__(self, rerank_client: RerankClient) -> None:
        self.rerank_client = rerank_client

    def rerank(self, query: str, candidates: List[Dict[str, object]]) -> List[Dict[str, object]]:
        pairs = [(str(candidate.get("chunk_id") or ""), str(candidate.get("text") or "")) for candidate in candidates]
        scored = self.rerank_client.rerank(query, pairs)
        score_map = {chunk_id: score for chunk_id, score in scored}
        reranked = []
        for candidate in candidates:
            chunk_id = str(candidate.get("chunk_id") or "")
            reranked.append({**candidate, "rerank_score": score_map.get(chunk_id, 0.0)})
        reranked.sort(key=lambda item: float(item["rerank_score"]), reverse=True)
        return reranked
