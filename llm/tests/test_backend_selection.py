from __future__ import annotations

import json
import tempfile
import unittest
from datetime import datetime, timedelta, timezone
from pathlib import Path
from unittest.mock import patch

from qqa_llm.core.config import Settings
from qqa_llm.inference.ollama_client import OllamaGenerationClient
from qqa_llm.main import build_container, create_app
from qqa_llm.inference.embeddings import EmbeddingModel, build_embedding_client, describe_embedding_backend
from qqa_llm.inference.reranker import RerankerModel, build_rerank_client, describe_rerank_backend
from qqa_llm.services.model_profile_service import ModelProfileService
from qqa_llm.storage.file_repo import FileStore
from qqa_llm.storage.model_profile_repo import ModelProfileRepository


def make_settings(**overrides) -> Settings:
    defaults = dict(
        data_dir=Path(tempfile.mkdtemp()),
        internal_auth_token="test-token",
        embedding_dim=768,
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
        index_backend="auto",
        postgres_dsn="",
        postgres_schema="qqaat_test",
        postgres_connect_timeout=5,
        default_chat_profile_id="profile_chat_fast_default",
        default_report_profile_id="profile_report_default",
    )
    defaults.update(overrides)
    return Settings(**defaults)


class BackendSelectionTest(unittest.TestCase):
    def test_auto_index_backend_uses_file_when_postgres_is_missing(self) -> None:
        settings = make_settings(index_backend="auto", postgres_dsn="")
        container = build_container(settings)

        self.assertEqual(container.index_backend.backend_name(), "file")

    def test_trace_retention_cleanup_runs_on_startup_for_file_backend(self) -> None:
        temp_dir = Path(tempfile.mkdtemp())
        traces_path = temp_dir / "traces.jsonl"
        now = datetime.now(timezone.utc)
        payloads = [
            {
                "trace_id": "trace_old_chat",
                "request_type": "chat",
                "created_at": (now - timedelta(days=45)).isoformat(),
            },
            {
                "trace_id": "trace_old_report",
                "request_type": "report",
                "created_at": (now - timedelta(days=45)).isoformat(),
            },
            {
                "trace_id": "trace_old_benchmark",
                "request_type": "quality_benchmark",
                "created_at": (now - timedelta(days=45)).isoformat(),
            },
            {
                "trace_id": "trace_very_old_report",
                "request_type": "report",
                "created_at": (now - timedelta(days=120)).isoformat(),
            },
        ]
        traces_path.write_text("".join(json.dumps(item, ensure_ascii=False) + "\n" for item in payloads), encoding="utf-8")

        settings = make_settings(
            data_dir=temp_dir,
            index_backend="file",
            trace_retention_days=30,
            report_trace_retention_days=90,
        )
        app = create_app(settings)
        cleanup = app.state.trace_cleanup
        remaining = [json.loads(line) for line in traces_path.read_text(encoding="utf-8").splitlines() if line.strip()]
        remaining_ids = {item["trace_id"] for item in remaining}

        self.assertGreaterEqual(cleanup["removed"], 2)
        self.assertNotIn("trace_old_chat", remaining_ids)
        self.assertIn("trace_old_report", remaining_ids)
        self.assertIn("trace_old_benchmark", remaining_ids)
        self.assertNotIn("trace_very_old_report", remaining_ids)

    def test_bce_backends_can_be_selected(self) -> None:
        if EmbeddingModel is None or RerankerModel is None:
            self.skipTest("BCEmbedding is an optional dependency")
        settings = make_settings(embedding_backend="bce", rerank_backend="bce")
        embedding_client = build_embedding_client(settings)
        rerank_client = build_rerank_client(settings, embedding_client)

        self.assertEqual(embedding_client.backend_name(), "bce")
        self.assertEqual(rerank_client.backend_name(), "bce")
        self.assertEqual(describe_embedding_backend(settings)["backend"], "bce")
        self.assertEqual(describe_rerank_backend(settings, embedding_client)["backend"], "bce")

    def test_bce_backends_degrade_safely_when_dependency_is_missing(self) -> None:
        settings = make_settings(embedding_backend="bce", rerank_backend="bce", embedding_dim=384)
        with patch("qqa_llm.inference.embeddings.EmbeddingModel", None), patch(
            "qqa_llm.inference.reranker.RerankerModel", None
        ):
            embedding_client = build_embedding_client(settings)
            rerank_client = build_rerank_client(settings, embedding_client)

            self.assertEqual(embedding_client.backend_name(), "hash")
            self.assertEqual(rerank_client.backend_name(), "simple")
            self.assertEqual(describe_embedding_backend(settings)["status"], "degraded")
            self.assertEqual(describe_rerank_backend(settings, embedding_client)["status"], "degraded")

    def test_default_openai_models_can_differ_by_purpose(self) -> None:
        settings = make_settings(
            generator_backend="openai_compatible",
            openai_api_base="https://example.invalid/v1",
            openai_api_key="sk-test",
            default_chat_model="gpt-fast",
            default_chat_quality_model="gpt-quality",
            default_report_model="gpt-report",
        )
        profile_service = ModelProfileService(
            settings,
            ModelProfileRepository(FileStore(settings.data_dir), settings.data_dir / "model_profiles.json"),
        )

        chat_fast = profile_service.resolve(user_id="", purpose="chat_fast")
        chat_quality = profile_service.resolve(user_id="", purpose="chat_quality")
        report_writer = profile_service.resolve(user_id="", purpose="report_writer")

        self.assertEqual(chat_fast.model_name, "gpt-fast")
        self.assertEqual(chat_quality.model_name, "gpt-quality")
        self.assertEqual(report_writer.model_name, "gpt-report")

    def test_ollama_client_accepts_openai_style_base_url(self) -> None:
        client = OllamaGenerationClient(
            model="gemma3n:e4b",
            base_url="http://127.0.0.1:11434/v1",
        )

        self.assertEqual(client.base_url, "http://127.0.0.1:11434")


if __name__ == "__main__":
    unittest.main()
