from __future__ import annotations

import json
import tempfile
import time
import unittest
from pathlib import Path
from unittest.mock import patch

from fastapi.testclient import TestClient

from qqa_llm.core.config import Settings
from qqa_llm.main import create_app


class _BrokenStreamGenerator:
    def generate_stream(self, _prompt):
        raise RuntimeError("synthetic generator failure")


class InternalAPITest(unittest.TestCase):
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
        self.headers = {"Authorization": "Bearer test-token"}

    def tearDown(self) -> None:
        self.temp_dir.cleanup()

    def test_upsert_retrieve_and_stream_chat(self) -> None:
        health_response = self.client.get("/internal/v1/health")
        self.assertEqual(health_response.status_code, 200)
        health_payload = health_response.json()
        self.assertEqual(health_payload["index_backend"], "file")
        self.assertEqual(health_payload["db"]["status"], "ok")
        self.assertEqual(health_payload["generator"]["backend"], "fallback")
        self.assertEqual(health_payload["embedding"]["backend"], "hash")
        self.assertEqual(health_payload["rerank"]["backend"], "simple")
        self.assertEqual(health_payload["status"], "degraded")
        self.assertEqual(health_payload["checks"]["database"], "ok")
        self.assertEqual(health_payload["checks"]["vectorIndex"], "degraded")
        self.assertEqual(health_payload["checks"]["modelProfiles"], "ok")
        self.assertEqual(health_payload["checks"]["qualityBenchmarks"], "ok")
        self.assertIn("chat_fast", health_payload["model_profiles"])
        self.assertIn("chat_quality", health_payload["model_profiles"])
        self.assertIn("report_writer", health_payload["model_profiles"])
        self.assertEqual(health_payload["model_profiles"]["chat_quality"]["profile_id"], "profile_chat_quality_default")
        self.assertIn("vector index is not using native pgvector execution", health_payload["degraded_reasons"])

        upsert_response = self.client.post(
            "/internal/v1/index/upsert-batch",
            headers=self.headers,
            json={
                "user_id": "u1",
                "scope_id": "c1",
                "name": "产品文档",
                "scope_type": "knowledge_connection",
                "provider": "yuque",
                "documents": [
                    {
                        "external_id": "d1",
                        "title": "发布说明",
                        "repo": "产品文档",
                        "doc_ref": "release-note",
                        "source_url": "https://example.com/release-note",
                        "updated_at": "2026-04-25T10:00:00+08:00",
                        "raw_body": "QuickQue 在四月新增了 run timeline 和 artifact 视图。",
                    }
                ],
            },
        )
        self.assertEqual(upsert_response.status_code, 200)
        upsert_payload = upsert_response.json()
        self.assertEqual(upsert_payload["document_count"], 1)
        self.assertGreater(upsert_payload["chunk_count"], 0)
        self.assertTrue(upsert_payload["job_id"])
        self.assertEqual(upsert_payload["changed_documents"]["added"], 1)
        self.assertEqual(upsert_payload["changed_documents"]["updated"], 0)
        self.assertEqual(upsert_payload["changed_documents"]["deleted"], 0)
        self.assertEqual(upsert_payload["changed_documents"]["unchanged"], 0)

        job_response = self.client.get(
            f"/internal/v1/index/jobs/{upsert_payload['job_id']}",
            headers=self.headers,
        )
        self.assertEqual(job_response.status_code, 200)
        job_payload = job_response.json()
        self.assertEqual(job_payload["status"], "completed")
        self.assertEqual(job_payload["scope_id"], "c1")
        self.assertEqual(job_payload["document_count"], 1)
        self.assertEqual(job_payload["job_type"], "upsert_batch")
        self.assertTrue(job_payload["started_at"])
        self.assertTrue(job_payload["finished_at"])

        retrieve_response = self.client.post(
            "/internal/v1/retrieve",
            headers=self.headers,
            json={"user_id": "u1", "scope_ids": ["c1"], "query": "四月新增了什么"},
        )
        self.assertEqual(retrieve_response.status_code, 200)
        retrieve_payload = retrieve_response.json()
        self.assertTrue(retrieve_payload["trace_id"])
        self.assertEqual(retrieve_payload["query"], "四月新增了什么")
        self.assertGreaterEqual(len(retrieve_payload["sources"]), 1)
        self.assertTrue(retrieve_payload["sources"][0]["chunk_id"])

        merge_response = self.client.post(
            "/internal/v1/evidence/merge",
            headers=self.headers,
            json={
                "user_id": "u1",
                "goal": "总结 QuickQue 四月的核心变化",
                "mode": "kb_only",
                "top_k": 5,
                "evidences": [
                    {
                        "provider": "yuque",
                        "connection_id": "c1",
                        "document_id": "d1",
                        "chunk_id": "chunk-1",
                        "title": "发布说明",
                        "repo": "产品文档",
                        "url": "https://example.com/release-note",
                        "snippet": "QuickQue 在四月新增了 run timeline 和 artifact 视图。",
                        "body": "QuickQue 在四月新增了 run timeline 和 artifact 视图。",
                        "score": 0.8,
                    },
                    {
                        "provider": "yuque",
                        "connection_id": "c1",
                        "document_id": "d1",
                        "chunk_id": "chunk-2",
                        "title": "发布说明",
                        "repo": "产品文档",
                        "url": "https://example.com/release-note",
                        "snippet": "QuickQue 在四月新增了 run timeline 和 artifact 页面。",
                        "body": "QuickQue 在四月新增了 run timeline 和 artifact 页面。",
                        "score": 0.7,
                    },
                    {
                        "provider": "yuque",
                        "connection_id": "c1",
                        "document_id": "d1",
                        "chunk_id": "chunk-2",
                        "title": "发布说明",
                        "repo": "产品文档",
                        "url": "https://example.com/release-note",
                        "snippet": "QuickQue 在四月新增了 run timeline 和 artifact 页面。",
                        "body": "QuickQue 在四月新增了 run timeline 和 artifact 页面。",
                        "score": 0.6,
                    },
                ],
            },
        )
        self.assertEqual(merge_response.status_code, 200)
        merge_payload = merge_response.json()
        self.assertEqual(merge_payload["duplicate_count"], 1)
        self.assertEqual(merge_payload["group_count"], 1)
        self.assertEqual(merge_payload["conflict_count"], 1)
        self.assertTrue(merge_payload["trace_id"])
        self.assertGreaterEqual(len(merge_payload["warnings"]), 1)
        self.assertEqual(merge_payload["provider_summary"]["yuque"], 2)
        self.assertEqual(len(merge_payload["theme_summaries"]), 1)
        self.assertEqual(merge_payload["theme_summaries"][0]["label"], "发布说明")
        self.assertEqual(len(merge_payload["conflict_details"]), 1)
        self.assertEqual(merge_payload["evidences"][0]["theme_label"], "发布说明")
        self.assertGreater(merge_payload["evidences"][0]["authority_score"], 0)

        stream_response = self.client.post(
            "/internal/v1/chat/stream",
            headers=self.headers,
            json={
                "user_id": "u1",
                "message": "四月新增了什么？",
                "scope_ids": ["c1"],
                "mode": "answer",
            },
        )
        self.assertEqual(stream_response.status_code, 200)
        events = []
        for line in stream_response.text.splitlines():
            if not line.startswith("data: "):
                continue
            events.append(json.loads(line[len("data: ") :]))
        self.assertGreaterEqual(len(events), 2)
        self.assertEqual(events[0]["type"], "retrieval")
        self.assertEqual(events[-1]["type"], "done")
        self.assertTrue(events[0]["traceId"])
        self.assertEqual(events[0]["traceId"], events[-1]["traceId"])
        self.assertTrue(events[-1]["sources"])
        self.assertTrue(events[-1]["sources"][0]["chunk_id"])
        self.assertTrue(events[-1]["sources"][0]["matched_answer_lines"])
        self.assertIn("发布说明", stream_response.text)

        report_response = self.client.post(
            "/internal/v1/report/generate",
            headers=self.headers,
            json={
                "user_id": "u1",
                "goal": "总结 QuickQue 四月的核心变化",
                "run_id": "run_u1_report",
                "scope_ids": ["c1"],
                "mode": "report",
                "skill_snapshot": {
                    "prompt": "请强调面向产品交付的价值。",
                    "runtime_spec": {"instructions": "请强调面向产品交付的价值。"},
                },
                "evidence_diagnostics": {
                    "duplicate_count": 1,
                    "group_count": 2,
                    "conflict_count": 1,
                    "group_labels": ["发布说明", "官方公告"],
                    "warnings": ["Evidence group '发布说明' contains 2 non-identical variants."],
                },
                "evidences": [
                    {
                        "provider": "web",
                        "title": "官方公告",
                        "repo": "外部来源",
                        "url": "https://example.com/blog",
                        "snippet": "外部公告提到交付体验提升。",
                        "body": "外部公告提到交付体验提升，并强调了报告与时间线体验。",
                        "score": 0.8,
                    }
                ],
            },
        )
        self.assertEqual(report_response.status_code, 200)
        report_payload = report_response.json()
        self.assertTrue(report_payload["trace_id"])
        self.assertIn("Research Report", report_payload["report_markdown"])
        self.assertIn("# Report Outline", report_payload["outline_markdown"])
        self.assertIn("## Executive Summary", report_payload["draft_markdown"])
        self.assertIn("# Grounding Trace", report_payload["retrieval_markdown"])
        self.assertIn("## Chunk Flow", report_payload["retrieval_markdown"])
        self.assertIn("Used in report grounding", report_payload["retrieval_markdown"])
        self.assertIn("《发布说明》 (产品文档)", report_payload["retrieval_markdown"])
        self.assertNotIn("`chunk-", report_payload["retrieval_markdown"])
        self.assertIn("## Risks", report_payload["report_markdown"])
        self.assertEqual([item["title"] for item in report_payload["outline_sections"]], ["Executive Summary", "Findings", "Risks", "Recommended Actions"])
        self.assertEqual([item["title"] for item in report_payload["draft_sections"]], ["Executive Summary", "Findings", "Risks", "Recommended Actions"])
        self.assertEqual([item["title"] for item in report_payload["report_sections"]], ["Executive Summary", "Findings", "Risks", "Recommended Actions"])
        self.assertIn("冲突候选", report_payload["report_markdown"])
        self.assertIn("围绕“总结 QuickQue 四月的核心变化”", report_payload["summary"])
        self.assertIn("主要主题集中在", report_payload["summary"])
        self.assertIn("来源分布", report_payload["report_markdown"])
        self.assertGreaterEqual(len(report_payload["sources"]), 1)
        self.assertGreaterEqual(report_payload["metrics"]["total_ms"], report_payload["metrics"]["generate_ms"])

        trace_lines = (Path(self.temp_dir.name) / "traces.jsonl").read_text(encoding="utf-8").strip().splitlines()
        trace_payloads = [json.loads(line) for line in trace_lines]
        trace_types = [payload["request_type"] for payload in trace_payloads]
        self.assertIn("retrieve", trace_types)
        self.assertIn("evidence_merge", trace_types)
        self.assertIn("chat", trace_types)
        self.assertIn("report", trace_types)

        retrieve_trace = next(payload for payload in reversed(trace_payloads) if payload["request_type"] == "retrieve")
        self.assertTrue(retrieve_trace["trace_id"])
        self.assertGreaterEqual(len(retrieve_trace["retrieved_chunk_ids"]), 1)
        self.assertGreaterEqual(len(retrieve_trace["reranked_chunk_ids"]), 1)
        self.assertGreaterEqual(len(retrieve_trace["used_chunk_ids"]), 1)
        self.assertIn("latency_retrieve_ms", retrieve_trace)
        self.assertEqual(retrieve_trace["latency_generate_ms"], 0)

        chat_trace = next(payload for payload in reversed(trace_payloads) if payload["request_type"] == "chat")
        self.assertTrue(chat_trace["model_profile_id"])
        self.assertTrue(chat_trace["output_preview"])
        self.assertGreaterEqual(len(chat_trace["retrieved_chunk_ids"]), 1)
        self.assertGreaterEqual(len(chat_trace["used_chunk_ids"]), 1)
        self.assertIn("latency_generate_ms", chat_trace)
        self.assertIn("candidate_mode", chat_trace)

        evidence_trace = next(payload for payload in reversed(trace_payloads) if payload["request_type"] == "evidence_merge")
        self.assertEqual(evidence_trace["trace_id"], merge_payload["trace_id"])
        self.assertTrue(evidence_trace["output_preview"])
        self.assertGreaterEqual(len(evidence_trace["used_chunk_ids"]), 1)

        report_trace = next(payload for payload in reversed(trace_payloads) if payload["request_type"] == "report")
        self.assertTrue(report_trace["model_profile_id"])
        self.assertEqual(report_trace["run_id"], "run_u1_report")
        self.assertTrue(report_trace["output_preview"])
        self.assertGreaterEqual(len(report_trace["used_chunk_ids"]), 1)
        self.assertIn("latency_total_ms", report_trace)
        self.assertIn("retrieval_backend", report_trace)
        self.assertEqual(len(report_trace["report_sections"]), 4)

        trace_detail = self.client.get(
            f"/internal/v1/request-traces/{report_trace['trace_id']}",
            headers=self.headers,
        )
        self.assertEqual(trace_detail.status_code, 200)
        self.assertEqual(trace_detail.json()["trace_id"], report_trace["trace_id"])
        self.assertEqual(trace_detail.json()["request_type"], "report")
        self.assertEqual(len(trace_detail.json()["report_sections"]), 4)

        trace_list = self.client.get(
            "/internal/v1/request-traces",
            headers=self.headers,
            params={"user_id": "u1", "request_type": "chat", "limit": 5},
        )
        self.assertEqual(trace_list.status_code, 200)
        trace_items = trace_list.json()
        self.assertGreaterEqual(len(trace_items), 1)
        self.assertEqual(trace_items[0]["request_type"], "chat")

        filtered_trace_list = self.client.get(
            "/internal/v1/request-traces",
            headers=self.headers,
            params={
                "user_id": "u1",
                "request_type": "report",
                "run_id": "run_u1_report",
                "model_profile_id": report_trace["model_profile_id"],
                "limit": 5,
            },
        )
        self.assertEqual(filtered_trace_list.status_code, 200)
        filtered_items = filtered_trace_list.json()
        self.assertEqual(len(filtered_items), 1)
        self.assertEqual(filtered_items[0]["trace_id"], report_trace["trace_id"])

    def test_quality_benchmark_endpoints(self) -> None:
        suites_response = self.client.get(
            "/internal/v1/eval/quality/suites",
            headers=self.headers,
        )
        self.assertEqual(suites_response.status_code, 200)
        suites_payload = suites_response.json()
        self.assertEqual(len(suites_payload), 1)
        self.assertEqual(suites_payload[0]["suite_name"], "builtin")
        self.assertGreaterEqual(suites_payload[0]["retrieval_case_count"], 1)
        self.assertGreaterEqual(suites_payload[0]["evidence_case_count"], 1)
        self.assertGreaterEqual(suites_payload[0]["report_case_count"], 1)

        run_response = self.client.post(
            "/internal/v1/eval/quality/run",
            headers=self.headers,
            json={
                "user_id": "u_eval",
                "suite_name": "builtin",
                "cleanup": True,
                "scope_prefix": "api_eval",
            },
        )
        self.assertEqual(run_response.status_code, 200)
        run_payload = run_response.json()
        self.assertTrue(run_payload["trace_id"])
        self.assertEqual(run_payload["suite_name"], "builtin")
        self.assertEqual(run_payload["failed_count"], 0)
        self.assertGreaterEqual(run_payload["passed_count"], 1)
        self.assertGreaterEqual(run_payload["average_score"], 0.9)
        self.assertTrue(run_payload["cleanup_applied"])
        self.assertIn("# LLM Quality Benchmark", run_payload["markdown_report"])
        self.assertEqual(run_payload["backend_snapshot"]["checks"]["qualityBenchmarks"], "ok")
        self.assertIn("evidence_merge", run_payload["kind_scores"])
        self.assertIn("retrieve", run_payload["kind_scores"])
        self.assertIn("report", run_payload["kind_scores"])
        self.assertGreaterEqual(len(run_payload["results"]), 1)
        self.assertTrue(run_payload["results"][0]["trace_id"])

        # Benchmark cleanup should remove the temporary scopes from the index.
        retrieve_after_cleanup = self.client.post(
            "/internal/v1/retrieve",
            headers=self.headers,
            json={
                "user_id": "u_eval",
                "scope_ids": run_payload["scope_ids"],
                "query": "四月发布说明",
            },
        )
        self.assertEqual(retrieve_after_cleanup.status_code, 200)
        self.assertEqual(retrieve_after_cleanup.json()["sources"], [])

        benchmark_trace = self.client.get(
            "/internal/v1/request-traces",
            headers=self.headers,
            params={"user_id": "u_eval", "request_type": "quality_benchmark", "limit": 5},
        )
        self.assertEqual(benchmark_trace.status_code, 200)
        benchmark_items = benchmark_trace.json()
        self.assertEqual(len(benchmark_items), 1)
        self.assertEqual(benchmark_items[0]["trace_id"], run_payload["trace_id"])
        self.assertEqual(benchmark_items[0]["request_type"], "quality_benchmark")

    def test_upsert_sync_mode_tracks_changed_documents(self) -> None:
        first = self.client.post(
            "/internal/v1/index/upsert-batch",
            headers=self.headers,
            json={
                "user_id": "u_sync",
                "scope_id": "c_sync",
                "name": "同步知识库",
                "scope_type": "knowledge_connection",
                "provider": "yuque",
                "sync_mode": "replace",
                "documents": [
                    {
                        "external_id": "d1",
                        "title": "文档一",
                        "repo": "同步知识库",
                        "doc_ref": "doc-one",
                        "updated_at": "2026-04-25T10:00:00+08:00",
                        "raw_body": "第一版正文",
                    }
                ],
            },
        )
        self.assertEqual(first.status_code, 200)
        self.assertEqual(first.json()["changed_documents"], {"added": 1, "updated": 0, "deleted": 0, "unchanged": 0})

        second = self.client.post(
            "/internal/v1/index/upsert-batch",
            headers=self.headers,
            json={
                "user_id": "u_sync",
                "scope_id": "c_sync",
                "name": "同步知识库",
                "scope_type": "knowledge_connection",
                "provider": "yuque",
                "sync_mode": "replace",
                "documents": [
                    {
                        "external_id": "d1",
                        "title": "文档一",
                        "repo": "同步知识库",
                        "doc_ref": "doc-one",
                        "updated_at": "2026-04-25T10:00:00+08:00",
                        "raw_body": "第一版正文",
                    }
                ],
            },
        )
        self.assertEqual(second.status_code, 200)
        self.assertEqual(second.json()["changed_documents"], {"added": 0, "updated": 0, "deleted": 0, "unchanged": 1})

        merged = self.client.post(
            "/internal/v1/index/upsert-batch",
            headers=self.headers,
            json={
                "user_id": "u_sync",
                "scope_id": "c_sync",
                "name": "同步知识库",
                "scope_type": "knowledge_connection",
                "provider": "yuque",
                "sync_mode": "merge",
                "documents": [
                    {
                        "external_id": "d2",
                        "title": "文档二",
                        "repo": "同步知识库",
                        "doc_ref": "doc-two",
                        "updated_at": "2026-04-25T11:00:00+08:00",
                        "raw_body": "第二版正文",
                    }
                ],
            },
        )
        self.assertEqual(merged.status_code, 200)
        merged_payload = merged.json()
        self.assertEqual(merged_payload["document_count"], 2)
        self.assertEqual(merged_payload["changed_documents"], {"added": 1, "updated": 0, "deleted": 0, "unchanged": 1})

        replaced = self.client.post(
            "/internal/v1/index/upsert-batch",
            headers=self.headers,
            json={
                "user_id": "u_sync",
                "scope_id": "c_sync",
                "name": "同步知识库",
                "scope_type": "knowledge_connection",
                "provider": "yuque",
                "sync_mode": "replace",
                "documents": [
                    {
                        "external_id": "d2",
                        "title": "文档二",
                        "repo": "同步知识库",
                        "doc_ref": "doc-two",
                        "updated_at": "2026-04-25T12:00:00+08:00",
                        "raw_body": "第二版正文，新增补充内容",
                    }
                ],
            },
        )
        self.assertEqual(replaced.status_code, 200)
        replaced_payload = replaced.json()
        self.assertEqual(replaced_payload["document_count"], 1)
        self.assertEqual(replaced_payload["changed_documents"], {"added": 0, "updated": 1, "deleted": 1, "unchanged": 0})

    def test_async_index_job_can_be_polled_until_completed(self) -> None:
        response = self.client.post(
            "/internal/v1/index/upsert-batch",
            headers=self.headers,
            json={
                "user_id": "u_async",
                "scope_id": "c_async",
                "name": "异步知识库",
                "scope_type": "knowledge_connection",
                "provider": "yuque",
                "sync_mode": "replace",
                "run_async": True,
                "documents": [
                    {
                        "external_id": "d_async",
                        "title": "异步文档",
                        "repo": "异步知识库",
                        "doc_ref": "async-doc",
                        "updated_at": "2026-04-27T12:00:00+08:00",
                        "raw_body": "这条文档用于验证异步索引 job 是否能被轮询到 completed。",
                    }
                ],
            },
        )
        self.assertEqual(response.status_code, 200)
        payload = response.json()
        self.assertEqual(payload["status"], "queued")
        self.assertEqual(payload["execution_mode"], "async")
        self.assertEqual(payload["document_count"], 0)
        self.assertTrue(payload["job_id"])

        job_payload = {}
        for _ in range(50):
            job_response = self.client.get(
                f"/internal/v1/index/jobs/{payload['job_id']}",
                headers=self.headers,
            )
            self.assertEqual(job_response.status_code, 200)
            job_payload = job_response.json()
            if job_payload["status"] == "completed":
                break
            time.sleep(0.02)
        self.assertEqual(job_payload["status"], "completed")
        self.assertEqual(job_payload["document_count"], 1)
        self.assertGreaterEqual(job_payload["chunk_count"], 1)

        retrieve_response = self.client.post(
            "/internal/v1/retrieve",
            headers=self.headers,
            json={
                "user_id": "u_async",
                "scope_ids": ["c_async"],
                "query": "异步索引验证",
            },
        )
        self.assertEqual(retrieve_response.status_code, 200)
        sources = retrieve_response.json()["sources"]
        self.assertGreaterEqual(len(sources), 1)
        self.assertEqual(sources[0]["title"], "异步文档")

    def test_resume_pending_jobs_recovers_queued_scope_work(self) -> None:
        container = self.client.app.state.container
        created_at = "2026-04-27T12:00:00+08:00"
        job_id = "job_recover_async"
        job_payload = container.indexing_service._job_payload(
            job_id=job_id,
            operation="upsert_scope",
            job_type="upsert_batch",
            status="queued",
            user_id="u_resume",
            scope_id="c_resume",
            scope_name="恢复知识库",
            scope_type="knowledge_connection",
            provider="yuque",
            sync_mode="replace",
            created_at=created_at,
            updated_at=created_at,
        )
        job_payload["task_kind"] = "upsert_documents"
        job_payload["task_payload"] = {
            "user_id": "u_resume",
            "scope_id": "c_resume",
            "scope_name": "恢复知识库",
            "scope_type": "knowledge_connection",
            "provider": "yuque",
            "sync_mode": "replace",
            "created_at": created_at,
            "documents": [
                {
                    "external_id": "d_resume",
                    "title": "恢复文档",
                    "repo": "恢复知识库",
                    "doc_ref": "resume-doc",
                    "updated_at": "2026-04-27T12:00:00+08:00",
                    "raw_body": "这条文档用于验证服务重启后会继续处理已排队的索引任务。",
                }
            ],
        }
        container.index_job_repo.upsert(job_id, job_payload)

        container.indexing_service.resume_pending_jobs()

        recovered_job = {}
        for _ in range(50):
            recovered_job = container.index_job_repo.get(job_id) or {}
            if recovered_job.get("status") == "completed":
                break
            time.sleep(0.02)
        self.assertEqual(recovered_job.get("status"), "completed")
        self.assertEqual(recovered_job.get("document_count"), 1)

        retrieve_response = self.client.post(
            "/internal/v1/retrieve",
            headers=self.headers,
            json={
                "user_id": "u_resume",
                "scope_ids": ["c_resume"],
                "query": "服务重启后会继续处理什么",
            },
        )
        self.assertEqual(retrieve_response.status_code, 200)
        sources = retrieve_response.json()["sources"]
        self.assertGreaterEqual(len(sources), 1)
        self.assertEqual(sources[0]["title"], "恢复文档")

    def test_delete_scope_v1_body_route(self) -> None:
        self.client.post(
            "/internal/v1/index/upsert-batch",
            headers=self.headers,
            json={
                "user_id": "u1",
                "scope_id": "c_delete",
                "name": "待删除知识库",
                "scope_type": "knowledge_connection",
                "provider": "yuque",
                "documents": [
                    {
                        "external_id": "d1",
                        "title": "待删除文档",
                        "repo": "待删除知识库",
                        "doc_ref": "doc-1",
                        "updated_at": "2026-04-25T10:00:00+08:00",
                        "raw_body": "scope delete validation",
                    }
                ],
            },
        )
        delete_response = self.client.request(
            "DELETE",
            "/internal/v1/index/scopes/c_delete",
            headers=self.headers,
            json={"user_id": "u1"},
        )
        self.assertEqual(delete_response.status_code, 200)
        self.assertEqual(delete_response.json()["scope_id"], "c_delete")

    def test_camel_case_requests_are_accepted(self) -> None:
        upsert_response = self.client.post(
            "/internal/v1/index/upsert-batch",
            headers=self.headers,
            json={
                "userId": "u2",
                "scopeId": "c2",
                "name": "CamelCase 知识库",
                "scopeType": "knowledge_connection",
                "provider": "yuque",
                "documents": [
                    {
                        "externalId": "d2",
                        "title": "Camel 文档",
                        "repo": "CamelCase 知识库",
                        "docRef": "camel-doc",
                        "sourceUrl": "https://example.com/camel",
                        "sourceUpdatedAt": "2026-04-25T10:00:00+08:00",
                        "rawBody": "Camel case request should still index successfully.",
                        "normalizedBody": "Camel case request should still index successfully.",
                        "metadata": {"authorName": "Alice"},
                    }
                ],
            },
        )
        self.assertEqual(upsert_response.status_code, 200)

        retrieve_response = self.client.post(
            "/internal/v1/retrieve",
            headers=self.headers,
            json={
                "userId": "u2",
                "scopeIds": ["c2"],
                "query": "camel case request",
                "topK": 3,
                "returnTrace": True,
                "trace": {"runId": "run-camel"},
            },
        )
        self.assertEqual(retrieve_response.status_code, 200)
        self.assertTrue(retrieve_response.json()["trace_id"])

        report_response = self.client.post(
            "/internal/v1/report/generate",
            headers=self.headers,
            json={
                "userId": "u2",
                "runId": "run-camel",
                "goal": "Summarize camel case compatibility",
                "scopeIds": ["c2"],
                "mode": "report",
                "modelProfile": {"purpose": "report_writer", "profileId": "report_writer"},
                "skillSnapshot": {"prompt": "Keep it concise."},
            },
        )
        self.assertEqual(report_response.status_code, 200)
        self.assertTrue(report_response.json()["trace_id"])

    def test_model_profiles_can_be_upserted_and_listed(self) -> None:
        upsert_response = self.client.post(
            "/internal/v1/model-profiles/upsert",
            headers=self.headers,
            json={
                "userId": "u_profile",
                "profileId": "profile_chat_quality_local",
                "purpose": "chat_quality",
                "provider": "openai_compatible",
                "name": "Chat Quality Local",
                "baseUrl": "https://example.invalid/v1",
                "apiKeyRef": "env:OPENAI_API_KEY",
                "modelName": "gpt-4.1-mini",
                "temperature": 0.1,
                "maxTokens": 2048,
                "isDefault": True,
            },
        )
        self.assertEqual(upsert_response.status_code, 200)
        self.assertEqual(upsert_response.json()["profile_id"], "profile_chat_quality_local")

        list_response = self.client.get(
            "/internal/v1/model-profiles",
            headers=self.headers,
            params={"user_id": "u_profile"},
        )
        self.assertEqual(list_response.status_code, 200)
        payload = list_response.json()
        ids = {item["id"] for item in payload}
        self.assertIn("profile_chat_fast_default", ids)
        self.assertIn("profile_chat_quality_default", ids)
        self.assertIn("profile_report_default", ids)
        self.assertIn("profile_chat_quality_local", ids)

        profile = next(item for item in payload if item["id"] == "profile_chat_quality_local")
        self.assertEqual(profile["purpose"], "chat_quality")
        self.assertTrue(profile["is_default"])

        get_response = self.client.get(
            "/internal/v1/model-profiles/profile_chat_quality_local",
            headers=self.headers,
            params={"user_id": "u_profile"},
        )
        self.assertEqual(get_response.status_code, 200)
        self.assertEqual(get_response.json()["id"], "profile_chat_quality_local")

        upsert_kb_response = self.client.post(
            "/internal/v1/index/upsert-batch",
            headers=self.headers,
            json={
                "user_id": "u_profile",
                "scope_id": "c_profile",
                "name": "模型配置知识库",
                "scope_type": "knowledge_connection",
                "provider": "yuque",
                "documents": [
                    {
                        "external_id": "d_profile",
                        "title": "模型配置文档",
                        "repo": "模型配置知识库",
                        "doc_ref": "profile-doc",
                        "source_url": "https://example.com/profile-doc",
                        "updated_at": "2026-04-27T15:00:00+08:00",
                        "raw_body": "这条文档用于验证 chat trace 是否会记录自定义 profile。",
                    }
                ],
            },
        )
        self.assertEqual(upsert_kb_response.status_code, 200)

        stream_response = self.client.post(
            "/internal/v1/chat/stream",
            headers=self.headers,
            json={
                "user_id": "u_profile",
                "message": "总结一下模型配置文档",
                "scope_ids": ["c_profile"],
                "modelProfile": {
                    "purpose": "chat_quality",
                    "profileId": "profile_chat_quality_local",
                },
            },
        )
        self.assertEqual(stream_response.status_code, 200)
        events = []
        for line in stream_response.text.splitlines():
            if not line.startswith("data: "):
                continue
            events.append(json.loads(line[len("data: ") :]))
        self.assertGreaterEqual(len(events), 2)
        trace_id = events[-1]["traceId"]
        trace_response = self.client.get(
            f"/internal/v1/request-traces/{trace_id}",
            headers=self.headers,
        )
        self.assertEqual(trace_response.status_code, 200)
        self.assertEqual(trace_response.json()["model_profile_id"], "profile_chat_quality_local")

        delete_response = self.client.request(
            "DELETE",
            "/internal/v1/model-profiles/profile_chat_quality_local",
            headers=self.headers,
            json={"user_id": "u_profile"},
        )
        self.assertEqual(delete_response.status_code, 200)
        self.assertEqual(delete_response.json()["status"], "ok")

        list_after_delete = self.client.get(
            "/internal/v1/model-profiles",
            headers=self.headers,
            params={"user_id": "u_profile"},
        )
        self.assertEqual(list_after_delete.status_code, 200)
        ids_after_delete = {item["id"] for item in list_after_delete.json()}
        self.assertNotIn("profile_chat_quality_local", ids_after_delete)

    def test_skill_delete_v1_body_route(self) -> None:
        upsert_response = self.client.post(
            "/internal/v1/skills/upsert",
            headers=self.headers,
            json={
                "id": "skill_delete_me",
                "user_id": "u_skill",
                "slug": "skill-delete-me",
                "title": "Skill Delete Me",
                "prompt": "helpful skill",
                "mode": "answer",
            },
        )
        self.assertEqual(upsert_response.status_code, 200)

        delete_response = self.client.request(
            "DELETE",
            "/internal/v1/skills/skill_delete_me",
            headers=self.headers,
            json={"user_id": "u_skill"},
        )
        self.assertEqual(delete_response.status_code, 200)
        self.assertEqual(delete_response.json()["status"], "ok")

    def test_chat_error_marks_request_trace(self) -> None:
        upsert_response = self.client.post(
            "/internal/v1/index/upsert-batch",
            headers=self.headers,
            json={
                "user_id": "u_err",
                "scope_id": "c_err",
                "name": "错误验证知识库",
                "scope_type": "knowledge_connection",
                "provider": "yuque",
                "documents": [
                    {
                        "external_id": "d_err",
                        "title": "错误验证文档",
                        "repo": "错误验证知识库",
                        "doc_ref": "error-check",
                        "source_url": "https://example.com/error-check",
                        "updated_at": "2026-04-27T11:00:00+08:00",
                        "raw_body": "这条文档用于验证 chat 失败时是否会更新 request trace。",
                    }
                ],
            },
        )
        self.assertEqual(upsert_response.status_code, 200)

        with patch("qqa_llm.services.chat_service.build_generation_client", return_value=_BrokenStreamGenerator()):
            stream_response = self.client.post(
                "/internal/v1/chat/stream",
                headers=self.headers,
                json={
                    "user_id": "u_err",
                    "message": "帮我总结这条文档",
                    "scope_ids": ["c_err"],
                    "mode": "answer",
                },
            )
        self.assertEqual(stream_response.status_code, 200)
        events = []
        for line in stream_response.text.splitlines():
            if not line.startswith("data: "):
                continue
            events.append(json.loads(line[len("data: ") :]))
        self.assertGreaterEqual(len(events), 2)
        self.assertEqual(events[0]["type"], "retrieval")
        self.assertEqual(events[-1]["type"], "error")
        trace_id = events[-1]["traceId"]
        self.assertTrue(trace_id)

        trace_response = self.client.get(
            f"/internal/v1/request-traces/{trace_id}",
            headers=self.headers,
        )
        self.assertEqual(trace_response.status_code, 200)
        trace_payload = trace_response.json()
        self.assertEqual(trace_payload["trace_id"], trace_id)
        self.assertEqual(trace_payload["request_type"], "chat")
        self.assertEqual(trace_payload["error_code"], "GENERATION_ERROR")
        self.assertEqual(trace_payload["error_message"], "synthetic generator failure")


if __name__ == "__main__":
    unittest.main()
