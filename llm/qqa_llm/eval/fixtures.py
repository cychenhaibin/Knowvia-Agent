from __future__ import annotations

from qqa_llm.eval.quality_runner import (
    EvidenceEvalCase,
    EvalDocumentFixture,
    EvalScopeFixture,
    ReportEvalCase,
    RetrievalEvalCase,
)


QUALITY_SCOPE_FIXTURES = [
    EvalScopeFixture(
        scope_id="scope_release",
        name="产品发布说明",
        provider="yuque",
        documents=[
            EvalDocumentFixture(
                external_id="doc_release_note",
                title="四月发布说明",
                repo="产品文档",
                doc_ref="release-note-april",
                source_url="https://example.com/release-note-april",
                updated_at="2026-04-25T10:00:00+08:00",
                raw_body=(
                    "Knowvia 在四月新增了 run timeline 和 artifact 视图，"
                    "并优化了最终报告交付体验。"
                ),
            ),
        ],
    ),
    EvalScopeFixture(
        scope_id="scope_auth",
        name="权限系统文档",
        provider="yuque",
        documents=[
            EvalDocumentFixture(
                external_id="doc_auth_model",
                title="权限模型说明",
                repo="权限系统",
                doc_ref="auth-model-april",
                source_url="https://example.com/auth-model-april",
                updated_at="2026-04-24T09:00:00+08:00",
                raw_body=(
                    "四月的权限系统新增了角色模板、审批限制和企业客户可见范围设置，"
                    "用于提升企业客户的配置效率。"
                ),
            ),
        ],
    ),
    EvalScopeFixture(
        scope_id="scope_search",
        name="检索架构文档",
        provider="yuque",
        documents=[
            EvalDocumentFixture(
                external_id="doc_search_strategy",
                title="检索策略说明",
                repo="LLM 架构",
                doc_ref="retrieval-strategy",
                source_url="https://example.com/retrieval-strategy",
                updated_at="2026-04-23T08:00:00+08:00",
                raw_body=(
                    "系统先做关键词召回，再做向量召回，之后通过 RRF 融合候选，"
                    "最后进入 rerank。"
                ),
            ),
        ],
    ),
]


QUALITY_RETRIEVAL_CASES = [
    RetrievalEvalCase(
        name="release_features",
        query="四月新增了什么交付体验相关功能",
        scope_ids=["scope_release", "scope_auth", "scope_search"],
        expected_title="四月发布说明",
        expected_keywords=["timeline", "artifact"],
    ),
    RetrievalEvalCase(
        name="auth_enterprise_visibility",
        query="权限系统有哪些企业客户相关变化",
        scope_ids=["scope_release", "scope_auth"],
        expected_title="权限模型说明",
        expected_keywords=["角色模板", "企业客户可见范围"],
    ),
    RetrievalEvalCase(
        name="hybrid_retrieval_strategy",
        query="检索策略为什么会提到 RRF 融合",
        scope_ids=["scope_search"],
        expected_title="检索策略说明",
        expected_keywords=["关键词召回", "RRF", "rerank"],
    ),
]


QUALITY_REPORT_CASES = [
    ReportEvalCase(
        name="april_release_summary",
        goal="总结 QuickQue 四月的核心变化",
        scope_ids=["scope_release", "scope_auth"],
        expected_sections=["## Executive Summary", "## Findings", "## Risks", "## Recommended Actions"],
        expected_keywords=["timeline", "artifact", "角色模板"],
        expected_source_titles=["四月发布说明", "权限模型说明"],
    ),
]


QUALITY_EVIDENCE_CASES = [
    EvidenceEvalCase(
        name="april_release_merge",
        goal="总结 QuickQue 四月的核心变化",
        mode="kb_only",
        evidences=[
            {
                "provider": "yuque",
                "connection_id": "scope_release",
                "document_id": "doc_release_note",
                "chunk_id": "chunk_release_1",
                "title": "发布说明",
                "repo": "产品文档",
                "url": "https://example.com/release-note-april",
                "snippet": "QuickQue 在四月新增了 run timeline 和 artifact 视图。",
                "body": "QuickQue 在四月新增了 run timeline 和 artifact 视图。",
                "score": 0.8,
            },
            {
                "provider": "yuque",
                "connection_id": "scope_release",
                "document_id": "doc_release_note",
                "chunk_id": "chunk_release_2",
                "title": "发布说明",
                "repo": "产品文档",
                "url": "https://example.com/release-note-april",
                "snippet": "QuickQue 在四月新增了 run timeline 和 artifact 页面。",
                "body": "QuickQue 在四月新增了 run timeline 和 artifact 页面。",
                "score": 0.7,
            },
            {
                "provider": "web",
                "connection_id": "",
                "document_id": "",
                "chunk_id": "chunk_web_1",
                "title": "行业评论",
                "repo": "外部来源",
                "url": "https://example.com/industry-april",
                "snippet": "外部评论也提到 timeline 和 artifact 体验。",
                "body": "外部评论也提到 timeline 和 artifact 体验。",
                "score": 0.6,
            },
            {
                "provider": "web",
                "connection_id": "",
                "document_id": "",
                "chunk_id": "chunk_web_1",
                "title": "行业评论",
                "repo": "外部来源",
                "url": "https://example.com/industry-april",
                "snippet": "外部评论也提到 timeline 和 artifact 体验。",
                "body": "外部评论也提到 timeline 和 artifact 体验。",
                "score": 0.5,
            },
        ],
        expected_theme_labels=["发布说明", "行业评论"],
        expected_provider_summary={"yuque": 2, "web": 1},
        expected_duplicate_count=1,
        expected_conflict_count=1,
    ),
]
