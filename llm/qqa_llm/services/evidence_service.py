from __future__ import annotations

from collections import Counter
from datetime import datetime, timezone
import uuid
from typing import Any, Dict, Iterable, List

from qqa_llm.core.config import Settings
from qqa_llm.domain.models import EvidenceMergeResult, EvidenceRecord
from qqa_llm.retrieve.query_normalizer import tokenize
from qqa_llm.storage.trace_repo import TraceRepository


class EvidenceService:
    def __init__(self, settings: Settings, *, trace_repo: TraceRepository) -> None:
        self.settings = settings
        self.trace_repo = trace_repo

    def merge(
        self,
        *,
        user_id: str,
        goal: str,
        mode: str,
        evidences: List[Dict[str, Any]],
        skill_snapshot: Dict[str, Any],
        top_k: int = 0,
    ) -> EvidenceMergeResult:
        # Evidence merge remains deterministic on purpose, but it now does
        # more than simple sorting: we normalize records, infer themes, apply
        # lightweight authority/support boosts, and surface structured conflict
        # details that later report stages can explain to the user.
        trace_id = str(uuid.uuid4())
        base_trace = {
            "id": trace_id,
            "trace_id": trace_id,
            "type": "evidence_merge",
            "request_type": "evidence_merge",
            "user_id": user_id,
            "query_text": goal,
            "goal": goal,
            "mode": mode,
            "model_profile_id": "",
            "created_at": datetime.now(timezone.utc).isoformat(),
        }
        self.trace_repo.create_trace(base_trace)
        merged, diagnostics = self._dedupe_and_rank(goal=goal, evidences=evidences, top_k=top_k)
        self.trace_repo.update_trace_metrics(
            trace_id,
            **{
                **base_trace,
                "input_count": len(evidences),
                "output_count": len(merged),
                "duplicate_count": diagnostics["duplicate_count"],
                "group_count": diagnostics["group_count"],
                "conflict_count": diagnostics["conflict_count"],
                "warnings": diagnostics["warnings"],
                "theme_summaries": diagnostics["theme_summaries"],
                "conflict_details": diagnostics["conflict_details"],
                "provider_summary": diagnostics["provider_summary"],
                "used_chunk_ids": [item.chunk_id for item in merged if item.chunk_id],
                "output_preview": self._trace_preview(goal=goal, diagnostics=diagnostics),
                "skill_prompt": self._extract_skill_prompt(skill_snapshot),
            },
        )
        return EvidenceMergeResult(
            goal=goal,
            mode=mode,
            trace_id=trace_id,
            evidences=merged,
            duplicate_count=diagnostics["duplicate_count"],
            group_count=diagnostics["group_count"],
            conflict_count=diagnostics["conflict_count"],
            group_labels=diagnostics["group_labels"],
            warnings=diagnostics["warnings"],
            theme_summaries=diagnostics["theme_summaries"],
            conflict_details=diagnostics["conflict_details"],
            provider_summary=diagnostics["provider_summary"],
        )

    def _trace_preview(self, *, goal: str, diagnostics: Dict[str, Any]) -> str:
        labels = [str(item).strip() for item in diagnostics.get("group_labels") or [] if str(item).strip()]
        if labels:
            return f"{goal}: themes={', '.join(labels[:2])}"
        warnings = [str(item).strip() for item in diagnostics.get("warnings") or [] if str(item).strip()]
        if warnings:
            return warnings[0][:200]
        return goal[:200]

    def _dedupe_and_rank(
        self,
        *,
        goal: str,
        evidences: Iterable[Dict[str, Any]],
        top_k: int,
    ) -> tuple[List[EvidenceRecord], Dict[str, Any]]:
        query_tokens = tokenize(goal)
        merged: Dict[tuple[str, ...], EvidenceRecord] = {}
        grouped_variants: Dict[tuple[str, ...], List[EvidenceRecord]] = {}
        input_count = 0
        for raw in evidences:
            input_count += 1
            record = self._normalize_evidence(raw, query_tokens)
            if not record.title and not record.snippet and not record.body:
                continue
            key = (
                record.provider,
                record.connection_id,
                record.document_id,
                record.chunk_id,
                record.url,
                record.title,
                record.body or record.snippet,
            )
            current = merged.get(key)
            if current is None or record.score > current.score:
                merged[key] = record
        deduped_records = list(merged.values())
        duplicate_count = max(0, input_count - len(deduped_records))

        for record in deduped_records:
            group_key = (
                record.provider,
                record.connection_id,
                record.document_id or record.title,
                record.url,
                record.title,
            )
            grouped_variants.setdefault(group_key, []).append(record)

        theme_groups: Dict[str, List[EvidenceRecord]] = {}
        provider_summary_counter: Counter[str] = Counter()
        for record in deduped_records:
            theme_groups.setdefault(record.theme_label or record.title or "Untitled", []).append(record)
            provider_summary_counter[record.provider or "unknown"] += 1

        conflict_count = 0
        warnings: List[str] = []
        group_labels: List[str] = []
        conflict_details: List[Dict[str, Any]] = []
        theme_summaries: List[Dict[str, Any]] = []
        scored_records: List[EvidenceRecord] = []
        for theme_label, theme_items in sorted(
            theme_groups.items(),
            key=lambda item: (-len(item[1]), item[0]),
        ):
            group_labels.append(theme_label)
            provider_counts = Counter(item.provider or "unknown" for item in theme_items)
            avg_score = sum(item.score for item in theme_items) / max(len(theme_items), 1)
            avg_authority_score = sum(item.authority_score for item in theme_items) / max(len(theme_items), 1)
            theme_summaries.append(
                {
                    "label": theme_label,
                    "evidence_count": len(theme_items),
                    "provider_summary": dict(provider_counts),
                    "top_titles": self._top_titles(theme_items),
                    "avg_score": round(avg_score, 4),
                    "avg_authority_score": round(avg_authority_score, 4),
                }
            )

        for _, items in grouped_variants.items():
            label = items[0].title or items[0].url or items[0].repo or "Untitled"
            signatures = {
                " ".join((item.body or item.snippet).strip().lower().split())
                for item in items
                if (item.body or item.snippet).strip()
            }
            if len(signatures) > 1:
                conflict_count += 1
                warnings.append(f"Evidence group '{label}' contains {len(signatures)} non-identical variants.")
                conflict_details.append(
                    {
                        "label": label,
                        "variant_count": len(signatures),
                        "titles": self._top_titles(items),
                        "providers": sorted({item.provider or "unknown" for item in items}),
                        "recommendation": "优先核查发布时间、正文细节和来源权威性后再下结论。",
                    }
                )
            support_boost = max(0, len(items) - 1) * 0.1
            for item in items:
                theme_boost = max(0, len(theme_groups.get(item.theme_label or "", [])) - 1) * 0.05
                scored_records.append(
                    EvidenceRecord(
                        provider=item.provider,
                        connection_id=item.connection_id,
                        document_id=item.document_id,
                        chunk_id=item.chunk_id,
                        title=item.title,
                        repo=item.repo,
                        url=item.url,
                        snippet=item.snippet,
                        body=item.body,
                        score=item.score + support_boost + theme_boost,
                        authority_score=item.authority_score,
                        theme_label=item.theme_label,
                    )
                )

        ranked = sorted(
            scored_records,
            key=lambda item: (-item.score, -item.authority_score, item.theme_label, item.title, item.url),
        )
        if top_k > 0:
            ranked = ranked[:top_k]
        else:
            ranked = ranked[: self.settings.max_sources]
        if provider_summary_counter and not any(provider in {"yuque", "feishu", "knowledge"} for provider in provider_summary_counter):
            warnings.append("当前证据主要来自外部来源，建议补充至少一条内部知识库证据。")
        diagnostics = {
            "duplicate_count": duplicate_count,
            "group_count": len(theme_groups),
            "conflict_count": conflict_count,
            "group_labels": group_labels[: self.settings.max_sources],
            "warnings": warnings[: self.settings.max_sources],
            "theme_summaries": theme_summaries[: self.settings.max_sources],
            "conflict_details": conflict_details[: self.settings.max_sources],
            "provider_summary": dict(provider_summary_counter),
        }
        return ranked, diagnostics

    def _normalize_evidence(self, raw: Dict[str, Any], query_tokens: List[str]) -> EvidenceRecord:
        provider = str(raw.get("provider") or "knowledge").strip() or "knowledge"
        connection_id = str(raw.get("connection_id") or raw.get("connectionId") or "").strip()
        document_id = str(raw.get("document_id") or raw.get("documentId") or "").strip()
        chunk_id = str(raw.get("chunk_id") or raw.get("chunkId") or "").strip()
        title = str(raw.get("title") or "").strip()
        repo = str(raw.get("repo") or "").strip()
        url = str(raw.get("url") or "").strip()
        snippet = str(raw.get("snippet") or "").strip()
        body = str(raw.get("body") or "").strip()
        combined = "\n".join(part for part in [title, repo, snippet, body] if part).lower()
        lexical_boost = sum(1 for token in query_tokens if token and token in combined) * 0.25
        authority_score = self._provider_authority(provider=provider, has_body=bool(body), has_url=bool(url))
        score = float(raw.get("score") or 0.0) + lexical_boost + (authority_score * 0.15)
        return EvidenceRecord(
            provider=provider,
            connection_id=connection_id,
            document_id=document_id,
            chunk_id=chunk_id,
            title=title or "Untitled",
            repo=repo,
            url=url,
            snippet=snippet or body[: self.settings.max_snippet_chars],
            body=body,
            score=score,
            authority_score=authority_score,
            theme_label=self._theme_label(title=title, repo=repo, body=body, snippet=snippet, query_tokens=query_tokens),
        )

    def _provider_authority(self, *, provider: str, has_body: bool, has_url: bool) -> float:
        normalized = (provider or "").strip().lower()
        if normalized in {"yuque", "feishu", "knowledge"}:
            base = 1.0
        elif normalized in {"web", "page", "browser"}:
            base = 0.7
        else:
            base = 0.55
        if has_body:
            base += 0.1
        if has_url:
            base += 0.05
        return round(min(base, 1.2), 4)

    def _theme_label(
        self,
        *,
        title: str,
        repo: str,
        body: str,
        snippet: str,
        query_tokens: List[str],
    ) -> str:
        title = (title or "").strip()
        if title and title != "Untitled":
            return title
        repo = (repo or "").strip()
        if repo:
            return repo
        combined = " ".join(part for part in [snippet, body] if part).strip()
        for token in query_tokens:
            if token and token in combined:
                return f"Theme:{token}"
        return "Untitled"

    def _top_titles(self, items: Iterable[EvidenceRecord]) -> List[str]:
        seen = []
        for item in items:
            title = (item.title or "").strip() or "Untitled"
            if title not in seen:
                seen.append(title)
            if len(seen) >= 3:
                break
        return seen

    def _extract_skill_prompt(self, skill_snapshot: Dict[str, Any]) -> str:
        runtime_spec = skill_snapshot.get("runtime_spec") or {}
        if isinstance(runtime_spec, dict):
            instructions = str(runtime_spec.get("instructions") or "").strip()
            if instructions:
                return instructions
        return str(skill_snapshot.get("prompt") or "").strip()
