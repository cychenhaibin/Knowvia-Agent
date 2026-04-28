from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Dict, Iterable, List, Optional, Protocol

from fastapi.testclient import TestClient


@dataclass
class EvalDocumentFixture:
    external_id: str
    title: str
    repo: str
    doc_ref: str
    source_url: str
    updated_at: str
    raw_body: str

    def to_payload(self) -> Dict[str, Any]:
        return {
            "external_id": self.external_id,
            "title": self.title,
            "repo": self.repo,
            "doc_ref": self.doc_ref,
            "source_url": self.source_url,
            "updated_at": self.updated_at,
            "raw_body": self.raw_body,
        }


@dataclass
class EvalScopeFixture:
    scope_id: str
    name: str
    provider: str
    documents: List[EvalDocumentFixture] = field(default_factory=list)


@dataclass
class RetrievalEvalCase:
    name: str
    query: str
    scope_ids: List[str]
    expected_title: str
    expected_keywords: List[str] = field(default_factory=list)
    top_k: int = 5
    minimum_score: float = 0.75


@dataclass
class ReportEvalCase:
    name: str
    goal: str
    scope_ids: List[str]
    mode: str = "report"
    expected_sections: List[str] = field(default_factory=list)
    expected_keywords: List[str] = field(default_factory=list)
    expected_source_titles: List[str] = field(default_factory=list)
    minimum_score: float = 0.8


@dataclass
class EvidenceEvalCase:
    name: str
    goal: str
    evidences: List[Dict[str, Any]] = field(default_factory=list)
    mode: str = "kb_only"
    expected_theme_labels: List[str] = field(default_factory=list)
    expected_provider_summary: Dict[str, int] = field(default_factory=dict)
    expected_duplicate_count: int = 0
    expected_conflict_count: int = 0
    minimum_score: float = 0.8


@dataclass
class EvalAssertion:
    passed: bool
    message: str


@dataclass
class EvalMetric:
    name: str
    score: float
    weight: float
    detail: str = ""


@dataclass
class EvalCaseResult:
    name: str
    kind: str
    passed: bool
    assertions: List[EvalAssertion] = field(default_factory=list)
    metrics: List[EvalMetric] = field(default_factory=list)
    score: float = 0.0
    trace_id: str = ""

    def to_dict(self) -> Dict[str, Any]:
        return {
            "name": self.name,
            "kind": self.kind,
            "passed": self.passed,
            "score": round(self.score, 4),
            "trace_id": self.trace_id,
            "assertions": [
                {"passed": item.passed, "message": item.message}
                for item in self.assertions
            ],
            "metrics": [
                {
                    "name": item.name,
                    "score": round(item.score, 4),
                    "weight": round(item.weight, 4),
                    "detail": item.detail,
                }
                for item in self.metrics
            ],
        }


@dataclass
class EvalSuiteResult:
    results: List[EvalCaseResult] = field(default_factory=list)

    @property
    def passed_count(self) -> int:
        return sum(1 for item in self.results if item.passed)

    @property
    def failed_count(self) -> int:
        return sum(1 for item in self.results if not item.passed)

    @property
    def average_score(self) -> float:
        if not self.results:
            return 0.0
        return sum(item.score for item in self.results) / len(self.results)

    @property
    def kind_scores(self) -> Dict[str, float]:
        grouped: Dict[str, List[float]] = {}
        for item in self.results:
            grouped.setdefault(item.kind, []).append(item.score)
        return {
            kind: (sum(scores) / len(scores) if scores else 0.0)
            for kind, scores in grouped.items()
        }

    def failure_report(self) -> str:
        lines: List[str] = []
        for result in self.results:
            if result.passed:
                continue
            lines.append(f"[{result.kind}] {result.name} (score={result.score:.2f})")
            for assertion in result.assertions:
                if not assertion.passed:
                    lines.append(f"  - {assertion.message}")
        return "\n".join(lines)

    def markdown_report(self) -> str:
        lines = [
            "# LLM Quality Benchmark",
            "",
            f"- Cases: {len(self.results)}",
            f"- Passed: {self.passed_count}",
            f"- Failed: {self.failed_count}",
            f"- Average score: {self.average_score:.2f}",
        ]
        for kind, score in sorted(self.kind_scores.items()):
            lines.append(f"- {kind} average: {score:.2f}")
        lines.append("")
        lines.append("## Case Results")
        for result in self.results:
            status = "PASS" if result.passed else "FAIL"
            lines.append(f"- [{status}] {result.kind}/{result.name}: {result.score:.2f}")
        return "\n".join(lines)


class QualityEvalExecutor(Protocol):
    """Minimal transport that lets the benchmark run against tests or services.

    The scoring logic should stay identical regardless of whether we execute
    through HTTP test clients or directly against service objects.
    """

    def seed_scope(self, *, user_id: str, scope: EvalScopeFixture) -> None: ...

    def retrieve(self, *, user_id: str, scope_ids: List[str], query: str, top_k: int) -> Dict[str, Any]: ...

    def merge_evidence(
        self,
        *,
        user_id: str,
        goal: str,
        mode: str,
        evidences: List[Dict[str, Any]],
        top_k: int,
    ) -> Dict[str, Any]: ...

    def generate_report(self, *, user_id: str, goal: str, scope_ids: List[str], mode: str) -> Dict[str, Any]: ...

    def get_trace(self, trace_id: str) -> Dict[str, Any]: ...


class HTTPQualityEvalExecutor:
    """Exercise the published internal API from tests.

    We keep this executor for regression tests so the benchmark still verifies
    the full HTTP contract, including serialization and auth handling.
    """

    def __init__(self, client: TestClient, *, token: str) -> None:
        self.client = client
        self.headers = {"Authorization": f"Bearer {token}"}

    def seed_scope(self, *, user_id: str, scope: EvalScopeFixture) -> None:
        response = self.client.post(
            "/internal/v1/index/upsert-batch",
            headers=self.headers,
            json={
                "user_id": user_id,
                "scope_id": scope.scope_id,
                "name": scope.name,
                "scope_type": "knowledge_connection",
                "provider": scope.provider,
                "sync_mode": "replace",
                "documents": [item.to_payload() for item in scope.documents],
            },
        )
        if response.status_code != 200:
            raise AssertionError(f"failed to seed scope {scope.scope_id}: {response.text}")

    def retrieve(self, *, user_id: str, scope_ids: List[str], query: str, top_k: int) -> Dict[str, Any]:
        response = self.client.post(
            "/internal/v1/retrieve",
            headers=self.headers,
            json={
                "user_id": user_id,
                "scope_ids": scope_ids,
                "query": query,
                "top_k": top_k,
                "return_trace": True,
            },
        )
        return {
            "status_code": response.status_code,
            "text": response.text,
            "payload": response.json() if response.headers.get("content-type", "").startswith("application/json") else {},
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
        response = self.client.post(
            "/internal/v1/evidence/merge",
            headers=self.headers,
            json={
                "user_id": user_id,
                "goal": goal,
                "mode": mode,
                "evidences": evidences,
                "top_k": top_k,
            },
        )
        return {
            "status_code": response.status_code,
            "text": response.text,
            "payload": response.json() if response.headers.get("content-type", "").startswith("application/json") else {},
        }

    def generate_report(self, *, user_id: str, goal: str, scope_ids: List[str], mode: str) -> Dict[str, Any]:
        response = self.client.post(
            "/internal/v1/report/generate",
            headers=self.headers,
            json={
                "user_id": user_id,
                "goal": goal,
                "scope_ids": scope_ids,
                "mode": mode,
            },
        )
        return {
            "status_code": response.status_code,
            "text": response.text,
            "payload": response.json() if response.headers.get("content-type", "").startswith("application/json") else {},
        }

    def get_trace(self, trace_id: str) -> Dict[str, Any]:
        response = self.client.get(
            f"/internal/v1/request-traces/{trace_id}",
            headers=self.headers,
        )
        if response.status_code != 200:
            return {}
        return response.json()


class QualityEvalRunner:
    """Exercise the internal API the same way product code does.

    Keeping evaluation at the HTTP boundary makes the regression suite useful
    for both storage backends and future model-profile changes.
    """

    def __init__(
        self,
        client: Optional[TestClient] = None,
        *,
        token: str = "",
        executor: Optional[QualityEvalExecutor] = None,
    ) -> None:
        if executor is not None:
            self.executor = executor
            return
        if client is None:
            raise ValueError("either client or executor must be provided")
        self.executor = HTTPQualityEvalExecutor(client, token=token)

    def seed_scopes(self, *, user_id: str, scopes: Iterable[EvalScopeFixture]) -> None:
        for scope in scopes:
            self.executor.seed_scope(user_id=user_id, scope=scope)

    def run_suite(
        self,
        *,
        user_id: str,
        scopes: Iterable[EvalScopeFixture],
        retrieval_cases: Iterable[RetrievalEvalCase],
        evidence_cases: Iterable[EvidenceEvalCase] = (),
        report_cases: Iterable[ReportEvalCase],
    ) -> EvalSuiteResult:
        self.seed_scopes(user_id=user_id, scopes=scopes)
        results: List[EvalCaseResult] = []
        for case in retrieval_cases:
            results.append(self.run_retrieval_case(user_id=user_id, case=case))
        for case in evidence_cases:
            results.append(self.run_evidence_case(user_id=user_id, case=case))
        for case in report_cases:
            results.append(self.run_report_case(user_id=user_id, case=case))
        return EvalSuiteResult(results=results)

    def run_retrieval_case(self, *, user_id: str, case: RetrievalEvalCase) -> EvalCaseResult:
        response = self.executor.retrieve(
            user_id=user_id,
            scope_ids=case.scope_ids,
            query=case.query,
            top_k=case.top_k,
        )
        assertions: List[EvalAssertion] = []
        if int(response.get("status_code") or 0) != 200:
            assertions.append(
                EvalAssertion(
                    False,
                    f"retrieve returned {response.get('status_code')}: {response.get('text')}",
                )
            )
            return EvalCaseResult(name=case.name, kind="retrieve", passed=False, assertions=assertions)

        payload = dict(response.get("payload") or {})
        sources = payload.get("sources", [])
        titles = [str(item.get("title") or "") for item in sources]
        snippets = "\n".join(str(item.get("snippet") or "") for item in sources)
        trace = self._get_trace(payload.get("trace_id", ""))
        metrics: List[EvalMetric] = []

        assertions.append(EvalAssertion(bool(payload.get("trace_id")), "retrieve should return a non-empty trace_id"))
        assertions.append(
            EvalAssertion(
                case.expected_title in titles,
                f"expected retrieval titles to include {case.expected_title!r}, got {titles!r}",
            )
        )
        for keyword in case.expected_keywords:
            assertions.append(
                EvalAssertion(
                    keyword in snippets,
                    f"expected retrieval snippets for {case.name} to contain keyword {keyword!r}",
                )
            )
        assertions.append(
            EvalAssertion(
                bool(trace.get("retrieved_chunk_ids")),
                f"expected request trace for {case.name} to record retrieved_chunk_ids",
            )
        )
        assertions.append(
            EvalAssertion(
                bool(trace.get("used_chunk_ids")),
                f"expected request trace for {case.name} to record used_chunk_ids",
            )
        )
        keyword_hits = sum(1 for keyword in case.expected_keywords if keyword in snippets)
        keyword_score = 1.0 if not case.expected_keywords else keyword_hits / len(case.expected_keywords)
        metrics.extend(
            [
                EvalMetric(
                    name="title_match",
                    score=1.0 if case.expected_title in titles else 0.0,
                    weight=0.4,
                    detail=f"expected={case.expected_title} got={titles[:3]}",
                ),
                EvalMetric(
                    name="keyword_recall",
                    score=keyword_score,
                    weight=0.3,
                    detail=f"matched {keyword_hits}/{len(case.expected_keywords)} expected keywords",
                ),
                EvalMetric(
                    name="trace_retrieved",
                    score=1.0 if bool(trace.get("retrieved_chunk_ids")) else 0.0,
                    weight=0.15,
                    detail="retrieved_chunk_ids should be present",
                ),
                EvalMetric(
                    name="trace_used",
                    score=1.0 if bool(trace.get("used_chunk_ids")) else 0.0,
                    weight=0.15,
                    detail="used_chunk_ids should be present",
                ),
            ]
        )
        score = self._weighted_score(metrics)
        passed = all(item.passed for item in assertions) and score >= case.minimum_score
        return EvalCaseResult(
            name=case.name,
            kind="retrieve",
            passed=passed,
            assertions=assertions,
            metrics=metrics,
            score=score,
            trace_id=str(payload.get("trace_id") or ""),
        )

    def run_evidence_case(self, *, user_id: str, case: EvidenceEvalCase) -> EvalCaseResult:
        response = self.executor.merge_evidence(
            user_id=user_id,
            goal=case.goal,
            mode=case.mode,
            evidences=case.evidences,
            top_k=max(len(case.evidences), 1),
        )
        assertions: List[EvalAssertion] = []
        if int(response.get("status_code") or 0) != 200:
            assertions.append(
                EvalAssertion(
                    False,
                    f"evidence merge returned {response.get('status_code')}: {response.get('text')}",
                )
            )
            return EvalCaseResult(name=case.name, kind="evidence_merge", passed=False, assertions=assertions)

        payload = dict(response.get("payload") or {})
        evidences = list(payload.get("evidences") or [])
        theme_labels = [str(item.get("label") or "") for item in payload.get("theme_summaries") or []]
        provider_summary = dict(payload.get("provider_summary") or {})
        metrics: List[EvalMetric] = []

        assertions.append(
            EvalAssertion(
                int(payload.get("duplicate_count") or 0) == case.expected_duplicate_count,
                (
                    f"expected duplicate_count={case.expected_duplicate_count}, "
                    f"got {payload.get('duplicate_count')}"
                ),
            )
        )
        assertions.append(
            EvalAssertion(
                int(payload.get("conflict_count") or 0) == case.expected_conflict_count,
                (
                    f"expected conflict_count={case.expected_conflict_count}, "
                    f"got {payload.get('conflict_count')}"
                ),
            )
        )
        for label in case.expected_theme_labels:
            assertions.append(
                EvalAssertion(
                    label in theme_labels,
                    f"expected theme summary labels to include {label!r}, got {theme_labels!r}",
                )
            )
        for provider, expected_count in case.expected_provider_summary.items():
            assertions.append(
                EvalAssertion(
                    int(provider_summary.get(provider) or 0) == expected_count,
                    (
                        f"expected provider_summary[{provider!r}]={expected_count}, "
                        f"got {provider_summary.get(provider)!r}"
                    ),
                )
            )
        assertions.append(
            EvalAssertion(
                bool(evidences) and bool(evidences[0].get("theme_label")) and float(evidences[0].get("authority_score") or 0) > 0,
                "expected merged evidence to carry synthesized theme_label and authority_score",
            )
        )

        theme_hits = sum(1 for label in case.expected_theme_labels if label in theme_labels)
        provider_hits = sum(
            1
            for provider, expected_count in case.expected_provider_summary.items()
            if int(provider_summary.get(provider) or 0) == expected_count
        )
        metrics.extend(
            [
                EvalMetric(
                    name="duplicate_detection",
                    score=1.0 if int(payload.get("duplicate_count") or 0) == case.expected_duplicate_count else 0.0,
                    weight=0.2,
                    detail="duplicate_count should match expected fixture",
                ),
                EvalMetric(
                    name="conflict_detection",
                    score=1.0 if int(payload.get("conflict_count") or 0) == case.expected_conflict_count else 0.0,
                    weight=0.2,
                    detail="conflict_count should match expected fixture",
                ),
                EvalMetric(
                    name="theme_grouping",
                    score=1.0 if not case.expected_theme_labels else theme_hits / len(case.expected_theme_labels),
                    weight=0.25,
                    detail=f"matched {theme_hits}/{len(case.expected_theme_labels)} expected theme labels",
                ),
                EvalMetric(
                    name="provider_distribution",
                    score=1.0 if not case.expected_provider_summary else provider_hits / len(case.expected_provider_summary),
                    weight=0.2,
                    detail=f"matched {provider_hits}/{len(case.expected_provider_summary)} provider counts",
                ),
                EvalMetric(
                    name="evidence_annotations",
                    score=1.0 if bool(evidences) and bool(evidences[0].get('theme_label')) and float(evidences[0].get('authority_score') or 0) > 0 else 0.0,
                    weight=0.15,
                    detail="top merged evidence should expose authority_score and theme_label",
                ),
            ]
        )
        score = self._weighted_score(metrics)
        passed = all(item.passed for item in assertions) and score >= case.minimum_score
        return EvalCaseResult(
            name=case.name,
            kind="evidence_merge",
            passed=passed,
            assertions=assertions,
            metrics=metrics,
            score=score,
            trace_id="",
        )

    def run_report_case(self, *, user_id: str, case: ReportEvalCase) -> EvalCaseResult:
        response = self.executor.generate_report(
            user_id=user_id,
            goal=case.goal,
            scope_ids=case.scope_ids,
            mode=case.mode,
        )
        assertions: List[EvalAssertion] = []
        if int(response.get("status_code") or 0) != 200:
            assertions.append(
                EvalAssertion(
                    False,
                    f"report returned {response.get('status_code')}: {response.get('text')}",
                )
            )
            return EvalCaseResult(name=case.name, kind="report", passed=False, assertions=assertions)

        payload = dict(response.get("payload") or {})
        report_markdown = str(payload.get("report_markdown") or "")
        report_sections = list(payload.get("report_sections") or [])
        report_section_titles = [str(item.get("title") or "") for item in report_sections]
        normalized_report_section_titles = {self._normalize_section_label(item) for item in report_section_titles}
        source_titles = [str(item.get("title") or "") for item in payload.get("sources", [])]
        trace = self._get_trace(payload.get("trace_id", ""))
        metrics: List[EvalMetric] = []

        assertions.append(EvalAssertion(bool(payload.get("trace_id")), "report should return a non-empty trace_id"))
        for section in case.expected_sections:
            normalized_section = self._normalize_section_label(section)
            assertions.append(
                EvalAssertion(
                    section in report_markdown,
                    f"expected report markdown for {case.name} to include section {section!r}",
                )
            )
            assertions.append(
                EvalAssertion(
                    normalized_section in normalized_report_section_titles,
                    f"expected structured report sections for {case.name} to include title {section!r}, got {report_section_titles!r}",
                )
            )
        for keyword in case.expected_keywords:
            assertions.append(
                EvalAssertion(
                    keyword in report_markdown,
                    f"expected report markdown for {case.name} to contain keyword {keyword!r}",
                )
            )
        for title in case.expected_source_titles:
            assertions.append(
                EvalAssertion(
                    title in source_titles,
                    f"expected grounded report sources for {case.name} to include title {title!r}, got {source_titles!r}",
                )
            )
        assertions.append(
            EvalAssertion(
                bool(trace.get("used_chunk_ids")),
                f"expected report trace for {case.name} to record used_chunk_ids",
            )
        )
        assertions.append(
            EvalAssertion(
                str(trace.get("request_type") or "") == "report",
                f"expected report trace for {case.name} to have request_type='report'",
            )
        )
        section_hits = sum(1 for section in case.expected_sections if section in report_markdown)
        structured_section_hits = sum(
            1 for section in case.expected_sections if self._normalize_section_label(section) in normalized_report_section_titles
        )
        keyword_hits = sum(1 for keyword in case.expected_keywords if keyword in report_markdown)
        source_hits = sum(1 for title in case.expected_source_titles if title in source_titles)
        metrics.extend(
            [
                EvalMetric(
                    name="section_coverage",
                    score=1.0 if not case.expected_sections else section_hits / len(case.expected_sections),
                    weight=0.25,
                    detail=f"matched {section_hits}/{len(case.expected_sections)} expected sections",
                ),
                EvalMetric(
                    name="structured_sections",
                    score=1.0 if not case.expected_sections else structured_section_hits / len(case.expected_sections),
                    weight=0.15,
                    detail=f"matched {structured_section_hits}/{len(case.expected_sections)} structured section titles",
                ),
                EvalMetric(
                    name="keyword_coverage",
                    score=1.0 if not case.expected_keywords else keyword_hits / len(case.expected_keywords),
                    weight=0.25,
                    detail=f"matched {keyword_hits}/{len(case.expected_keywords)} expected keywords",
                ),
                EvalMetric(
                    name="source_grounding",
                    score=1.0 if not case.expected_source_titles else source_hits / len(case.expected_source_titles),
                    weight=0.2,
                    detail=f"matched {source_hits}/{len(case.expected_source_titles)} expected source titles",
                ),
                EvalMetric(
                    name="trace_used",
                    score=1.0 if bool(trace.get("used_chunk_ids")) else 0.0,
                    weight=0.1,
                    detail="used_chunk_ids should be present",
                ),
                EvalMetric(
                    name="trace_type",
                    score=1.0 if str(trace.get("request_type") or "") == "report" else 0.0,
                    weight=0.1,
                    detail="request_type should be report",
                ),
                EvalMetric(
                    name="theme_summary",
                    score=1.0 if "主要主题集中在" in str(payload.get("summary") or "") else 0.0,
                    weight=0.05,
                    detail="summary should explain synthesized themes",
                ),
            ]
        )
        score = self._weighted_score(metrics)
        passed = all(item.passed for item in assertions) and score >= case.minimum_score
        return EvalCaseResult(
            name=case.name,
            kind="report",
            passed=passed,
            assertions=assertions,
            metrics=metrics,
            score=score,
            trace_id=str(payload.get("trace_id") or ""),
        )

    def _get_trace(self, trace_id: str) -> Dict[str, Any]:
        return self.executor.get_trace(trace_id)

    def _weighted_score(self, metrics: Iterable[EvalMetric]) -> float:
        total = 0.0
        weights = 0.0
        for metric in metrics:
            total += metric.score * metric.weight
            weights += metric.weight
        if weights <= 0:
            return 0.0
        return round(total / weights, 4)

    def _normalize_section_label(self, label: str) -> str:
        normalized = str(label or "").strip()
        while normalized.startswith("#"):
            normalized = normalized[1:].lstrip()
        return normalized
