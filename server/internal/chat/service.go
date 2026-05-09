package chat

import (
	"context"
	"errors"
	"strings"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/provider"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/runtimepolicy"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/tools"
)

type Skill string

const (
	SkillAnswer  Skill = "answer"
	SkillSummary Skill = "summary"
	SkillActions Skill = "actions"
)

type Service struct {
	modelStore        ChatModelStore
	sessionStore      SessionStore
	conversationStore ConversationStore
	prepareStore      PrepareStreamStore
	knowledgeTool     tools.KnowledgeSearcher
	llm               provider.ChatClient
	forward           provider.ForwardChatClient
	taskRunner        TaskConversationRunner
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
	EnableSearch  bool
}

type ChatCompletion struct {
	Answer string
	Usage  *domain.ChatUsage
}

type TaskConversationRequest struct {
	UserID    string
	RunID     string
	SessionID string
	Message   string
	Runtime   domain.ChatRuntimeConfig
}

type TaskConversationResult struct {
	Handled bool
	Answer  string
	Usage   *domain.ChatUsage
}

type TaskConversationRunner interface {
	RunTaskConversation(
		ctx context.Context,
		req TaskConversationRequest,
		onChunk func(string) error,
	) (TaskConversationResult, error)
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
	deps ServiceDeps,
	knowledgeTool tools.KnowledgeSearcher,
	llm provider.ChatClient,
	forward provider.ForwardChatClient,
) *Service {
	return &Service{
		modelStore:        deps.Models,
		sessionStore:      deps.Sessions,
		conversationStore: deps.Conversations,
		prepareStore:      deps.Prepare,
		knowledgeTool:     knowledgeTool,
		llm:               llm,
		forward:           forward,
	}
}

func (s *Service) CanForward(
	useKnowledge bool,
) bool {
	return s.forward != nil && useKnowledge
}

func (s *Service) SetTaskConversationRunner(runner TaskConversationRunner) {
	s.taskRunner = runner
}

func NormalizeSkill(raw string) Skill {
	return Skill(runtimepolicy.NormalizeSkillMode(raw))
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
) (ChatCompletion, error) {
	system := systemPrompt(skill, customPrompt)
	user := userPrompt(message, skill, evidences)
	var usage *domain.ChatUsage

	if dynamic, ok := s.llm.(provider.ModelSelectableChatClient); ok {
		answer, err := dynamic.StreamWithConfig(ctx, system, user, runtime, onChunk, func(next domain.ChatUsage) error {
			usage = &next
			return nil
		})
		if err == nil && strings.TrimSpace(answer) != "" {
			return ChatCompletion{Answer: strings.TrimSpace(answer), Usage: usage}, nil
		}
		if err != nil && strings.TrimSpace(runtime.ModelName) != "" {
			return ChatCompletion{Usage: usage}, err
		}
	}

	if streamer, ok := s.llm.(streamingClient); ok {
		answer, err := streamer.Stream(ctx, system, user, onChunk)
		if err == nil && strings.TrimSpace(answer) != "" {
			return ChatCompletion{Answer: strings.TrimSpace(answer), Usage: usage}, nil
		}
	}

	answer, err := s.Complete(ctx, runtime, message, skill, customPrompt, evidences)
	if err != nil {
		return ChatCompletion{Usage: usage}, err
	}

	if onChunk != nil {
		for _, chunk := range chunkText(answer, 48) {
			if err := onChunk(chunk); err != nil {
				return ChatCompletion{Answer: strings.TrimSpace(answer), Usage: usage}, err
			}
		}
	}

	return ChatCompletion{Answer: answer, Usage: usage}, nil
}

func (s *Service) StreamForwarded(
	ctx context.Context,
	req ForwardRequest,
	onRetrieval func([]domain.Evidence) error,
	onChunk func(string) error,
) (ChatCompletion, []domain.Evidence, error) {
	if s.forward == nil {
		return ChatCompletion{}, nil, errors.New("forward chat client unavailable")
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
			EnableSearch:  req.EnableSearch,
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
		return ChatCompletion{}, nil, err
	}

	sources := forwardedSourcesToEvidence(result.Sources)
	return ChatCompletion{
		Answer: strings.TrimSpace(result.Answer),
		Usage:  forwardedUsageToDomain(result.Usage),
	}, sources, nil
}

func forwardedUsageToDomain(usage *provider.ForwardedUsage) *domain.ChatUsage {
	if usage == nil {
		return nil
	}
	next := domain.ChatUsage{
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
	}
	if next.IsZero() {
		return nil
	}
	return &next
}
