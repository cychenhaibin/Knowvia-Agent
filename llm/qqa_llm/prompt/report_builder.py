from __future__ import annotations

from qqa_llm.domain.models import ReportEvidenceDiagnostics, ReportPrompt, RetrievedPassage
from qqa_llm.prompt.policy import REPORT_SYSTEM_POLICY


class ReportPromptBuilder:
    def build(
        self,
        *,
        goal: str,
        mode: str,
        passages: list[RetrievedPassage],
        diagnostics: ReportEvidenceDiagnostics,
        outline_markdown: str = "",
        draft_markdown: str = "",
        skill_prompt: str = "",
    ) -> ReportPrompt:
        # Report prompts carry both evidence content and evidence-quality
        # diagnostics so the model can surface uncertainty, not just findings.
        lines = [
            f"研究目标：{goal}",
            f"输出模式：{mode}",
            "请输出 Executive Summary、Findings、Risks、Recommended Actions 四个部分。",
        ]
        if skill_prompt:
            lines.append(f"技能指令：{skill_prompt}")
        if diagnostics.group_count or diagnostics.duplicate_count or diagnostics.conflict_count or diagnostics.warnings:
            lines.append("证据整理诊断：")
            lines.append(
                "分组数: "
                f"{diagnostics.group_count} | 去重条目: {diagnostics.duplicate_count} | 冲突候选: {diagnostics.conflict_count}"
            )
            if diagnostics.group_labels:
                lines.append("分组标签: " + "、".join(diagnostics.group_labels))
            if diagnostics.theme_summaries:
                lines.append("主题聚类摘要：")
                for item in diagnostics.theme_summaries:
                    lines.append(
                        f"- {item.get('label', 'Untitled')}: "
                        f"{int(item.get('evidence_count') or 0)} 条证据，"
                        f"providers={item.get('provider_summary') or {}}"
                    )
            if diagnostics.provider_summary:
                lines.append(
                    "来源分布: "
                    + " / ".join(f"{provider}:{count}" for provider, count in diagnostics.provider_summary.items())
                )
            if diagnostics.conflict_details:
                lines.append("冲突细节：")
                for item in diagnostics.conflict_details:
                    lines.append(
                        f"- {item.get('label', 'Untitled')}: "
                        f"{int(item.get('variant_count') or 0)} 个版本，建议 {item.get('recommendation') or '人工核查'}"
                    )
            for warning in diagnostics.warnings:
                lines.append(f"风险提示: {warning}")
        if outline_markdown.strip():
            lines.append("候选大纲：")
            lines.append(outline_markdown.strip())
        if draft_markdown.strip():
            lines.append("候选草稿：")
            lines.append(draft_markdown.strip())
        if passages:
            lines.append("可用证据：")
            for index, passage in enumerate(passages, start=1):
                lines.append(f"[{index}] 标题: {passage.title} | 来源: {passage.repo}")
                lines.append(f"内容: {passage.content}")
        else:
            lines.append("可用证据：当前没有命中任何证据。")
        return ReportPrompt(
            system_prompt=REPORT_SYSTEM_POLICY,
            user_prompt="\n".join(lines),
            goal=goal,
            mode=mode,
            passages=passages,
            diagnostics=diagnostics,
            outline_markdown=outline_markdown,
            draft_markdown=draft_markdown,
        )
