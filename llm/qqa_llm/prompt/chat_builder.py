from __future__ import annotations

from qqa_llm.domain.models import ChatPrompt, RetrievedPassage
from qqa_llm.prompt.policy import CHAT_SYSTEM_POLICY


class ChatPromptBuilder:
    def build(self, *, question: str, mode: str, skill_prompt: str, passages: list[RetrievedPassage]) -> ChatPrompt:
        instructions = {
            "answer": "直接回答用户问题。",
            "summary": "提炼成清晰要点。",
            "actions": "输出下一步行动建议。",
        }.get(mode, "直接回答用户问题。")

        lines = [instructions]
        if skill_prompt:
            lines.append(f"额外技能约束：{skill_prompt}")
        lines.append(f"用户问题：{question}")
        if passages:
            lines.append("检索证据：")
            for index, passage in enumerate(passages, start=1):
                lines.append(f"[{index}] 标题: {passage.title} | 知识库: {passage.repo}")
                if passage.url:
                    lines.append(f"链接: {passage.url}")
                lines.append(f"内容: {passage.content}")
        else:
            lines.append("检索证据：当前没有命中可用知识片段。")

        return ChatPrompt(
            system_prompt=CHAT_SYSTEM_POLICY,
            user_prompt="\n".join(lines),
            mode=mode,
            question=question,
            skill_prompt=skill_prompt,
            passages=passages,
        )
