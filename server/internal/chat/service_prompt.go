package chat

import (
	"fmt"
	"strings"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func systemPrompt(skill Skill, customPrompt string) string {
	style := map[Skill]string{
		SkillAnswer:  "Answer the user's request naturally in the same language as the user. If Yuque snippets are attached, use them as high-priority context.",
		SkillSummary: "Summarize the attached Yuque context or the user's topic into concise bullets and preserve the key facts.",
		SkillActions: "Turn the attached Yuque context or the user's topic into a practical action plan with explicit next steps.",
	}[skill]
	if style == "" {
		style = "Answer the user's request naturally."
	}

	parts := []string{
		"You are Knowvia, a general AI assistant with optional Yuque knowledge access.",
		"When Yuque snippets are provided, ground important claims in them and cite them when useful.",
		"When no Yuque snippets are provided, continue as a normal helpful AI assistant.",
		"If attached internal context is insufficient for a company-specific question, say what is missing instead of making it up.",
		style,
		"When useful, cite snippets with [1], [2], [3].",
	}
	if strings.TrimSpace(customPrompt) != "" {
		parts = append(parts, "Additional skill instructions: "+strings.TrimSpace(customPrompt))
	}

	return strings.Join(parts, " ")
}

func userPrompt(message string, skill Skill, evidences []domain.Evidence) string {
	lines := []string{
		fmt.Sprintf("User message: %s", strings.TrimSpace(message)),
		fmt.Sprintf("Skill: %s", skill),
	}

	if len(evidences) == 0 {
		lines = append(lines,
			"Yuque snippets:",
			"- No Yuque snippets are attached for this turn. Answer normally unless the request requires internal knowledge.",
		)
	} else {
		lines = append(lines, "Yuque snippets:")
		for idx, evidence := range evidences {
			line := fmt.Sprintf("[%d] %s", idx+1, evidence.Title)
			if evidence.Repo != "" {
				line += fmt.Sprintf(" | repo=%s", evidence.Repo)
			}
			if evidence.URL != "" {
				line += fmt.Sprintf(" | url=%s", evidence.URL)
			}
			if evidence.Snippet != "" {
				line += fmt.Sprintf("\n%s", evidence.Snippet)
			}
			lines = append(lines, line)
		}
	}

	return strings.Join(lines, "\n")
}

func fallbackAnswer(message string, skill Skill, evidences []domain.Evidence) string {
	if len(evidences) == 0 {
		return strings.Join([]string{
			"当前环境还没有配置可用的对话模型，所以我暂时不能像普通助手那样直接回答这个问题。",
			"如果你希望我回答日常问题，请先配置聊天模型；如果你希望我基于内部资料回答，也可以先挂载并同步语雀知识库。",
			fmt.Sprintf("当前输入：%s", strings.TrimSpace(message)),
		}, "\n\n")
	}

	lines := []string{}
	switch skill {
	case SkillSummary:
		lines = append(lines, "基于已命中的语雀内容，整理摘要如下：")
	case SkillActions:
		lines = append(lines, "基于已命中的语雀内容，建议按以下步骤推进：")
	default:
		lines = append(lines, "我根据语雀知识库中命中的内容整理了以下回答：")
	}

	for idx, evidence := range evidences {
		lines = append(lines, fmt.Sprintf("%d. %s：%s", idx+1, evidence.Title, evidence.Snippet))
	}

	if skill == SkillActions {
		lines = append(lines, "建议先核对上面的知识库依据，再执行具体动作。")
	}

	return strings.Join(lines, "\n")
}

func chunkText(text string, chunkSize int) []string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) == 0 {
		return []string{}
	}
	if chunkSize <= 0 {
		chunkSize = len(runes)
	}
	chunks := make([]string, 0, len(runes)/chunkSize+1)
	for start := 0; start < len(runes); start += chunkSize {
		end := start + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[start:end]))
	}
	return chunks
}
