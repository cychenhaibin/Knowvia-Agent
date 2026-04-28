package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/provider"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/tools"
)

type Skill string

const (
	SkillAnswer  Skill = "answer"
	SkillSummary Skill = "summary"
	SkillActions Skill = "actions"
)

type Service struct {
	knowledgeTool tools.KnowledgeSearcher
	llm           provider.ChatClient
	forward       provider.ForwardChatClient
}

type ForwardRequest struct {
	UserID        string
	Message       string
	ConnectionIDs []string
	Runtime       domain.ChatRuntimeConfig
	MessageID     string
	TraceID       string
	SkillID       string
	Skill         Skill
	CustomPrompt  string
	SkillSnapshot *domain.SkillRuntimeSnapshot
}

type streamingClient interface {
	Stream(
		ctx context.Context,
		systemPrompt string,
		userPrompt string,
		onDelta func(string) error,
	) (string, error)
}

func NewService(
	knowledgeTool tools.KnowledgeSearcher,
	llm provider.ChatClient,
	forward provider.ForwardChatClient,
) *Service {
	return &Service{
		knowledgeTool: knowledgeTool,
		llm:           llm,
		forward:       forward,
	}
}

func (s *Service) CanForward(
	useKnowledge bool,
) bool {
	return s.forward != nil && useKnowledge
}

func NormalizeSkill(raw string) Skill {
	switch Skill(strings.ToLower(strings.TrimSpace(raw))) {
	case SkillSummary:
		return SkillSummary
	case SkillActions:
		return SkillActions
	default:
		return SkillAnswer
	}
}

func (s *Service) Retrieve(
	ctx context.Context,
	userID string,
	message string,
	connectionIDs []string,
) ([]domain.Evidence, error) {
	if s.knowledgeTool == nil {
		return []domain.Evidence{}, nil
	}
	return s.knowledgeTool.Search(ctx, userID, connectionIDs, strings.TrimSpace(message), 6, nil)
}

func (s *Service) Complete(
	ctx context.Context,
	runtime domain.ChatRuntimeConfig,
	message string,
	skill Skill,
	customPrompt string,
	evidences []domain.Evidence,
) (string, error) {
	if s.llm != nil {
		system := systemPrompt(skill, customPrompt)
		user := userPrompt(message, skill, evidences)
		answer := ""
		err := error(nil)
		if dynamic, ok := s.llm.(provider.ModelSelectableChatClient); ok {
			answer, err = dynamic.CompleteWithConfig(ctx, system, user, runtime)
			if err != nil && strings.TrimSpace(runtime.ModelName) != "" {
				return "", err
			}
		} else {
			answer, err = s.llm.Complete(ctx, system, user)
		}
		if err == nil && strings.TrimSpace(answer) != "" {
			return strings.TrimSpace(answer), nil
		}
	}
	return fallbackAnswer(message, skill, evidences), nil
}

func (s *Service) Stream(
	ctx context.Context,
	runtime domain.ChatRuntimeConfig,
	message string,
	skill Skill,
	customPrompt string,
	evidences []domain.Evidence,
	onChunk func(string) error,
) (string, error) {
	system := systemPrompt(skill, customPrompt)
	user := userPrompt(message, skill, evidences)

	if dynamic, ok := s.llm.(provider.ModelSelectableChatClient); ok {
		answer, err := dynamic.StreamWithConfig(ctx, system, user, runtime, onChunk)
		if err == nil && strings.TrimSpace(answer) != "" {
			return strings.TrimSpace(answer), nil
		}
		if err != nil && strings.TrimSpace(runtime.ModelName) != "" {
			return "", err
		}
	}

	if streamer, ok := s.llm.(streamingClient); ok {
		answer, err := streamer.Stream(ctx, system, user, onChunk)
		if err == nil && strings.TrimSpace(answer) != "" {
			return strings.TrimSpace(answer), nil
		}
	}

	answer, err := s.Complete(ctx, runtime, message, skill, customPrompt, evidences)
	if err != nil {
		return "", err
	}

	if onChunk != nil {
		for _, chunk := range chunkText(answer, 48) {
			if err := onChunk(chunk); err != nil {
				return "", err
			}
		}
	}

	return answer, nil
}

func (s *Service) StreamForwarded(
	ctx context.Context,
	req ForwardRequest,
	onRetrieval func([]domain.Evidence) error,
	onChunk func(string) error,
) (string, []domain.Evidence, error) {
	if s.forward == nil {
		return "", nil, errors.New("forward chat client unavailable")
	}

	result, err := s.forward.StreamKnowledgeChat(
		ctx,
		provider.ForwardedChatRequest{
			UserID:        req.UserID,
			Message:       strings.TrimSpace(req.Message),
			ConnectionIDs: append([]string(nil), req.ConnectionIDs...),
			ChatModel:     strings.TrimSpace(req.Runtime.ModelName),
			ChatAPIBase:   strings.TrimSpace(req.Runtime.BaseURL),
			ChatAPIKey:    req.Runtime.APIKey,
			SkillID:       strings.TrimSpace(req.SkillID),
			SkillPrompt:   strings.TrimSpace(req.CustomPrompt),
			Mode:          string(req.Skill),
			SkillSnapshot: forwardedSkillSnapshot(req.SkillSnapshot),
			ModelProfile:  provider.InferForwardedModelProfile(req.Runtime, "chat_fast"),
			Trace: &provider.ForwardedTraceContext{
				TraceID:   strings.TrimSpace(req.TraceID),
				MessageID: strings.TrimSpace(req.MessageID),
				SkillID:   strings.TrimSpace(req.SkillID),
			},
		},
		func(items []provider.ForwardedSource) error {
			sources := forwardedSourcesToEvidence(items)
			if onRetrieval != nil {
				return onRetrieval(sources)
			}
			return nil
		},
		onChunk,
	)
	if err != nil {
		return "", nil, err
	}

	sources := forwardedSourcesToEvidence(result.Sources)
	return strings.TrimSpace(result.Answer), sources, nil
}

func forwardedSkillSnapshot(snapshot *domain.SkillRuntimeSnapshot) *provider.ForwardedSkillSnapshot {
	if snapshot == nil {
		return nil
	}
	runtimeSpec := map[string]any{}
	if strings.TrimSpace(snapshot.RuntimeSpecJSON) != "" {
		_ = json.Unmarshal([]byte(snapshot.RuntimeSpecJSON), &runtimeSpec)
	}
	return &provider.ForwardedSkillSnapshot{
		SnapshotID:     snapshot.ID,
		InstallationID: snapshot.InstallationID,
		DefinitionID:   snapshot.DefinitionID,
		RevisionID:     snapshot.RevisionID,
		Kind:           string(snapshot.Kind),
		Title:          snapshot.Title,
		Description:    snapshot.Description,
		Mode:           snapshot.Mode,
		Prompt:         snapshot.Prompt,
		RuntimeSpec:    runtimeSpec,
	}
}

func forwardedSourcesToEvidence(items []provider.ForwardedSource) []domain.Evidence {
	sources := make([]domain.Evidence, 0, len(items))
	for _, source := range items {
		providerName := domain.ProviderWeb
		if strings.EqualFold(source.Type, "knowledge_base") {
			switch strings.ToLower(strings.TrimSpace(source.Provider)) {
			case string(domain.ProviderFeishu):
				providerName = domain.ProviderFeishu
			default:
				providerName = domain.ProviderYuque
			}
		}
		sources = append(sources, domain.Evidence{
			Provider:     providerName,
			ConnectionID: strings.TrimSpace(firstNonEmpty(source.ConnectionID, source.ScopeID)),
			DocumentID:   strings.TrimSpace(source.DocumentID),
			ChunkID:      strings.TrimSpace(source.ChunkID),
			Title:        strings.TrimSpace(source.Title),
			Repo:         strings.TrimSpace(source.Repo),
			URL:          strings.TrimSpace(source.URL),
			Snippet:      strings.TrimSpace(source.Snippet),
			MatchedLines: append([]string(nil), source.MatchedLines...),
			Score:        source.Score,
		})
	}
	return sources
}

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
