from __future__ import annotations

import hashlib
import re
from datetime import datetime, timezone
from time import perf_counter
import uuid
from typing import Any, Dict, Iterable, List, Optional

from qqa_llm.core.config import Settings
from qqa_llm.domain.models import GeneratedReport, ReportEvidenceDiagnostics, ReportSection, RetrievedPassage
from qqa_llm.inference.generator import build_generation_client
from qqa_llm.prompt.report_builder import ReportPromptBuilder
from qqa_llm.services.model_profile_service import ModelProfileService
from qqa_llm.storage.trace_repo import TraceRepository


class ReportService:
    def __init__(
        self,
        settings: Settings,
        *,
        prompt_builder: ReportPromptBuilder,
        model_profile_service: ModelProfileService,
        trace_repo: TraceRepository,
    ) -> None:
        self.settings = settings
        self.prompt_builder = prompt_builder
        self.model_profile_service = model_profile_service
        self.trace_repo = trace_repo

    def generate(
        self,
        *,
        user_id: str = "",
        scope_ids: Optional[List[str]] = None,
        goal: str,
        mode: str,
        passages,
        evidences: Optional[List[Dict[str, Any]]] = None,
        evidence_diagnostics: Optional[Dict[str, Any]] = None,
        retrieval_diagnostics: Optional[Dict[str, Any]] = None,
        skill_snapshot: Optional[Dict[str, Any]] = None,
        model_profile: Optional[Dict[str, Any]] = None,
        trace_context: Optional[Dict[str, Any]] = None,
        chat_model: str = "",
        chat_api_base: str = "",
        chat_api_key: str = "",
    ) -> GeneratedReport:
        # Run/report generation can use both retrieved passages and explicit
        # evidence objects pushed from Go. Merging them here keeps the llm side
        # as the single owner of report grounding rules.
        trace_context = dict(trace_context or {})
        trace_id = str(trace_context.get("trace_id") or "").strip() or str(uuid.uuid4())
        resolved_passages = list(passages)
        resolved_passages.extend(self.passages_from_evidences(evidences or []))
        resolved_passages = self._dedupe_passages(resolved_passages)
        diagnostics = self._normalize_diagnostics(evidence_diagnostics or {})
        diagnostics = self._enrich_diagnostics_from_passages(diagnostics=diagnostics, passages=resolved_passages)
        resolved_profile = self.model_profile_service.resolve(
            user_id=user_id,
            purpose="report_writer",
            requested_profile=model_profile or {},
            legacy_model=chat_model,
            legacy_api_base=chat_api_base,
            legacy_api_key=chat_api_key,
        )
        base_trace = {
            "id": trace_id,
            "trace_id": trace_id,
            "type": "report",
            "request_type": "report",
            "user_id": user_id,
            "run_id": str(trace_context.get("run_id") or ""),
            "message_id": str(trace_context.get("message_id") or ""),
            "skill_id": str(trace_context.get("skill_id") or ""),
            "scope_ids": list(scope_ids or []),
            "query_text": goal,
            "goal": goal,
            "mode": mode,
            "model_profile_id": resolved_profile.id,
            "created_at": datetime.now(timezone.utc).isoformat(),
        }
        self.trace_repo.create_trace(base_trace)
        # Report synthesis is intentionally staged. Outline captures structure,
        # draft captures grounded content, and the final generation step can
        # refine both into a polished report when a real model backend exists.
        outline_markdown = self._build_outline(goal=goal, mode=mode, passages=resolved_passages, diagnostics=diagnostics)
        summary_markdown = self._build_summary(goal=goal, mode=mode, passages=resolved_passages, diagnostics=diagnostics)
        draft_markdown = self._build_draft(
            goal=goal,
            mode=mode,
            passages=resolved_passages,
            diagnostics=diagnostics,
            outline_markdown=outline_markdown,
            summary_markdown=summary_markdown,
        )
        prompt = self.prompt_builder.build(
            goal=goal,
            mode=mode,
            passages=resolved_passages,
            diagnostics=diagnostics,
            outline_markdown=outline_markdown,
            draft_markdown=draft_markdown,
            skill_prompt=self._extract_skill_prompt(skill_snapshot or {}),
        )
        generator = build_generation_client(self.settings, profile=resolved_profile)
        outline_sections = self._build_outline_sections(outline_markdown)
        draft_sections = self._parse_heading_sections(draft_markdown)
        generation_started = perf_counter()
        try:
            report_markdown = generator.generate(prompt)
            generation_elapsed_ms = int((perf_counter() - generation_started) * 1000)
        except Exception as exc:
            retrieval_trace = dict(retrieval_diagnostics or {})
            self.trace_repo.mark_trace_error(
                trace_id,
                code="GENERATION_ERROR",
                message=str(exc),
                updates={
                    **base_trace,
                    "retrieved_chunk_ids": list(retrieval_trace.get("fused_chunk_ids") or []),
                    "reranked_chunk_ids": list(retrieval_trace.get("reranked_chunk_ids") or []),
                    "used_chunk_ids": list(retrieval_trace.get("used_chunk_ids") or []),
                    "latency_retrieve_ms": int(retrieval_trace.get("latency_total_ms") or 0),
                    "latency_generate_ms": int((perf_counter() - generation_started) * 1000),
                    "latency_total_ms": int(retrieval_trace.get("latency_total_ms") or 0)
                    + int((perf_counter() - generation_started) * 1000),
                    "retrieval_backend": str(retrieval_trace.get("backend") or ""),
                    "candidate_mode": str(retrieval_trace.get("candidate_mode") or ""),
                    "output_preview": summary_markdown[:400],
                    "outline_markdown": outline_markdown,
                    "draft_markdown": draft_markdown,
                    "outline_sections": [item.to_dict() for item in outline_sections],
                    "draft_sections": [item.to_dict() for item in draft_sections],
                    "summary": summary_markdown,
                },
            )
            raise
        report_sections = self._parse_heading_sections(report_markdown)
        retrieval_trace = dict(retrieval_diagnostics or {})
        retrieved_chunk_ids = list(retrieval_trace.get("fused_chunk_ids") or [])
        reranked_chunk_ids = list(retrieval_trace.get("reranked_chunk_ids") or [])
        used_chunk_ids = list(retrieval_trace.get("used_chunk_ids") or []) or [p.chunk_id for p in resolved_passages if p.chunk_id]
        retrieval_elapsed_ms = int(retrieval_trace.get("latency_total_ms") or 0)
        # The product layer needs a human-readable grounding artifact, not
        # just raw trace JSON. We build it here so the llm side stays the
        # single owner of how retrieval, rerank, and final grounding should be
        # explained to users and teammates.
        retrieval_markdown = self._build_retrieval_markdown(
            passages=resolved_passages,
            retrieved_chunk_ids=retrieved_chunk_ids,
            reranked_chunk_ids=reranked_chunk_ids,
            used_chunk_ids=used_chunk_ids,
            retrieval_trace=retrieval_trace,
            retrieval_elapsed_ms=retrieval_elapsed_ms,
            generation_elapsed_ms=generation_elapsed_ms,
            explicit_evidence_count=len(evidences or []),
        )
        self.trace_repo.update_trace_metrics(
            trace_id,
            **{
                **base_trace,
                "source_count": len(resolved_passages),
                "summary": summary_markdown,
                "output_preview": summary_markdown[:400],
                "retrieved_chunk_ids": retrieved_chunk_ids,
                "reranked_chunk_ids": reranked_chunk_ids,
                "used_chunk_ids": used_chunk_ids,
                "latency_retrieve_ms": retrieval_elapsed_ms,
                "latency_generate_ms": generation_elapsed_ms,
                "latency_total_ms": retrieval_elapsed_ms + generation_elapsed_ms,
                "retrieval_backend": str(retrieval_trace.get("backend") or ""),
                "candidate_mode": str(retrieval_trace.get("candidate_mode") or ""),
                "diagnostics": {
                    "duplicate_count": diagnostics.duplicate_count,
                    "group_count": diagnostics.group_count,
                    "conflict_count": diagnostics.conflict_count,
                    "group_labels": diagnostics.group_labels,
                    "warnings": diagnostics.warnings,
                },
                "outline_markdown": outline_markdown,
                "draft_markdown": draft_markdown,
                "outline_sections": [item.to_dict() for item in outline_sections],
                "draft_sections": [item.to_dict() for item in draft_sections],
                "report_sections": [item.to_dict() for item in report_sections],
            },
        )
        return GeneratedReport(
            trace_id=trace_id,
            summary=summary_markdown,
            outline_markdown=outline_markdown,
            draft_markdown=draft_markdown,
            retrieval_markdown=retrieval_markdown,
            report_markdown=report_markdown,
            outline_sections=outline_sections,
            draft_sections=draft_sections,
            report_sections=report_sections,
            sources=[p.to_source_dict() for p in resolved_passages],
            metrics={
                "retrieve_ms": retrieval_elapsed_ms,
                "generate_ms": generation_elapsed_ms,
                "total_ms": retrieval_elapsed_ms + generation_elapsed_ms,
            },
        )

    def passages_from_evidences(self, evidences: List[Dict[str, Any]]) -> List[RetrievedPassage]:
        passages: List[RetrievedPassage] = []
        for index, evidence in enumerate(evidences):
            provider = str(evidence.get("provider") or "knowledge").strip() or "knowledge"
            connection_id = str(
                evidence.get("connection_id") or evidence.get("connectionId") or evidence.get("scope_id") or ""
            ).strip()
            document_id = str(evidence.get("document_id") or evidence.get("documentId") or "").strip()
            content = str(evidence.get("body") or evidence.get("snippet") or "").strip()
            title = str(evidence.get("title") or "Untitled").strip() or "Untitled"
            repo = str(evidence.get("repo") or "").strip()
            url = str(evidence.get("url") or "").strip()
            score = float(evidence.get("score") or 0.0)
            if not content and not title:
                continue
            raw_id = str(evidence.get("chunk_id") or evidence.get("chunkId") or "").strip()
            chunk_id = raw_id or hashlib.sha256(f"{provider}:{connection_id}:{document_id}:{index}".encode("utf-8")).hexdigest()
            passages.append(
                RetrievedPassage(
                    chunk_id=chunk_id,
                    scope_id=connection_id,
                    connection_id=connection_id,
                    document_id=document_id,
                    provider=provider,
                    title=title,
                    url=url,
                    repo=repo,
                    snippet=str(evidence.get("snippet") or content[: self.settings.max_snippet_chars]).strip(),
                    content=content or str(evidence.get("snippet") or "").strip(),
                    score=score,
                )
            )
        return passages

    def _extract_skill_prompt(self, skill_snapshot: Dict[str, Any]) -> str:
        runtime_spec = skill_snapshot.get("runtime_spec") or {}
        if isinstance(runtime_spec, dict):
            instructions = str(runtime_spec.get("instructions") or "").strip()
            if instructions:
                return instructions
        return str(skill_snapshot.get("prompt") or "").strip()

    def _dedupe_passages(self, passages: Iterable[RetrievedPassage]) -> List[RetrievedPassage]:
        seen = set()
        result: List[RetrievedPassage] = []
        for passage in passages:
            # Report generation may receive the same logical evidence twice:
            # once from an explicit Go-side evidence list and once from a fresh
            # scope retrieval. We dedupe by the stable source identity plus the
            # content body, instead of chunk_id alone, so both paths collapse
            # into one passage when they describe the same evidence.
            key = (
                passage.provider,
                passage.connection_id,
                passage.document_id,
                passage.url,
                passage.title,
                passage.content,
            )
            if key in seen:
                continue
            seen.add(key)
            result.append(passage)
        return result

    def _normalize_diagnostics(self, payload: Dict[str, Any]) -> ReportEvidenceDiagnostics:
        # Merge diagnostics come from the explicit evidence-merge stage. We
        # normalize them once here so prompt construction and fallback report
        # generation can speak consistently about evidence quality.
        return ReportEvidenceDiagnostics(
            duplicate_count=int(payload.get("duplicate_count") or 0),
            group_count=int(payload.get("group_count") or 0),
            conflict_count=int(payload.get("conflict_count") or 0),
            group_labels=[str(item).strip() for item in payload.get("group_labels") or [] if str(item).strip()],
            warnings=[str(item).strip() for item in payload.get("warnings") or [] if str(item).strip()],
            theme_summaries=[
                dict(item)
                for item in payload.get("theme_summaries") or payload.get("themeSummaries") or []
                if isinstance(item, dict)
            ],
            conflict_details=[
                dict(item)
                for item in payload.get("conflict_details") or payload.get("conflictDetails") or []
                if isinstance(item, dict)
            ],
            provider_summary={
                str(key): int(value)
                for key, value in (payload.get("provider_summary") or payload.get("providerSummary") or {}).items()
            },
        )

    def _enrich_diagnostics_from_passages(
        self,
        *,
        diagnostics: ReportEvidenceDiagnostics,
        passages: List[RetrievedPassage],
    ) -> ReportEvidenceDiagnostics:
        if not passages:
            return diagnostics
        if not diagnostics.group_labels:
            diagnostics.group_labels = self._unique_titles(passages)[: self.settings.max_sources]
        if not diagnostics.provider_summary:
            provider_summary: Dict[str, int] = {}
            for passage in passages:
                key = (passage.provider or "unknown").strip() or "unknown"
                provider_summary[key] = provider_summary.get(key, 0) + 1
            diagnostics.provider_summary = provider_summary
        if not diagnostics.theme_summaries:
            diagnostics.theme_summaries = [
                {
                    "label": title,
                    "evidence_count": sum(1 for passage in passages if passage.title == title),
                    "provider_summary": {
                        key: value
                        for key, value in diagnostics.provider_summary.items()
                        if any((passage.provider or "unknown") == key and passage.title == title for passage in passages)
                    },
                    "top_titles": [title],
                    "avg_score": round(
                        sum(passage.score for passage in passages if passage.title == title)
                        / max(1, sum(1 for passage in passages if passage.title == title)),
                        4,
                    ),
                    "avg_authority_score": 0.0,
                }
                for title in self._unique_titles(passages)[: self.settings.max_sources]
            ]
        return diagnostics

    def _build_outline(
        self,
        *,
        goal: str,
        mode: str,
        passages: List[RetrievedPassage],
        diagnostics: ReportEvidenceDiagnostics,
    ) -> str:
        # The outline stage is deterministic and cheap. It captures what the
        # final report must cover before any free-form generation happens.
        lines = [
            "# Report Outline",
            f"- Goal: {goal}",
            f"- Mode: {mode}",
            "- Section 1: Executive Summary",
            "- Section 2: Findings",
            "- Section 3: Risks",
            "- Section 4: Recommended Actions",
        ]
        if passages:
            lines.append("- Findings should cover these evidence themes:")
            for title in self._unique_titles(passages):
                lines.append(f"  - {title}")
        if diagnostics.theme_summaries:
            lines.append("- Theme clusters inferred during evidence merge:")
            for item in diagnostics.theme_summaries[: self.settings.max_sources]:
                lines.append(
                    f"  - {item.get('label', 'Untitled')} "
                    f"({int(item.get('evidence_count') or 0)} evidences, providers: {self._format_provider_summary(item.get('provider_summary') or {})})"
                )
        if diagnostics.conflict_count > 0:
            lines.append(f"- Risks must explain {diagnostics.conflict_count} conflict candidate groups.")
        if diagnostics.group_labels:
            lines.append("- Recommended Actions should reference these themes:")
            for label in diagnostics.group_labels[: self.settings.max_sources]:
                lines.append(f"  - {label}")
        return "\n".join(lines)

    def _build_draft(
        self,
        *,
        goal: str,
        mode: str,
        passages: List[RetrievedPassage],
        diagnostics: ReportEvidenceDiagnostics,
        outline_markdown: str,
        summary_markdown: str,
    ) -> str:
        # The draft stage turns the outline into a fully grounded baseline
        # report. Fallback generation returns this draft directly, while real
        # models can still refine it in the final stage.
        section_names = self._outline_sections(outline_markdown)
        executive_heading = section_names[0] if len(section_names) > 0 else "Executive Summary"
        findings_heading = section_names[1] if len(section_names) > 1 else "Findings"
        risks_heading = section_names[2] if len(section_names) > 2 else "Risks"
        actions_heading = section_names[3] if len(section_names) > 3 else "Recommended Actions"
        findings = [f"- {item.title}: {item.snippet}" for item in passages[: self.settings.max_sources]]
        if not findings:
            findings = ["- 当前没有可用证据，无法生成可靠报告。"]
        elif diagnostics.theme_summaries:
            findings.insert(
                0,
                "- 主题聚类: "
                + "；".join(
                    f"{item.get('label', 'Untitled')}（{int(item.get('evidence_count') or 0)} 条）"
                    for item in diagnostics.theme_summaries[:3]
                ),
            )

        risks = []
        if diagnostics.conflict_count > 0:
            risks.append(f"- 当前证据中存在 {diagnostics.conflict_count} 组冲突候选，需要人工复核。")
        risks.extend(f"- {warning}" for warning in diagnostics.warnings[: self.settings.max_sources])
        for detail in diagnostics.conflict_details[: self.settings.max_sources]:
            risks.append(
                f"- 冲突组 {detail.get('label', 'Untitled')}: "
                f"{int(detail.get('variant_count') or 0)} 个版本，建议 {detail.get('recommendation') or '人工核查'}"
            )
        if not risks:
            risks = ["- 当前没有检测到明显的证据冲突。"]

        actions = ["- 优先核查 Findings 中分值最高的来源。"]
        if diagnostics.conflict_count > 0:
            actions.append("- 对冲突候选按来源时间、权威性和正文细节做人工比对。")
        if diagnostics.group_labels:
            actions.append(f"- 围绕这些主题继续补充证据：{'、'.join(diagnostics.group_labels[:4])}。")
        else:
            actions.append("- 若结论仍不够稳健，继续补充更具体的内部或外部证据。")
        if diagnostics.provider_summary:
            actions.append(f"- 当前证据来源分布：{self._format_provider_summary(diagnostics.provider_summary)}。")

        return "\n".join(
            [
                "# Research Report",
                "",
                f"## {executive_heading}",
                summary_markdown,
                "",
                f"## {findings_heading}",
                "\n".join(findings),
                "",
                f"## {risks_heading}",
                "\n".join(risks),
                "",
                f"## {actions_heading}",
                "\n".join(actions),
            ]
        )

    def _build_summary(
        self,
        *,
        goal: str,
        mode: str,
        passages: List[RetrievedPassage],
        diagnostics: ReportEvidenceDiagnostics,
    ) -> str:
        if not passages:
            return f"围绕“{goal}”的 {mode} 研究当前缺少足够证据，因此只能给出有限结论。"
        primary_titles = "、".join(self._unique_titles(passages)[:3])
        summary = f"围绕“{goal}”的 {mode} 研究当前主要基于 {primary_titles} 等证据整理结论。"
        if diagnostics.theme_summaries:
            top_themes = "、".join(str(item.get("label") or "Untitled") for item in diagnostics.theme_summaries[:3])
            summary += f" 主要主题集中在 {top_themes}。"
        if diagnostics.conflict_count > 0:
            summary += f" 现有材料中识别出 {diagnostics.conflict_count} 组冲突候选，结论需要结合人工复核。"
        if diagnostics.provider_summary:
            summary += f" 当前来源分布为 {self._format_provider_summary(diagnostics.provider_summary)}。"
        return summary

    def _format_provider_summary(self, payload: Dict[str, Any]) -> str:
        parts = []
        for provider, count in sorted(payload.items(), key=lambda item: (-int(item[1]), str(item[0]))):
            parts.append(f"{provider}:{int(count)}")
        return " / ".join(parts) if parts else "unknown"

    def _build_retrieval_markdown(
        self,
        *,
        passages: List[RetrievedPassage],
        retrieved_chunk_ids: List[str],
        reranked_chunk_ids: List[str],
        used_chunk_ids: List[str],
        retrieval_trace: Dict[str, Any],
        retrieval_elapsed_ms: int,
        generation_elapsed_ms: int,
        explicit_evidence_count: int,
    ) -> str:
        # This artifact is intentionally optimized for product readability. It
        # mirrors the structured trace fields, but groups them into the three
        # questions a user usually cares about:
        # 1. How did we search?
        # 2. Which chunks survived each ranking stage?
        # 3. Which sources actually grounded the report?
        backend = str(retrieval_trace.get("backend") or "").strip()
        candidate_mode = str(retrieval_trace.get("candidate_mode") or "").strip()
        lexical_candidate_ids = [str(item).strip() for item in retrieval_trace.get("lexical_candidate_ids") or [] if str(item).strip()]
        vector_candidate_ids = [str(item).strip() for item in retrieval_trace.get("vector_candidate_ids") or [] if str(item).strip()]
        chunk_catalog = {
            str(chunk_id).strip(): {
                "title": str((payload or {}).get("title") or "").strip(),
                "repo": str((payload or {}).get("repo") or "").strip(),
                "provider": str((payload or {}).get("provider") or "").strip(),
                "url": str((payload or {}).get("url") or "").strip(),
            }
            for chunk_id, payload in dict(retrieval_trace.get("chunk_catalog") or {}).items()
            if str(chunk_id).strip()
        }
        scope_count = len({item.connection_id for item in passages if item.connection_id})

        lines = ["# Grounding Trace", ""]
        if backend or candidate_mode or retrieved_chunk_ids or reranked_chunk_ids or used_chunk_ids:
            lines.extend(
                [
                    "## Retrieval Path",
                    f"- Backend: `{backend or 'none'}`",
                    f"- Candidate mode: `{candidate_mode or 'explicit_evidence_only'}`",
                    f"- Scope count: {scope_count}",
                    f"- Retrieved candidate chunks: {len(retrieved_chunk_ids)}",
                    f"- Reranked chunks: {len(reranked_chunk_ids)}",
                    f"- Used chunks: {len(used_chunk_ids)}",
                    f"- Explicit evidence items merged from Go: {explicit_evidence_count}",
                    f"- Retrieval latency: {retrieval_elapsed_ms} ms",
                    f"- Generation latency: {generation_elapsed_ms} ms",
                    "",
                    "## Chunk Flow",
                    f"- Lexical candidates: {self._format_chunk_stage(lexical_candidate_ids, chunk_catalog)}",
                    f"- Vector candidates: {self._format_chunk_stage(vector_candidate_ids, chunk_catalog)}",
                    f"- Retrieved after fusion: {self._format_chunk_stage(retrieved_chunk_ids, chunk_catalog)}",
                    f"- Reranked shortlist: {self._format_chunk_stage(reranked_chunk_ids, chunk_catalog)}",
                    f"- Used in report grounding: {self._format_chunk_stage(used_chunk_ids, chunk_catalog)}",
                    "",
                ]
            )
        else:
            lines.extend(
                [
                    "## Retrieval Path",
                    "- No scope retrieval was executed for this report.",
                    f"- Explicit evidence items merged from Go: {explicit_evidence_count}",
                    f"- Generation latency: {generation_elapsed_ms} ms",
                    "",
                ]
            )

        lines.append("## Grounded Sources")
        if not passages:
            lines.append("- 当前没有可用于报告生成的 grounding 来源。")
            return "\n".join(lines)

        for index, passage in enumerate(passages[: self.settings.max_sources], start=1):
            source_label = passage.title or "Untitled"
            if passage.repo:
                source_label += f" ({passage.repo})"
            lines.append(f"{index}. {source_label}")
            lines.append(f"   - Provider: `{passage.provider or 'unknown'}`")
            if passage.url:
                lines.append(f"   - URL: {passage.url}")
            if passage.snippet:
                lines.append(f"   - Snippet: {passage.snippet}")
        return "\n".join(lines)

    def _format_chunk_stage(self, chunk_ids: List[str], chunk_catalog: Dict[str, Dict[str, str]]) -> str:
        if not chunk_ids:
            return "none"
        label_counts: Dict[str, int] = {}
        ordered_labels: List[str] = []
        for chunk_id in chunk_ids:
            label = self._chunk_stage_label(chunk_id, chunk_catalog)
            if label not in label_counts:
                ordered_labels.append(label)
                label_counts[label] = 0
            label_counts[label] += 1

        if len(ordered_labels) <= self.settings.max_sources:
            return "、".join(self._with_count(label, label_counts[label]) for label in ordered_labels)
        visible = "、".join(
            self._with_count(label, label_counts[label]) for label in ordered_labels[: self.settings.max_sources]
        )
        return f"{visible}、... (+{len(ordered_labels) - self.settings.max_sources} more)"

    def _chunk_stage_label(self, chunk_id: str, chunk_catalog: Dict[str, Dict[str, str]]) -> str:
        meta = chunk_catalog.get(str(chunk_id).strip()) or {}
        title = str(meta.get("title") or "").strip()
        repo = str(meta.get("repo") or "").strip()
        provider = str(meta.get("provider") or "").strip()
        if not title:
            normalized_id = str(chunk_id).strip()
            short_id = normalized_id[:8] if normalized_id else "unknown"
            return f"Chunk {short_id}"
        suffix = repo or provider
        if suffix:
            return f"《{title}》 ({suffix})"
        return f"《{title}》"

    def _with_count(self, label: str, count: int) -> str:
        if count <= 1:
            return label
        return f"{label} ×{count}"

    def _unique_titles(self, passages: Iterable[RetrievedPassage]) -> List[str]:
        seen = set()
        titles: List[str] = []
        for passage in passages:
            title = (passage.title or "Untitled").strip() or "Untitled"
            if title in seen:
                continue
            seen.add(title)
            titles.append(title)
        return titles

    def _outline_sections(self, outline_markdown: str) -> List[str]:
        names: List[str] = []
        for line in outline_markdown.splitlines():
            if ": " not in line or not line.startswith("- Section"):
                continue
            names.append(line.split(": ", 1)[1].strip())
        return names

    def _build_outline_sections(self, outline_markdown: str) -> List[ReportSection]:
        sections: List[ReportSection] = []
        current_title = ""
        current_lines: List[str] = []
        current_index = 0
        for line in outline_markdown.splitlines():
            if line.startswith("- Section") and ": " in line:
                if current_title:
                    sections.append(
                        ReportSection(
                            key=self._section_key(current_title, index=current_index),
                            title=current_title,
                            content_markdown="\n".join(current_lines).strip(),
                            heading_level=2,
                        )
                    )
                current_title = line.split(": ", 1)[1].strip()
                current_lines = [line]
                current_index += 1
                continue
            if current_title:
                current_lines.append(line)
        if current_title:
            sections.append(
                ReportSection(
                    key=self._section_key(current_title, index=current_index),
                    title=current_title,
                    content_markdown="\n".join(current_lines).strip(),
                    heading_level=2,
                )
            )
        return sections

    def _parse_heading_sections(self, markdown: str) -> List[ReportSection]:
        sections: List[ReportSection] = []
        current_title = ""
        current_lines: List[str] = []
        current_level = 2
        current_index = 0
        for line in markdown.splitlines():
            heading_match = re.match(r"^(#{2,6})\s+(.+?)\s*$", line)
            if heading_match:
                if current_title:
                    sections.append(
                        ReportSection(
                            key=self._section_key(current_title, index=current_index),
                            title=current_title,
                            content_markdown="\n".join(current_lines).strip(),
                            heading_level=current_level,
                        )
                    )
                current_level = len(heading_match.group(1))
                current_title = heading_match.group(2).strip()
                current_lines = [line]
                current_index += 1
                continue
            if current_title:
                current_lines.append(line)
        if current_title:
            sections.append(
                ReportSection(
                    key=self._section_key(current_title, index=current_index),
                    title=current_title,
                    content_markdown="\n".join(current_lines).strip(),
                    heading_level=current_level,
                )
            )
        return sections

    def _section_key(self, title: str, *, index: int) -> str:
        normalized = re.sub(r"[^a-z0-9]+", "_", title.strip().lower())
        normalized = normalized.strip("_")
        if normalized:
            return normalized
        return f"section_{index}"
