from __future__ import annotations

from dataclasses import replace
from datetime import datetime, timezone
import uuid
from typing import Any, Dict, List

from qqa_llm.eval.quality_runner import (
    EvalCaseResult,
    EvalDocumentFixture,
    EvalScopeFixture,
    QualityEvalExecutor,
    QualityEvalRunner,
)
from qqa_llm.eval.fixtures import (
    QUALITY_EVIDENCE_CASES,
    QUALITY_REPORT_CASES,
    QUALITY_RETRIEVAL_CASES,
    QUALITY_SCOPE_FIXTURES,
)
from qqa_llm.services.evidence_service import EvidenceService
from qqa_llm.services.indexing_service import IndexingService
from qqa_llm.services.report_service import ReportService
from qqa_llm.services.retrieval_service import RetrievalService
from qqa_llm.services.scope_service import ScopeService
from qqa_llm.storage.trace_repo import TraceRepository


class ServiceQualityEvalExecutor(QualityEvalExecutor):
    """Run the benchmark against in-process services.

    This keeps the benchmark available as a first-class llm capability without
    routing production requests through a nested TestClient.
    """

    def __init__(
        self,
        *,
        indexing_service: IndexingService,
        evidence_service: EvidenceService,
        retrieval_service: RetrievalService,
        report_service: ReportService,
        trace_repo: TraceRepository,
    ) -> None:
        self.indexing_service = indexing_service
        self.evidence_service = evidence_service
        self.retrieval_service = retrieval_service
        self.report_service = report_service
        self.trace_repo = trace_repo

    def seed_scope(self, *, user_id: str, scope: EvalScopeFixture) -> None:
        self.indexing_service.upsert_documents(
            user_id=user_id,
            scope_id=scope.scope_id,
            scope_name=scope.name,
            scope_type="benchmark_fixture",
            provider=scope.provider,
            sync_mode="replace",
            documents=[item.to_payload() for item in scope.documents],
        )

    def retrieve(self, *, user_id: str, scope_ids: List[str], query: str, top_k: int) -> Dict[str, Any]:
        trace_id = str(uuid.uuid4())
        base_trace = {
            "id": trace_id,
            "trace_id": trace_id,
            "type": "retrieve",
            "request_type": "retrieve",
            "user_id": user_id,
            "scope_ids": scope_ids,
            "query_text": query,
            "model_profile_id": "",
            "created_at": datetime.now(timezone.utc).isoformat(),
        }
        self.trace_repo.create_trace(base_trace)
        try:
            result = self.retrieval_service.retrieve(
                user_id=user_id,
                scope_ids=scope_ids,
                query=query,
                top_k=top_k,
            )
        except Exception as exc:
            self.trace_repo.mark_trace_error(
                trace_id,
                code="BENCHMARK_RETRIEVE_ERROR",
                message=str(exc),
                updates=base_trace,
            )
            raise
        result.trace_id = trace_id
        self.trace_repo.update_trace_metrics(
            trace_id,
            **{
                **base_trace,
                "retrieved_chunk_ids": result.diagnostics.fused_chunk_ids,
                "reranked_chunk_ids": result.diagnostics.reranked_chunk_ids,
                "used_chunk_ids": result.diagnostics.used_chunk_ids,
                "latency_retrieve_ms": result.diagnostics.latency_total_ms,
                "latency_generate_ms": 0,
                "latency_total_ms": result.diagnostics.latency_total_ms,
                "retrieval_backend": result.diagnostics.backend,
                "candidate_mode": result.diagnostics.candidate_mode,
                "output_preview": "",
            },
        )
        return {
            "status_code": 200,
            "text": "",
            "payload": {
                "trace_id": trace_id,
                "query": query,
                "sources": result.sources(),
                "metrics": {
                    "retrieve_ms": result.diagnostics.latency_total_ms,
                    "generate_ms": 0,
                    "total_ms": result.diagnostics.latency_total_ms,
                },
            },
        }

    def generate_report(self, *, user_id: str, goal: str, scope_ids: List[str], mode: str) -> Dict[str, Any]:
        retrieval = self.retrieval_service.retrieve(
            user_id=user_id,
            scope_ids=scope_ids,
            query=goal,
        )
        report = self.report_service.generate(
            user_id=user_id,
            scope_ids=scope_ids,
            goal=goal,
            mode=mode,
            passages=retrieval.passages,
            retrieval_diagnostics=retrieval.diagnostics.to_dict(),
        )
        return {
            "status_code": 200,
            "text": "",
            "payload": {
                "trace_id": report.trace_id,
                "summary": report.summary,
                "outline_markdown": report.outline_markdown,
                "draft_markdown": report.draft_markdown,
                "retrieval_markdown": report.retrieval_markdown,
                "report_markdown": report.report_markdown,
                "outline_sections": [item.to_dict() for item in report.outline_sections],
                "draft_sections": [item.to_dict() for item in report.draft_sections],
                "report_sections": [item.to_dict() for item in report.report_sections],
                "sources": report.sources,
                "metrics": report.metrics,
            },
        }

    def merge_evidence(
        self,
        *,
        user_id: str,
        goal: str,
        mode: str,
        evidences: List[Dict[str, Any]],
        top_k: int,
    ) -> Dict[str, Any]:
        result = self.evidence_service.merge(
            user_id=user_id,
            goal=goal,
            mode=mode,
            evidences=evidences,
            skill_snapshot={},
            top_k=top_k,
        )
        return {
            "status_code": 200,
            "text": "",
            "payload": {
                "trace_id": result.trace_id,
                "goal": result.goal,
                "mode": result.mode,
                "evidences": [item.to_dict() for item in result.evidences],
                "duplicate_count": result.duplicate_count,
                "group_count": result.group_count,
                "conflict_count": result.conflict_count,
                "group_labels": result.group_labels,
                "warnings": result.warnings,
                "theme_summaries": result.theme_summaries,
                "conflict_details": result.conflict_details,
                "provider_summary": result.provider_summary,
            },
        }

    def get_trace(self, trace_id: str) -> Dict[str, Any]:
        return self.trace_repo.get(trace_id) or {}


class QualityEvalService:
    """Expose the built-in benchmark as an internal llm capability."""

    def __init__(
        self,
        *,
        indexing_service: IndexingService,
        evidence_service: EvidenceService,
        retrieval_service: RetrievalService,
        report_service: ReportService,
        scope_service: ScopeService,
        trace_repo: TraceRepository,
    ) -> None:
        self.indexing_service = indexing_service
        self.evidence_service = evidence_service
        self.retrieval_service = retrieval_service
        self.report_service = report_service
        self.scope_service = scope_service
        self.trace_repo = trace_repo
        self._executor = ServiceQualityEvalExecutor(
            indexing_service=indexing_service,
            evidence_service=evidence_service,
            retrieval_service=retrieval_service,
            report_service=report_service,
            trace_repo=trace_repo,
        )

    def list_suites(self) -> List[Dict[str, Any]]:
        return [
            {
                "suite_name": "builtin",
                "description": (
                    "Built-in llm quality benchmark covering hybrid retrieval, "
                    "grounded report generation, and trace completeness."
                ),
                "scope_count": len(QUALITY_SCOPE_FIXTURES),
                "retrieval_case_count": len(QUALITY_RETRIEVAL_CASES),
                "evidence_case_count": len(QUALITY_EVIDENCE_CASES),
                "report_case_count": len(QUALITY_REPORT_CASES),
                "retrieval_case_names": [item.name for item in QUALITY_RETRIEVAL_CASES],
                "evidence_case_names": [item.name for item in QUALITY_EVIDENCE_CASES],
                "report_case_names": [item.name for item in QUALITY_REPORT_CASES],
            }
        ]

    def run_suite(
        self,
        *,
        user_id: str,
        suite_name: str = "builtin",
        cleanup: bool = True,
        scope_prefix: str = "",
    ) -> Dict[str, Any]:
        if (suite_name or "builtin").strip() != "builtin":
            raise ValueError(f"unsupported quality suite: {suite_name}")
        namespaced = self._namespace_suite(scope_prefix=scope_prefix)
        runner = QualityEvalRunner(executor=self._executor)
        try:
            summary = runner.run_suite(
                user_id=user_id,
                scopes=namespaced["scopes"],
                retrieval_cases=namespaced["retrieval_cases"],
                evidence_cases=QUALITY_EVIDENCE_CASES,
                report_cases=namespaced["report_cases"],
            )
        finally:
            if cleanup:
                for scope_id in namespaced["scope_ids"]:
                    self.scope_service.delete_scope(user_id, scope_id)
        return {
            "suite_name": "builtin",
            "passed_count": summary.passed_count,
            "failed_count": summary.failed_count,
            "average_score": summary.average_score,
            "kind_scores": summary.kind_scores,
            "markdown_report": summary.markdown_report(),
            "scope_ids": namespaced["scope_ids"],
            "cleanup_applied": cleanup,
            "results": [self._serialize_case_result(item) for item in summary.results],
        }

    def _namespace_suite(self, *, scope_prefix: str) -> Dict[str, Any]:
        # Benchmarks should never collide with live scope ids. Namespacing keeps
        # the suite hermetic and also lets us clean up the temporary scopes in
        # one pass after the run finishes.
        prefix = (scope_prefix or "").strip() or f"benchmark_{uuid.uuid4().hex[:8]}"
        scope_mapping = {
            fixture.scope_id: f"{prefix}_{fixture.scope_id}"
            for fixture in QUALITY_SCOPE_FIXTURES
        }
        scopes = [
            EvalScopeFixture(
                scope_id=scope_mapping[fixture.scope_id],
                name=fixture.name,
                provider=fixture.provider,
                documents=[
                    EvalDocumentFixture(
                        external_id=document.external_id,
                        title=document.title,
                        repo=document.repo,
                        doc_ref=document.doc_ref,
                        source_url=document.source_url,
                        updated_at=document.updated_at,
                        raw_body=document.raw_body,
                    )
                    for document in fixture.documents
                ],
            )
            for fixture in QUALITY_SCOPE_FIXTURES
        ]
        retrieval_cases = [
            replace(
                case,
                scope_ids=[scope_mapping.get(scope_id, scope_id) for scope_id in case.scope_ids],
            )
            for case in QUALITY_RETRIEVAL_CASES
        ]
        report_cases = [
            replace(
                case,
                scope_ids=[scope_mapping.get(scope_id, scope_id) for scope_id in case.scope_ids],
            )
            for case in QUALITY_REPORT_CASES
        ]
        return {
            "scope_ids": list(scope_mapping.values()),
            "scopes": scopes,
            "retrieval_cases": retrieval_cases,
            "report_cases": report_cases,
        }

    def _serialize_case_result(self, result: EvalCaseResult) -> Dict[str, Any]:
        payload = result.to_dict()
        return {
            "name": payload["name"],
            "kind": payload["kind"],
            "passed": payload["passed"],
            "score": payload["score"],
            "trace_id": payload["trace_id"],
            "assertions": payload["assertions"],
            "metrics": payload["metrics"],
        }
