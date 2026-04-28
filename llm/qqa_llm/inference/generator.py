from __future__ import annotations

from typing import Iterator, Protocol, Union

from qqa_llm.core.config import Settings
from qqa_llm.domain.models import ChatPrompt, ReportPrompt, ResolvedModelProfile
from qqa_llm.inference.ollama_client import OllamaGenerationClient
from qqa_llm.inference.openai_client import OpenAIGenerationClient


class GenerationClient(Protocol):
    def generate_stream(self, prompt: Union[ChatPrompt, ReportPrompt]) -> Iterator[str]:
        ...

    def generate(self, prompt: Union[ChatPrompt, ReportPrompt]) -> str:
        ...


class FallbackGenerationClient:
    def generate_stream(self, prompt: Union[ChatPrompt, ReportPrompt]) -> Iterator[str]:
        yield self.generate(prompt)

    def generate(self, prompt: Union[ChatPrompt, ReportPrompt]) -> str:
        if isinstance(prompt, ChatPrompt):
            if not prompt.passages:
                return "我在当前知识范围内没有检索到足够信息，暂时无法给出可靠结论。"
            if prompt.mode == "summary":
                bullets = [f"- {item.title}: {item.snippet}" for item in prompt.passages[:4]]
                return "基于当前知识片段，关键信息如下：\n" + "\n".join(bullets)
            if prompt.mode == "actions":
                bullets = [f"1. 查看《{item.title}》中的相关片段：{item.snippet}" for item in prompt.passages[:3]]
                return "基于当前知识片段，建议按以下顺序推进：\n" + "\n".join(bullets)
            top = prompt.passages[0]
            return f"基于当前知识片段，最相关的信息来自《{top.title}》：{top.snippet}"
        if prompt.draft_markdown.strip():
            # The fallback backend has no true refinement step, so the
            # synthesized draft becomes the final report. Keeping the staged
            # artifact on the prompt still mirrors the real three-step flow.
            return prompt.draft_markdown.strip()
        diagnostics = prompt.diagnostics
        findings = [f"- {item.title}: {item.snippet}" for item in prompt.passages[:6]]
        if not findings:
            findings = ["- 当前没有可用证据，无法生成可靠报告。"]
        risks = []
        if diagnostics.conflict_count > 0:
            risks.append(f"- 当前证据中存在 {diagnostics.conflict_count} 组冲突候选，需要人工复核。")
        risks.extend(f"- {warning}" for warning in diagnostics.warnings[:3])
        if not risks:
            risks = ["- 当前没有检测到明显的证据冲突。"]
        actions = ["- 优先核查 Findings 中分值最高的来源。"]
        if diagnostics.conflict_count > 0:
            actions.append("- 对冲突候选按来源时间、权威性和正文细节做人工比对。")
        if diagnostics.group_labels:
            actions.append(f"- 围绕这些主题继续补充证据：{'、'.join(diagnostics.group_labels[:4])}。")
        return (
            "# Research Report\n\n"
            "## Executive Summary\n"
            "基于当前证据整理出以下重点。\n\n"
            "## Findings\n"
            + "\n".join(findings)
            + "\n\n## Risks\n"
            + "\n".join(risks)
            + "\n\n## Recommended Actions\n"
            + "\n".join(actions)
        )


def _default_model_name(settings: Settings, profile: ResolvedModelProfile) -> str:
    normalized_purpose = (profile.purpose or "").strip().lower()
    if normalized_purpose == "chat_quality":
        return settings.default_chat_quality_model
    if normalized_purpose == "report_writer":
        return settings.default_report_model
    return settings.default_chat_model


def build_generation_client(settings: Settings, *, profile: ResolvedModelProfile) -> GenerationClient:
    provider = (profile.provider or "").strip().lower()

    if provider in {"openai", "openai_compatible"} and profile.base_url and profile.api_key:
        return OpenAIGenerationClient(
            model=profile.model_name or _default_model_name(settings, profile),
            api_base=profile.base_url,
            api_key=profile.api_key,
            temperature=profile.temperature,
            max_tokens=profile.max_tokens,
        )
    if provider == "ollama":
        return OllamaGenerationClient(
            model=profile.model_name or settings.ollama_model,
            base_url=profile.base_url or settings.ollama_base_url,
            temperature=profile.temperature,
            max_tokens=profile.max_tokens,
        )
    return FallbackGenerationClient()


def describe_generation_backend(settings: Settings, profile: ResolvedModelProfile | None = None) -> dict[str, str]:
    """Expose lightweight readiness without making outbound network calls.

    Health checks should tell operators which generation path will be used and
    whether required credentials are present, but they should not block on
    calling remote model providers.
    """

    resolved = profile
    backend = (resolved.provider if resolved else settings.generator_backend) or settings.generator_backend
    if backend in {"auto", "openai", "openai_compatible"}:
        api_base = resolved.base_url if resolved else settings.openai_api_base
        api_key = resolved.api_key if resolved else settings.openai_api_key
        if api_base and api_key:
            return {
                "status": "ok",
                "backend": "openai",
                "message": "OpenAI-compatible generation is configured.",
            }
        if backend in {"openai", "openai_compatible"}:
            return {
                "status": "degraded",
                "backend": "openai",
                "message": "OPENAI_API_BASE or OPENAI_API_KEY is missing.",
            }
    if backend == "ollama":
        return {
            "status": "ok",
            "backend": "ollama",
            "message": f"Ollama generation is configured for {settings.ollama_base_url}.",
        }
    return {
        "status": "ok",
        "backend": "fallback",
        "message": "Fallback generator is available for local development.",
    }
