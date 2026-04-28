from __future__ import annotations

from typing import Dict, Iterable, List

import numpy as np

from qqa_llm.inference.embeddings import EmbeddingClient


class VectorRetriever:
    def __init__(self, embedding_client: EmbeddingClient) -> None:
        self.embedding_client = embedding_client

    def score(self, query: str, chunks: List[Dict[str, object]], embeddings: np.ndarray) -> List[Dict[str, object]]:
        if embeddings.size == 0 or not chunks:
            return []
        query_vector = self.embedding_client.embed_texts([query])
        if query_vector.size == 0:
            return []
        scores = embeddings @ query_vector[0]
        ranked: List[Dict[str, object]] = []
        for index, score in enumerate(scores.tolist()):
            ranked.append({"chunk_id": chunks[index].get("id"), "score": float(score)})
        ranked.sort(key=lambda item: float(item["score"]), reverse=True)
        return ranked
