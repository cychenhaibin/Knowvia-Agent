from __future__ import annotations

import hashlib
from typing import Iterable, List, Protocol

import numpy as np
from openai import OpenAI

from qqa_llm.core.config import Settings
from qqa_llm.retrieve.query_normalizer import tokenize

try:
    from BCEmbedding import EmbeddingModel
except ImportError:  # pragma: no cover - depends on optional local dependency
    EmbeddingModel = None


class EmbeddingClient(Protocol):
    def embed_texts(self, texts: Iterable[str]) -> np.ndarray:
        ...

    def backend_name(self) -> str:
        ...


class HashEmbeddingClient:
    # The hash embedding keeps the retrieval pipeline deterministic in local
    # tests and offline environments. It remains as the fallback path, but the
    # service can now switch to a real OpenAI-compatible embedding backend when
    # credentials are present.
    def __init__(self, dim: int) -> None:
        self.dim = dim

    def embed_texts(self, texts: Iterable[str]) -> np.ndarray:
        vectors: List[np.ndarray] = []
        for text in texts:
            vector = np.zeros(self.dim, dtype=np.float32)
            for token in tokenize(text):
                digest = hashlib.blake2b(token.encode("utf-8"), digest_size=16).digest()
                index = int.from_bytes(digest[:8], "big") % self.dim
                sign = 1.0 if digest[8] % 2 == 0 else -1.0
                vector[index] += sign
            norm = np.linalg.norm(vector)
            if norm > 0:
                vector /= norm
            vectors.append(vector)
        if not vectors:
            return np.zeros((0, self.dim), dtype=np.float32)
        return np.vstack(vectors)

    def backend_name(self) -> str:
        return "hash"


class OpenAIEmbeddingClient:
    def __init__(self, *, model: str, api_base: str, api_key: str) -> None:
        self.model = model
        self.client = OpenAI(api_key=api_key, base_url=api_base)

    def embed_texts(self, texts: Iterable[str]) -> np.ndarray:
        items = [str(text) for text in texts]
        if not items:
            return np.zeros((0, 0), dtype=np.float32)
        response = self.client.embeddings.create(model=self.model, input=items)
        vectors = [np.asarray(item.embedding, dtype=np.float32) for item in response.data]
        return np.vstack(vectors)

    def backend_name(self) -> str:
        return "openai_compatible"


class BCEmbeddingClient:
    """Optional local embedding backend inspired by yuque-rag.

    We keep model loading lazy so the llm service can start quickly even if the
    caller never selects the BCE path during this process lifetime.
    """

    def __init__(self, *, model_name: str) -> None:
        self.model_name = model_name
        self._model = None

    def embed_texts(self, texts: Iterable[str]) -> np.ndarray:
        items = [str(text) for text in texts]
        if not items:
            return np.zeros((0, 0), dtype=np.float32)
        model = self._ensure_model()
        vectors = model.encode(items)
        return np.asarray(vectors, dtype=np.float32)

    def backend_name(self) -> str:
        return "bce"

    def _ensure_model(self):
        if EmbeddingModel is None:
            raise RuntimeError("BCEmbedding is not installed")
        if self._model is None:
            self._model = EmbeddingModel(model_name_or_path=self.model_name)
        return self._model


def build_embedding_client(settings: Settings) -> EmbeddingClient:
    backend = settings.embedding_backend
    if backend == "bce":
        if EmbeddingModel is not None:
            return BCEmbeddingClient(model_name=settings.bce_embedding_model)
        return HashEmbeddingClient(settings.embedding_dim)
    if backend in {"auto", "openai"} and settings.openai_api_base and settings.openai_api_key:
        return OpenAIEmbeddingClient(
            model=settings.embedding_model,
            api_base=settings.openai_api_base,
            api_key=settings.openai_api_key,
        )
    return HashEmbeddingClient(settings.embedding_dim)


def describe_embedding_backend(settings: Settings) -> dict[str, str]:
    if settings.embedding_backend == "bce":
        if EmbeddingModel is None:
            return {
                "status": "degraded",
                "backend": "bce",
                "message": "BCEmbedding is not installed for BCE embeddings.",
            }
        return {
            "status": "ok",
            "backend": "bce",
            "message": f"BCE embedding backend is configured for {settings.bce_embedding_model}.",
        }
    if settings.embedding_backend in {"auto", "openai"}:
        if settings.openai_api_base and settings.openai_api_key:
            return {
                "status": "ok",
                "backend": "openai_compatible",
                "message": f"Embedding backend is configured for {settings.embedding_model}.",
            }
        if settings.embedding_backend == "openai":
            return {
                "status": "degraded",
                "backend": "openai_compatible",
                "message": "OPENAI_API_BASE or OPENAI_API_KEY is missing for embeddings.",
            }
    return {
        "status": "ok",
        "backend": "hash",
        "message": "Hash embedding fallback is active.",
    }
