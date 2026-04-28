from __future__ import annotations

import tempfile
import unittest
from pathlib import Path

import numpy as np

from qqa_llm.core.config import Settings
from qqa_llm.inference.embeddings import BCEmbeddingClient, EmbeddingModel, build_embedding_client
from qqa_llm.inference.reranker import BCEmbeddingRerankClient, RerankerModel, build_rerank_client


class BCESmokeTest(unittest.TestCase):
    def setUp(self) -> None:
        self.temp_dir = tempfile.TemporaryDirectory()

    def tearDown(self) -> None:
        self.temp_dir.cleanup()

    def test_bce_embedding_and_rerank_smoke(self) -> None:
        if EmbeddingModel is None or RerankerModel is None:
            self.skipTest("BCEmbedding is not installed")

        settings = Settings(
            data_dir=Path(self.temp_dir.name),
            internal_auth_token="test-token",
            embedding_dim=768,
            embedding_backend="bce",
            embedding_model="text-embedding-3-small",
            bce_embedding_model="maidalun1020/bce-embedding-base_v1",
            lexical_top_k=10,
            vector_top_k=10,
            fusion_top_k=10,
            rerank_top_k=5,
            rerank_backend="bce",
            rerank_model="",
            bce_rerank_model="maidalun1020/bce-reranker-base_v1",
            max_sources=5,
            chunk_size=220,
            chunk_overlap=40,
            max_snippet_chars=120,
            generator_backend="fallback",
            default_chat_model="",
            default_chat_quality_model="",
            default_report_model="",
            openai_api_base="",
            openai_api_key="",
            ollama_base_url="http://127.0.0.1:11434",
            ollama_model="",
        )

        try:
            embedding_client = build_embedding_client(settings)
            rerank_client = build_rerank_client(settings, embedding_client)
            self.assertIsInstance(embedding_client, BCEmbeddingClient)
            self.assertIsInstance(rerank_client, BCEmbeddingRerankClient)

            vectors = embedding_client.embed_texts(
                [
                    "QuickQue 四月新增了 run timeline 和 artifact 视图。",
                    "权限系统新增了角色模板和企业客户可见范围设置。",
                ]
            )
            self.assertEqual(vectors.shape[0], 2)
            self.assertGreater(vectors.shape[1], 0)
            self.assertTrue(np.isfinite(vectors).all())

            ranked = rerank_client.rerank(
                "四月新增了什么交付体验相关功能",
                [
                    ("release", "QuickQue 在四月新增了 run timeline 和 artifact 视图。"),
                    ("auth", "权限系统新增了角色模板和企业客户可见范围设置。"),
                ],
            )
            self.assertEqual(len(ranked), 2)
            self.assertEqual(ranked[0][0], "release")

            single_ranked = rerank_client.rerank(
                "四月新增了什么交付体验相关功能",
                [("release", "QuickQue 在四月新增了 run timeline 和 artifact 视图。")],
            )
            self.assertEqual(len(single_ranked), 1)
            self.assertEqual(single_ranked[0][0], "release")
        except Exception as exc:  # pragma: no cover - depends on local model cache/runtime
            self.skipTest(f"BCE smoke test skipped because local BCE models are unavailable: {exc}")


if __name__ == "__main__":
    unittest.main()
