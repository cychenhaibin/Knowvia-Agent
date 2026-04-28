from __future__ import annotations

import hashlib
import os
import tempfile
import unittest
import uuid
from datetime import datetime, timedelta, timezone
from pathlib import Path

from fastapi.testclient import TestClient
import psycopg
from psycopg.rows import dict_row

from qqa_llm.core.config import Settings
from qqa_llm.eval.fixtures import (
    QUALITY_EVIDENCE_CASES,
    QUALITY_REPORT_CASES,
    QUALITY_RETRIEVAL_CASES,
    QUALITY_SCOPE_FIXTURES,
)
from qqa_llm.eval.quality_runner import QualityEvalRunner
from qqa_llm.main import create_app


class PostgresBackendIntegrationTest(unittest.TestCase):
    def setUp(self) -> None:
        self.postgres_dsn = os.getenv("QQA_POSTGRES_TEST_DSN", "").strip()
        if not self.postgres_dsn:
            self.skipTest("QQA_POSTGRES_TEST_DSN is not configured")

    def test_health_upsert_and_retrieve_with_postgres_backend(self) -> None:
        schema_name = f"qqaat_pgtest_{uuid.uuid4().hex[:12]}"
        document_id = hashlib.sha256("c_pg:d_pg".encode("utf-8")).hexdigest()
        with tempfile.TemporaryDirectory() as temp_dir:
            settings = Settings(
                data_dir=Path(temp_dir),
                internal_auth_token="test-token",
                embedding_dim=64,
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
                index_backend="postgres",
                postgres_dsn=self.postgres_dsn,
                postgres_schema=schema_name,
                postgres_connect_timeout=5,
            )
            client = TestClient(create_app(settings))
            headers = {"Authorization": "Bearer test-token"}

            health = client.get("/internal/v1/health")
            self.assertEqual(health.status_code, 200)
            health_payload = health.json()
            self.assertEqual(health_payload["index_backend"], "postgres_pgvector")
            self.assertEqual(health_payload["db"]["status"], "ok")
            self.assertEqual(health_payload["db"]["vector_mode"], "native")

            upsert = client.post(
                "/internal/v1/index/upsert-batch",
                headers=headers,
                json={
                    "user_id": "u_pg",
                    "scope_id": "c_pg",
                    "name": "验证知识库",
                    "scope_type": "knowledge_connection",
                    "provider": "yuque",
                    "documents": [
                        {
                            "external_id": "d_pg",
                            "title": "Postgres 验证文档",
                            "repo": "验证知识库",
                            "doc_ref": "postgres-check",
                            "source_url": "https://example.com/postgres-check",
                            "updated_at": "2026-04-26T22:00:00+08:00",
                            "raw_body": "QuickQue 的 llm postgres backend 已经可以完成健康检查、索引写入和检索回读。",
                        }
                    ],
                },
            )
            self.assertEqual(upsert.status_code, 200)
            upsert_payload = upsert.json()
            self.assertEqual(upsert_payload["document_count"], 1)
            self.assertEqual(upsert_payload["chunk_count"], 1)
            self.assertTrue(upsert_payload["job_id"])
            self.assertEqual(upsert_payload["changed_documents"], {"added": 1, "updated": 0, "deleted": 0, "unchanged": 0})

            unchanged = client.post(
                "/internal/v1/index/upsert-batch",
                headers=headers,
                json={
                    "user_id": "u_pg",
                    "scope_id": "c_pg",
                    "name": "验证知识库",
                    "scope_type": "knowledge_connection",
                    "provider": "yuque",
                    "sync_mode": "replace",
                    "documents": [
                        {
                            "external_id": "d_pg",
                            "title": "Postgres 验证文档",
                            "repo": "验证知识库",
                            "doc_ref": "postgres-check",
                            "source_url": "https://example.com/postgres-check",
                            "updated_at": "2026-04-26T22:00:00+08:00",
                            "raw_body": "QuickQue 的 llm postgres backend 已经可以完成健康检查、索引写入和检索回读。",
                        }
                    ],
                },
            )
            self.assertEqual(unchanged.status_code, 200)
            self.assertEqual(unchanged.json()["changed_documents"], {"added": 0, "updated": 0, "deleted": 0, "unchanged": 1})

            updated = client.post(
                "/internal/v1/index/upsert-batch",
                headers=headers,
                json={
                    "user_id": "u_pg",
                    "scope_id": "c_pg",
                    "name": "验证知识库",
                    "scope_type": "knowledge_connection",
                    "provider": "yuque",
                    "sync_mode": "replace",
                    "documents": [
                        {
                            "external_id": "d_pg",
                            "title": "Postgres 验证文档",
                            "repo": "验证知识库",
                            "doc_ref": "postgres-check",
                            "source_url": "https://example.com/postgres-check",
                            "updated_at": "2026-04-27T08:00:00+08:00",
                            "raw_body": "QuickQue 的 llm postgres backend 已经可以完成健康检查、索引写入、检索回读，并支持版本化文档。",
                        }
                    ],
                },
            )
            self.assertEqual(updated.status_code, 200)
            self.assertEqual(updated.json()["changed_documents"], {"added": 0, "updated": 1, "deleted": 0, "unchanged": 0})

            job = client.get(f"/internal/v1/index/jobs/{upsert_payload['job_id']}", headers=headers)
            self.assertEqual(job.status_code, 200)
            job_payload = job.json()
            self.assertEqual(job_payload["status"], "completed")
            self.assertEqual(job_payload["scope_id"], "c_pg")
            self.assertEqual(job_payload["job_type"], "upsert_batch")

            retrieve = client.post(
                "/internal/v1/retrieve",
                headers=headers,
                json={
                    "user_id": "u_pg",
                    "scope_ids": ["c_pg"],
                    "query": "postgres backend 可以做什么",
                },
            )
            self.assertEqual(retrieve.status_code, 200)
            retrieve_payload = retrieve.json()
            self.assertTrue(retrieve_payload["trace_id"])
            self.assertEqual(len(retrieve_payload["sources"]), 1)
            self.assertEqual(retrieve_payload["sources"][0]["title"], "Postgres 验证文档")
            self.assertTrue(retrieve_payload["sources"][0]["chunk_id"])

            with psycopg.connect(self.postgres_dsn, row_factory=dict_row) as conn:
                with conn.cursor() as cur:
                    cur.execute(
                        f'''
                        SELECT id, model_profile_id, retrieval_backend, candidate_mode, used_chunk_ids
                        FROM "{schema_name}"."request_traces"
                        WHERE request_type = 'retrieve'
                        ORDER BY created_at DESC
                        LIMIT 1
                        '''
                    )
                    request_trace = cur.fetchone()
                with conn.cursor() as cur:
                    cur.execute(
                        f'''
                        SELECT d.latest_version_id, v.id AS version_id, v.normalized_body
                        FROM "{schema_name}"."documents" d
                        JOIN "{schema_name}"."document_versions" v
                          ON v.id = d.latest_version_id
                        WHERE d.user_id = %s AND d.scope_id = %s
                        ''',
                        ("u_pg", "c_pg"),
                    )
                    version_row = cur.fetchone()
                with conn.cursor() as cur:
                    cur.execute(
                        f'SELECT COUNT(*) AS version_count, MAX(version_no) AS max_version_no, MIN(body_hash) AS min_body_hash FROM "{schema_name}"."document_versions" WHERE document_id = %s',
                        (document_id,),
                    )
                    version_count_row = cur.fetchone()
            self.assertIsNotNone(request_trace)
            self.assertEqual(request_trace["id"], retrieve_payload["trace_id"])
            self.assertEqual(request_trace["retrieval_backend"], "postgres_pgvector")
            self.assertEqual(request_trace["candidate_mode"], "postgres_assisted")
            self.assertGreaterEqual(len(request_trace["used_chunk_ids"]), 1)
            self.assertIsNotNone(version_row)
            self.assertEqual(version_row["latest_version_id"], version_row["version_id"])
            self.assertIn("postgres backend", version_row["normalized_body"])
            self.assertIsNotNone(version_count_row)
            self.assertEqual(version_count_row["version_count"], 2)
            self.assertEqual(version_count_row["max_version_no"], 2)
            self.assertTrue(version_count_row["min_body_hash"])

    def test_quality_regression_suite_with_postgres_backend(self) -> None:
        schema_name = f"qqaat_pgquality_{uuid.uuid4().hex[:12]}"
        with tempfile.TemporaryDirectory() as temp_dir:
            settings = Settings(
                data_dir=Path(temp_dir),
                internal_auth_token="test-token",
                embedding_dim=64,
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
                index_backend="postgres",
                postgres_dsn=self.postgres_dsn,
                postgres_schema=schema_name,
                postgres_connect_timeout=5,
            )
            client = TestClient(create_app(settings))
            runner = QualityEvalRunner(client, token="test-token")
            summary = runner.run_suite(
                user_id="u_pg_quality",
                scopes=QUALITY_SCOPE_FIXTURES,
                retrieval_cases=QUALITY_RETRIEVAL_CASES,
                evidence_cases=QUALITY_EVIDENCE_CASES,
                report_cases=QUALITY_REPORT_CASES,
            )
            self.assertEqual(summary.failed_count, 0, summary.failure_report())

            with psycopg.connect(self.postgres_dsn, row_factory=dict_row) as conn:
                with conn.cursor() as cur:
                    cur.execute(
                        f'''
                        SELECT request_type, COUNT(*) AS trace_count
                        FROM "{schema_name}"."request_traces"
                        GROUP BY request_type
                        ORDER BY request_type ASC
                        '''
                    )
                    rows = cur.fetchall()
            counts = {row["request_type"]: row["trace_count"] for row in rows}
            self.assertGreaterEqual(counts.get("evidence_merge", 0), len(QUALITY_EVIDENCE_CASES))
            self.assertGreaterEqual(counts.get("retrieve", 0), len(QUALITY_RETRIEVAL_CASES))
            self.assertGreaterEqual(counts.get("report", 0), len(QUALITY_REPORT_CASES))

    def test_quality_benchmark_endpoint_with_postgres_backend(self) -> None:
        schema_name = f"qqaat_pgevalapi_{uuid.uuid4().hex[:12]}"
        with tempfile.TemporaryDirectory() as temp_dir:
            settings = Settings(
                data_dir=Path(temp_dir),
                internal_auth_token="test-token",
                embedding_dim=64,
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
                index_backend="postgres",
                postgres_dsn=self.postgres_dsn,
                postgres_schema=schema_name,
                postgres_connect_timeout=5,
            )
            client = TestClient(create_app(settings))
            headers = {"Authorization": "Bearer test-token"}

            suites = client.get("/internal/v1/eval/quality/suites", headers=headers)
            self.assertEqual(suites.status_code, 200)
            self.assertEqual(suites.json()[0]["suite_name"], "builtin")

            run = client.post(
                "/internal/v1/eval/quality/run",
                headers=headers,
                json={
                    "user_id": "u_pg_eval_api",
                    "suite_name": "builtin",
                    "cleanup": True,
                    "scope_prefix": "pg_api_eval",
                },
            )
            self.assertEqual(run.status_code, 200)
            payload = run.json()
            self.assertTrue(payload["trace_id"])
            self.assertEqual(payload["failed_count"], 0)
            self.assertEqual(payload["backend_snapshot"]["index_backend"], "postgres_pgvector")
            self.assertEqual(payload["backend_snapshot"]["checks"]["database"], "ok")
            self.assertEqual(payload["backend_snapshot"]["checks"]["qualityBenchmarks"], "ok")
            self.assertIn("evidence_merge", payload["kind_scores"])

            with psycopg.connect(self.postgres_dsn, row_factory=dict_row) as conn:
                with conn.cursor() as cur:
                    cur.execute(
                        f'''
                        SELECT id, request_type, output_preview
                        FROM "{schema_name}"."request_traces"
                        WHERE request_type = 'quality_benchmark'
                        ORDER BY created_at DESC
                        LIMIT 1
                        '''
                    )
                    benchmark_trace = cur.fetchone()
            self.assertIsNotNone(benchmark_trace)
            self.assertEqual(benchmark_trace["id"], payload["trace_id"])
            self.assertEqual(benchmark_trace["request_type"], "quality_benchmark")
            self.assertIn("average_score", benchmark_trace["output_preview"])

    def test_trace_retention_cleanup_with_postgres_backend(self) -> None:
        schema_name = f"qqaat_pgtracegc_{uuid.uuid4().hex[:12]}"
        with tempfile.TemporaryDirectory() as temp_dir:
            settings = Settings(
                data_dir=Path(temp_dir),
                internal_auth_token="test-token",
                embedding_dim=64,
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
                index_backend="postgres",
                postgres_dsn=self.postgres_dsn,
                postgres_schema=schema_name,
                postgres_connect_timeout=5,
            )
            app = create_app(settings)
            adapter = app.state.container.trace_repo.db

            old_default = datetime.now(timezone.utc) - timedelta(days=45)
            old_report = datetime.now(timezone.utc) - timedelta(days=120)
            with psycopg.connect(self.postgres_dsn, row_factory=dict_row) as conn:
                with conn.cursor() as cur:
                    cur.execute(
                        f'''
                        INSERT INTO "{schema_name}"."request_traces" (
                            id, request_type, user_id, output_preview, payload, created_at
                        ) VALUES
                            ('trace_old_chat', 'chat', 'u_gc', 'old chat', '{{}}'::jsonb, %s),
                            ('trace_old_report', 'report', 'u_gc', 'old report', '{{}}'::jsonb, %s),
                            ('trace_old_benchmark', 'quality_benchmark', 'u_gc', 'old benchmark', '{{}}'::jsonb, %s)
                        ''',
                        (old_default, old_report, old_default),
                    )
                    cur.execute(
                        f'''
                        INSERT INTO "{schema_name}"."traces" (payload, created_at) VALUES
                            (%s::jsonb, %s),
                            (%s::jsonb, %s),
                            (%s::jsonb, %s)
                        ''',
                        (
                            '{"trace_id":"trace_old_chat","request_type":"chat"}', old_default,
                            '{"trace_id":"trace_old_report","request_type":"report"}', old_report,
                            '{"trace_id":"trace_old_benchmark","request_type":"quality_benchmark"}', old_default,
                        ),
                    )
                conn.commit()

            cleanup = adapter.cleanup_request_traces(trace_retention_days=30, report_trace_retention_days=90)
            self.assertGreaterEqual(cleanup["removed"], 2)

            with psycopg.connect(self.postgres_dsn, row_factory=dict_row) as conn:
                with conn.cursor() as cur:
                    cur.execute(
                        f'''
                        SELECT id FROM "{schema_name}"."request_traces"
                        ORDER BY id ASC
                        '''
                    )
                    remaining = {row["id"] for row in cur.fetchall()}
            self.assertNotIn("trace_old_chat", remaining)
            self.assertNotIn("trace_old_report", remaining)
            self.assertIn("trace_old_benchmark", remaining)


if __name__ == "__main__":
    unittest.main()
