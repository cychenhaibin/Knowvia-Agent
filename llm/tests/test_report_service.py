from __future__ import annotations

import json
import tempfile
import unittest
from pathlib import Path

from qqa_llm.core.config import Settings
from qqa_llm.domain.models import RetrievedPassage
from qqa_llm.prompt.report_builder import ReportPromptBuilder
from qqa_llm.services.model_profile_service import ModelProfileService
from qqa_llm.services.report_service import ReportService
from qqa_llm.storage.trace_repo import TraceRepository
from qqa_llm.storage.file_repo import FileStore
from qqa_llm.storage.model_profile_repo import ModelProfileRepository


class ReportServiceDedupTest(unittest.TestCase):
    def test_same_logical_evidence_is_deduped(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            settings = Settings(
                data_dir=Path(temp_dir),
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
            file_store = FileStore(Path(temp_dir))
            trace_repo = TraceRepository(FileStore(Path(temp_dir)), Path(temp_dir) / "traces.jsonl")
            service = ReportService(
                settings,
                prompt_builder=ReportPromptBuilder(),
                model_profile_service=ModelProfileService(
                    settings,
                    ModelProfileRepository(file_store, Path(temp_dir) / "model_profiles.json"),
                ),
                trace_repo=trace_repo,
            )

            report = service.generate(
                goal="总结 QuickQue 四月版本变化",
                mode="report",
                passages=[
                    RetrievedPassage(
                        chunk_id="chunk-1",
                        scope_id="conn-1",
                        connection_id="conn-1",
                        document_id="doc-1",
                        provider="yuque",
                        title="发布说明",
                        url="https://example.com/release",
                        repo="产品文档",
                        snippet="QuickQue 在四月新增了 timeline 和 artifact 视图。",
                        content="QuickQue 在四月新增了 timeline 和 artifact 视图。",
                        score=1.0,
                    )
                ],
                evidences=[
                    {
                        "provider": "yuque",
                        "connection_id": "conn-1",
                        "document_id": "doc-1",
                        "title": "发布说明",
                        "url": "https://example.com/release",
                        "repo": "产品文档",
                        "snippet": "QuickQue 在四月新增了 timeline 和 artifact 视图。",
                        "body": "QuickQue 在四月新增了 timeline 和 artifact 视图。",
                        "score": 1.0,
                    }
                ],
                evidence_diagnostics={
                    "duplicate_count": 1,
                    "group_count": 1,
                    "conflict_count": 1,
                    "group_labels": ["发布说明"],
                    "warnings": ["Evidence group '发布说明' contains 2 non-identical variants."],
                    "theme_summaries": [
                        {
                            "label": "发布说明",
                            "evidence_count": 2,
                            "provider_summary": {"yuque": 2},
                            "top_titles": ["发布说明"],
                            "avg_score": 1.05,
                            "avg_authority_score": 1.1,
                        }
                    ],
                    "conflict_details": [
                        {
                            "label": "发布说明",
                            "variant_count": 2,
                            "titles": ["发布说明"],
                            "providers": ["yuque"],
                            "recommendation": "优先核查发布时间、正文细节和来源权威性后再下结论。",
                        }
                    ],
                    "provider_summary": {"yuque": 2},
                },
                retrieval_diagnostics={
                    "backend": "postgres_pgvector",
                    "candidate_mode": "postgres_assisted",
                    "lexical_candidate_ids": ["chunk-1", "chunk-2"],
                    "vector_candidate_ids": ["chunk-1"],
                    "fused_chunk_ids": ["chunk-1"],
                    "reranked_chunk_ids": ["chunk-1"],
                    "used_chunk_ids": ["chunk-1"],
                    "chunk_catalog": {
                        "chunk-1": {
                            "title": "发布说明",
                            "repo": "产品文档",
                            "provider": "yuque",
                            "url": "https://example.com/release",
                        },
                        "chunk-2": {
                            "title": "发布时间",
                            "repo": "产品文档",
                            "provider": "yuque",
                            "url": "https://example.com/release",
                        },
                    },
                    "latency_total_ms": 42,
                },
            )

            self.assertEqual(
                report.report_markdown.count("- 发布说明: QuickQue 在四月新增了 timeline 和 artifact 视图。"),
                1,
            )
            self.assertIn("# Report Outline", report.outline_markdown)
            self.assertIn("Theme clusters inferred during evidence merge", report.outline_markdown)
            self.assertIn("## Executive Summary", report.draft_markdown)
            self.assertIn("主题聚类", report.draft_markdown)
            self.assertIn("围绕“总结 QuickQue 四月版本变化”", report.summary)
            self.assertIn("主要主题集中在 发布说明", report.summary)
            self.assertIn("# Grounding Trace", report.retrieval_markdown)
            self.assertIn("postgres_pgvector", report.retrieval_markdown)
            self.assertIn("Used in report grounding", report.retrieval_markdown)
            self.assertIn("《发布说明》 (产品文档)", report.retrieval_markdown)
            self.assertNotIn("`chunk-1`", report.retrieval_markdown)
            self.assertTrue(report.trace_id)
            self.assertEqual(len(report.sources), 1)
            self.assertIn("## Risks", report.report_markdown)
            self.assertIn("冲突候选", report.report_markdown)
            self.assertIn("Evidence group '发布说明' contains 2 non-identical variants.", report.report_markdown)
            self.assertEqual([item.title for item in report.outline_sections], ["Executive Summary", "Findings", "Risks", "Recommended Actions"])
            self.assertEqual([item.title for item in report.draft_sections], ["Executive Summary", "Findings", "Risks", "Recommended Actions"])
            self.assertEqual([item.title for item in report.report_sections], ["Executive Summary", "Findings", "Risks", "Recommended Actions"])
            self.assertTrue(all(item.content_markdown for item in report.report_sections))
            trace_lines = (Path(temp_dir) / "traces.jsonl").read_text(encoding="utf-8").strip().splitlines()
            self.assertGreaterEqual(len(trace_lines), 2)
            trace_payload = json.loads(trace_lines[-1])
            self.assertEqual(trace_payload["request_type"], "report")
            self.assertTrue(trace_payload["trace_id"])
            self.assertTrue(trace_payload["model_profile_id"])
            self.assertEqual(trace_payload["query_text"], "总结 QuickQue 四月版本变化")
            self.assertIn("outline_markdown", trace_payload)
            self.assertIn("# Report Outline", trace_payload["outline_markdown"])
            self.assertIn("draft_markdown", trace_payload)
            self.assertIn("## Executive Summary", trace_payload["draft_markdown"])
            self.assertIn("outline_sections", trace_payload)
            self.assertEqual(len(trace_payload["outline_sections"]), 4)
            self.assertIn("report_sections", trace_payload)
            self.assertEqual(len(trace_payload["report_sections"]), 4)
            self.assertEqual(trace_payload["used_chunk_ids"], ["chunk-1"])
            self.assertTrue(trace_payload["output_preview"])
            self.assertIn("latency_generate_ms", trace_payload)
            self.assertIn("latency_total_ms", trace_payload)


if __name__ == "__main__":
    unittest.main()
