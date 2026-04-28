from __future__ import annotations

import tempfile
import unittest
from pathlib import Path

from fastapi.testclient import TestClient

from qqa_llm.core.config import Settings
from qqa_llm.eval.fixtures import (
    QUALITY_EVIDENCE_CASES,
    QUALITY_REPORT_CASES,
    QUALITY_RETRIEVAL_CASES,
    QUALITY_SCOPE_FIXTURES,
)
from qqa_llm.eval.quality_runner import QualityEvalRunner
from qqa_llm.main import create_app


class QualityRegressionTest(unittest.TestCase):
    def setUp(self) -> None:
        self.temp_dir = tempfile.TemporaryDirectory()
        settings = Settings(
            data_dir=Path(self.temp_dir.name),
            internal_auth_token="test-token",
            embedding_dim=128,
            embedding_backend="auto",
            embedding_model="text-embedding-3-small",
            bce_embedding_model="maidalun1020/bce-embedding-base_v1",
            lexical_top_k=10,
            vector_top_k=10,
            fusion_top_k=10,
            rerank_top_k=5,
            rerank_backend="auto",
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
        self.client = TestClient(create_app(settings))
        self.runner = QualityEvalRunner(self.client, token="test-token")

    def tearDown(self) -> None:
        self.temp_dir.cleanup()

    def test_quality_regression_suite(self) -> None:
        summary = self.runner.run_suite(
            user_id="u_quality",
            scopes=QUALITY_SCOPE_FIXTURES,
            retrieval_cases=QUALITY_RETRIEVAL_CASES,
            evidence_cases=QUALITY_EVIDENCE_CASES,
            report_cases=QUALITY_REPORT_CASES,
        )
        self.assertEqual(
            summary.failed_count,
            0,
            f"quality regression suite should pass, failures were:\n{summary.failure_report()}",
        )
        self.assertEqual(
            summary.passed_count,
            len(QUALITY_RETRIEVAL_CASES) + len(QUALITY_EVIDENCE_CASES) + len(QUALITY_REPORT_CASES),
        )
        self.assertGreaterEqual(summary.average_score, 0.95)
        benchmark_report = summary.markdown_report()
        self.assertIn("# LLM Quality Benchmark", benchmark_report)
        self.assertIn("evidence_merge average", benchmark_report)
        self.assertIn("retrieve average", benchmark_report)
        self.assertIn("report average", benchmark_report)
        first_case = summary.results[0].to_dict()
        self.assertIn("score", first_case)
        self.assertIn("metrics", first_case)
