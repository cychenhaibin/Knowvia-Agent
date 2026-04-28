from __future__ import annotations

from typing import Iterable, List, Protocol, Sequence, Tuple

import numpy as np

from qqa_llm.core.config import Settings
from qqa_llm.inference.embeddings import EmbeddingClient
from qqa_llm.retrieve.query_normalizer import tokenize

try:
    from BCEmbedding import RerankerModel
except ImportError:  # pragma: no cover - depends on optional local dependency
    RerankerModel = None


class RerankClient(Protocol):
    def rerank(self, query: str, candidates: Sequence[Tuple[str, str]]) -> List[Tuple[str, float]]:
        ...

    def backend_name(self) -> str:
        ...


class SimpleRerankClient:
    def rerank(self, query: str, candidates: Sequence[Tuple[str, str]]) -> List[Tuple[str, float]]:
        query_tokens = tokenize(query)
        scored: List[Tuple[str, float]] = []
        for candidate_id, text in candidates:
            score = self._score(query_tokens, text)
            scored.append((candidate_id, score))
        scored.sort(key=lambda item: item[1], reverse=True)
        return scored

    def backend_name(self) -> str:
        return "simple"

    def _score(self, query_tokens: Iterable[str], text: str) -> float:
        text_tokens = tokenize(text)
        if not text_tokens:
            return 0.0
        overlap = sum(1 for token in query_tokens if token in text_tokens)
        density = overlap / max(len(text_tokens), 1)
        return overlap + density


class EmbeddingRerankClient:
    """Second-stage rerank using real embeddings when available.

    This keeps the two-stage retrieval shape used by `yuque-rag` while staying
    compatible with the hybrid lexical/vector candidate recall we borrowed from
    `YuqueSyncPlatform`.
    """

    def __init__(self, embedding_client: EmbeddingClient) -> None:
        self.embedding_client = embedding_client

    def rerank(self, query: str, candidates: Sequence[Tuple[str, str]]) -> List[Tuple[str, float]]:
        if not candidates:
            return []
        texts = [query, *[text for _, text in candidates]]
        embeddings = self.embedding_client.embed_texts(texts)
        if embeddings.shape[0] != len(candidates) + 1:
            return SimpleRerankClient().rerank(query, candidates)
        query_vector = embeddings[0]
        query_norm = float(np.linalg.norm(query_vector)) or 1.0
        scored: List[Tuple[str, float]] = []
        for index, (candidate_id, _) in enumerate(candidates, start=1):
            candidate_vector = embeddings[index]
            candidate_norm = float(np.linalg.norm(candidate_vector)) or 1.0
            score = float(candidate_vector @ query_vector) / (candidate_norm * query_norm)
            scored.append((candidate_id, score))
        scored.sort(key=lambda item: item[1], reverse=True)
        return scored

    def backend_name(self) -> str:
        return f"embedding:{self.embedding_client.backend_name()}"


class BCEmbeddingRerankClient:
    """Optional local BCE rerank backend inspired by yuque-rag."""

    def __init__(self, *, model_name: str) -> None:
        self.model_name = model_name
        self._model = None

    def rerank(self, query: str, candidates: Sequence[Tuple[str, str]]) -> List[Tuple[str, float]]:
        if not candidates:
            return []
        pairs = [(query, text) for _, text in candidates]
        model = self._ensure_model()
        if hasattr(model, "compute_score"):
            scores = model.compute_score(pairs)
            # BCE returns a bare float for a single pair but an iterable for
            # multi-pair batches. Normalize both shapes so the rest of the
            # retrieval pipeline can treat rerank output uniformly.
            if isinstance(scores, (float, int)):
                scores = [float(scores)]
            elif not isinstance(scores, list):
                scores = list(scores)
            ranked = [(candidate_id, float(score)) for (candidate_id, _), score in zip(candidates, scores)]
            ranked.sort(key=lambda item: item[1], reverse=True)
            return ranked

        result = model.rerank(query, [text for _, text in candidates])
        if isinstance(result, dict) and "rerank_passages" in result and "rerank_scores" in result:
            passages = [str(text) for text in result.get("rerank_passages") or []]
            scores = [float(score) for score in result.get("rerank_scores") or []]
            positions: dict[str, List[int]] = {}
            for index, (_, text) in enumerate(candidates):
                positions.setdefault(text, []).append(index)
            ranked: List[Tuple[str, float]] = []
            for text, score in zip(passages, scores):
                bucket = positions.get(text) or []
                if not bucket:
                    continue
                candidate_index = bucket.pop(0)
                ranked.append((candidates[candidate_index][0], score))
            if ranked:
                return ranked

        return SimpleRerankClient().rerank(query, candidates)

    def backend_name(self) -> str:
        return "bce"

    def _ensure_model(self):
        if RerankerModel is None:
            raise RuntimeError("BCEmbedding is not installed")
        if self._model is None:
            self._model = RerankerModel(model_name_or_path=self.model_name)
        return self._model


def build_rerank_client(settings: Settings, embedding_client: EmbeddingClient) -> RerankClient:
    if settings.rerank_backend == "bce":
        if RerankerModel is not None:
            return BCEmbeddingRerankClient(model_name=settings.bce_rerank_model)
        return SimpleRerankClient()
    if settings.rerank_backend in {"auto", "embedding"} and embedding_client.backend_name() != "hash":
        return EmbeddingRerankClient(embedding_client)
    return SimpleRerankClient()


def describe_rerank_backend(settings: Settings, embedding_client: EmbeddingClient) -> dict[str, str]:
    if settings.rerank_backend == "bce":
        if RerankerModel is None:
            return {
                "status": "degraded",
                "backend": "bce",
                "message": "BCEmbedding is not installed for BCE rerank.",
            }
        return {
            "status": "ok",
            "backend": "bce",
            "message": f"BCE rerank backend is configured for {settings.bce_rerank_model}.",
        }
    if settings.rerank_backend in {"auto", "embedding"} and embedding_client.backend_name() != "hash":
        return {
            "status": "ok",
            "backend": "embedding",
            "message": f"Embedding-backed rerank is configured via {embedding_client.backend_name()}.",
        }
    if settings.rerank_backend == "embedding":
        return {
            "status": "degraded",
            "backend": "embedding",
            "message": "Embedding backend is unavailable, so rerank fell back to simple lexical scoring.",
        }
    return {
        "status": "ok",
        "backend": "simple",
        "message": "Simple rerank fallback is active.",
    }
