from __future__ import annotations

import os
import tempfile
import unittest
import uuid
from pathlib import Path

from fastapi.testclient import TestClient

from qqa_llm.core.config import Settings
from qqa_llm.main import create_app


def _env_flag(name: str) -> bool:
    return os.getenv(name, "").strip().lower() in {"1", "true", "yes", "on"}


class RealBackendBenchmarkTest(unittest.TestCase):
    """Optional benchmark that exercises non-fallback backends end to end.

    This suite is intentionally opt-in because it may require local BCE model
    caches or outbound access to an OpenAI-compatible endpoint. It exists to
    turn "manual real-backend verification" into a repeatable quality gate.
    """

    def setUp(self) -> None:
        if not _env_flag("QQA_RUN_REAL_BACKEND_BENCHMARK"):
            self.skipTest("QQA_RUN_REAL_BACKEND_BENCHMARK is not enabled")
        self.mode = (os.getenv("QQA_REAL_BACKEND_MODE", "bce").strip().lower() or "bce")
        if self.mode not in {"bce", "openai"}:
            self.skipTest(f"unsupported QQA_REAL_BACKEND_MODE: {self.mode}")
        self.postgres_dsn = os.getenv("QQA_POSTGRES_TEST_DSN", "").strip()
        self.temp_dir = tempfile.TemporaryDirectory()

    def tearDown(self) -> None:
        if hasattr(self, "temp_dir"):
            self.temp_dir.cleanup()

    def test_quality_benchmark_with_real_backend(self) -> None:
        schema_name = f"qqaat_realbench_{uuid.uuid4().hex[:12]}"
        settings = self._build_settings(schema_name=schema_name)
        client = TestClient(create_app(settings))
        headers = {"Authorization": "Bearer test-token"}

        suites = client.get("/internal/v1/eval/quality/suites", headers=headers)
        self.assertEqual(suites.status_code, 200)
        self.assertEqual(suites.json()[0]["suite_name"], "builtin")

        run = client.post(
            "/internal/v1/eval/quality/run",
            headers=headers,
            json={
                "user_id": "u_real_backend",
                "suite_name": "builtin",
                "cleanup": True,
                "scope_prefix": f"real_{self.mode}",
            },
        )
        self.assertEqual(run.status_code, 200)
        payload = run.json()
        self.assertTrue(payload["trace_id"])
        self.assertEqual(payload["failed_count"], 0, payload["markdown_report"])
        self.assertGreaterEqual(payload["average_score"], 0.9)
        self.assertTrue(payload["cleanup_applied"])
        self.assertEqual(payload["backend_snapshot"]["checks"]["qualityBenchmarks"], "ok")

        if self.postgres_dsn:
            self.assertEqual(payload["backend_snapshot"]["index_backend"], "postgres_pgvector")
            self.assertEqual(payload["backend_snapshot"]["checks"]["database"], "ok")
        else:
            self.assertEqual(payload["backend_snapshot"]["index_backend"], "file")

        embedding_backend = payload["backend_snapshot"]["embedding"]["backend"]
        rerank_backend = payload["backend_snapshot"]["rerank"]["backend"]
        generator_backend = payload["backend_snapshot"]["generator"]["backend"]

        if self.mode == "bce":
            self.assertEqual(embedding_backend, "bce")
            self.assertEqual(rerank_backend, "bce")
            # Report generation may still intentionally use fallback if the
            # benchmark only targets retrieval quality with local BCE models.
            self.assertIn(generator_backend, {"fallback", "openai", "ollama"})
        else:
            self.assertEqual(embedding_backend, "openai_compatible")
            self.assertIn(rerank_backend, {"embedding", "bce"})
            self.assertEqual(generator_backend, "openai")

    def _build_settings(self, *, schema_name: str) -> Settings:
        if self.mode == "openai":
            api_base = os.getenv("OPENAI_API_BASE", "").strip()
            api_key = os.getenv("OPENAI_API_KEY", "").strip()
            if not api_base or not api_key:
                self.skipTest("OPENAI_API_BASE and OPENAI_API_KEY are required for openai real benchmark mode")
            embedding_backend = "openai"
            rerank_backend = "embedding"
            generator_backend = "openai"
        else:
            embedding_backend = "bce"
            rerank_backend = "bce"
            generator_backend = "fallback"
            api_base = os.getenv("OPENAI_API_BASE", "").strip()
            api_key = os.getenv("OPENAI_API_KEY", "").strip()

        return Settings(
            data_dir=Path(self.temp_dir.name),
            internal_auth_token="test-token",
            embedding_dim=768 if self.mode == "bce" else 384,
            embedding_backend=embedding_backend,
            embedding_model=os.getenv("QQA_REAL_BENCHMARK_EMBEDDING_MODEL", "text-embedding-3-small"),
            bce_embedding_model=os.getenv("QQA_BCE_EMBEDDING_MODEL", "maidalun1020/bce-embedding-base_v1"),
            lexical_top_k=10,
            vector_top_k=10,
            fusion_top_k=10,
            rerank_top_k=5,
            rerank_backend=rerank_backend,
            rerank_model=os.getenv("QQA_RERANK_MODEL", ""),
            bce_rerank_model=os.getenv("QQA_BCE_RERANK_MODEL", "maidalun1020/bce-reranker-base_v1"),
            max_sources=5,
            chunk_size=220,
            chunk_overlap=40,
            max_snippet_chars=120,
            generator_backend=generator_backend,
            default_chat_model=os.getenv("OPENAI_MODEL", ""),
            default_chat_quality_model=os.getenv("OPENAI_CHAT_QUALITY_MODEL", os.getenv("OPENAI_MODEL", "")),
            default_report_model=os.getenv("OPENAI_REPORT_MODEL", os.getenv("OPENAI_MODEL", "")),
            openai_api_base=api_base,
            openai_api_key=api_key,
            ollama_base_url=os.getenv("OLLAMA_BASE_URL", "http://127.0.0.1:11434"),
            ollama_model=os.getenv("OLLAMA_MODEL", "qwen2.5:7b"),
            index_backend="postgres" if self.postgres_dsn else "file",
            postgres_dsn=self.postgres_dsn,
            postgres_schema=schema_name,
            postgres_connect_timeout=5,
        )


if __name__ == "__main__":
    unittest.main()
