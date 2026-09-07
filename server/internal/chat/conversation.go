package chat

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/persistence"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skillruntime"
)

type StreamConversationRequest struct {
	UserID        string
	SessionID     string
	Message       string
	ConnectionIDs []string
	Runtime       domain.ChatRuntimeConfig
	SkillID       string
	Skill         Skill
	CustomPrompt  string
	UseKnowledge  bool
	EnableSearch  bool
	SelectedSkill *domain.Skill
}

type StreamConversationHooks struct {
	OnSession   func(domain.ChatSession) error
	OnRetrieval func([]domain.Evidence) error
	OnChunk     func(string) error
	OnUsage     func(domain.ChatUsage) error
}

type StreamConversationResult struct {
	Session          domain.ChatSession
	UserMessage      domain.ChatMessage
	AssistantMessage domain.ChatMessage
	SkillSnapshot    *domain.SkillRuntimeSnapshot
	Answer           string
	Sources          []domain.Evidence
	Usage            *domain.ChatUsage
}

func (s *Service) StreamConversation(
	ctx context.Context,
	req StreamConversationRequest,
	hooks StreamConversationHooks,
) (StreamConversationResult, error) {
	if s.conversationStore == nil {
		return StreamConversationResult{}, errors.New("chat conversation store unavailable")
	}

	session, err := ensureChatSession(ctx, s.conversationStore, req.UserID, req.SessionID, req.Message)
	if err != nil {
		return StreamConversationResult{}, err
	}
	if hooks.OnSession != nil {
		if err := hooks.OnSession(session); err != nil {
			return StreamConversationResult{}, err
		}
	}

	now := time.Now().UTC()
	userMessage := domain.ChatMessage{
		ID:           uuid.NewString(),
		SessionID:    session.ID,
		UserID:       req.UserID,
		Role:         domain.ChatRoleUser,
		Content:      strings.TrimSpace(req.Message),
		Skill:        string(req.Skill),
		UseKnowledge: req.UseKnowledge,
		CreatedAt:    now,
		CompletedAt:  &now,
	}
	if err := s.conversationStore.SaveChatMessage(ctx, userMessage); err != nil {
		return StreamConversationResult{}, err
	}

	assistantMessage := domain.ChatMessage{
		ID:           uuid.NewString(),
		SessionID:    session.ID,
		UserID:       req.UserID,
		Role:         domain.ChatRoleAssistant,
		Skill:        string(req.Skill),
		UseKnowledge: req.UseKnowledge,
		CreatedAt:    time.Now().UTC(),
	}
	if err := s.conversationStore.SaveChatMessage(ctx, assistantMessage); err != nil {
		return StreamConversationResult{}, err
	}

	var skillSnapshot *domain.SkillRuntimeSnapshot
	if req.SelectedSkill != nil {
		snapshot, err := skillruntime.BuildSnapshot(
			*req.SelectedSkill,
			domain.SkillRuntimeScopeChat,
			assistantMessage.ID,
			uuid.NewString(),
			time.Now().UTC(),
		)
		if err != nil {
			return StreamConversationResult{}, err
		}
		if err := s.conversationStore.CreateSkillRuntimeSnapshot(ctx, snapshot); err != nil {
			return StreamConversationResult{}, err
		}
		skillSnapshot = &snapshot
	}

	if session.Kind == domain.ChatSessionKindTask && strings.TrimSpace(session.RunID) != "" && s.taskRunner != nil {
		taskResult, taskErr := s.taskRunner.RunTaskConversation(
			ctx,
			TaskConversationRequest{
				UserID:    req.UserID,
				RunID:     session.RunID,
				SessionID: session.ID,
				Message:   req.Message,
				Runtime:   req.Runtime,
			},
			hooks.OnChunk,
		)
		if taskResult.Handled || taskErr != nil {
			finishedAt := time.Now().UTC()
			answer := strings.TrimSpace(taskResult.Answer)
			if taskErr != nil {
				answer = taskErr.Error()
			}
			assistantMessage.Content = answer
			assistantMessage.Usage = taskResult.Usage
			assistantMessage.CompletedAt = &finishedAt
			if err := s.conversationStore.SaveChatMessage(ctx, assistantMessage); err != nil {
				return StreamConversationResult{}, err
			}
			session.LastMessageAt = &finishedAt
			session.UpdatedAt = finishedAt
			if err := s.conversationStore.UpdateChatSession(ctx, session); err != nil {
				return StreamConversationResult{}, err
			}
			result := StreamConversationResult{
				Session:          session,
				UserMessage:      userMessage,
				AssistantMessage: assistantMessage,
				SkillSnapshot:    skillSnapshot,
				Answer:           answer,
				Usage:            taskResult.Usage,
			}
			if taskErr != nil {
				return result, taskErr
			}
			return result, nil
		}
	}

	sources := []domain.Evidence{}
	answer := ""
	var usage *domain.ChatUsage
	useForward := s.CanForward(req.UseKnowledge)
	if useForward {
		completion, forwardedSources, streamErr := s.StreamForwarded(
			ctx,
			ForwardRequest{
				UserID:        req.UserID,
				Message:       req.Message,
				ConnectionIDs: req.ConnectionIDs,
				Runtime:       req.Runtime,
				MessageID:     assistantMessage.ID,
				SkillID:       req.SkillID,
				Skill:         req.Skill,
				CustomPrompt:  req.CustomPrompt,
				SkillSnapshot: skillSnapshot,
				EnableSearch:  req.EnableSearch,
			},
			func(items []domain.Evidence) error {
				sources = append([]domain.Evidence(nil), items...)
				if hooks.OnRetrieval != nil {
					return hooks.OnRetrieval(sources)
				}
				return nil
			},
			hooks.OnChunk,
		)
		err = streamErr
		answer = completion.Answer
		usage = completion.Usage
		sources = forwardedSources
	} else {
		if req.UseKnowledge {
			sources, err = s.Retrieve(ctx, req.UserID, req.Message, req.ConnectionIDs)
			if err != nil {
				return StreamConversationResult{}, err
			}
		}
		if hooks.OnRetrieval != nil {
			if err := hooks.OnRetrieval(sources); err != nil {
				return StreamConversationResult{}, err
			}
		}
		completion, streamErr := s.Stream(ctx, req.Runtime, req.Message, req.Skill, req.CustomPrompt, sources, hooks.OnChunk)
		err = streamErr
		answer = completion.Answer
		usage = completion.Usage
	}
	if err != nil {
		finishedAt := time.Now().UTC()
		assistantMessage.Content = err.Error()
		assistantMessage.CompletedAt = &finishedAt
		_ = s.conversationStore.SaveChatMessage(ctx, assistantMessage)
		return StreamConversationResult{
			Session:          session,
			UserMessage:      userMessage,
			AssistantMessage: assistantMessage,
			SkillSnapshot:    skillSnapshot,
			Sources:          sources,
			Usage:            usage,
		}, err
	}
	if usage != nil && !usage.IsZero() && hooks.OnUsage != nil {
		if err := hooks.OnUsage(*usage); err != nil {
			return StreamConversationResult{}, err
		}
	}

	if len(sources) > 0 {
		if err := saveChatSources(ctx, s.conversationStore, assistantMessage.ID, sources); err != nil {
			return StreamConversationResult{}, err
		}
	}
	if useForward && len(sources) == 0 && hooks.OnRetrieval != nil {
		if err := hooks.OnRetrieval([]domain.Evidence{}); err != nil {
			return StreamConversationResult{}, err
		}
	}

	finishedAt := time.Now().UTC()
	assistantMessage.Content = answer
	assistantMessage.Usage = usage
	assistantMessage.CompletedAt = &finishedAt
	if err := s.conversationStore.SaveChatMessage(ctx, assistantMessage); err != nil {
		return StreamConversationResult{}, err
	}
	session.LastMessageAt = &finishedAt
	session.UpdatedAt = finishedAt
	if err := s.conversationStore.UpdateChatSession(ctx, session); err != nil {
		return StreamConversationResult{}, err
	}

	return StreamConversationResult{
		Session:          session,
		UserMessage:      userMessage,
		AssistantMessage: assistantMessage,
		SkillSnapshot:    skillSnapshot,
		Answer:           answer,
		Sources:          sources,
		Usage:            usage,
	}, nil
}

func ensureChatSession(ctx context.Context, st ConversationStore, userID, sessionID, firstMessage string) (domain.ChatSession, error) {
	if strings.TrimSpace(sessionID) != "" {
		session, err := st.GetChatSession(ctx, userID, sessionID)
		if err != nil {
			if errors.Is(err, persistence.ErrNotFound) {
				return domain.ChatSession{}, errors.New("chat session not found")
			}
			return domain.ChatSession{}, err
		}
		return session, nil
	}

	now := time.Now().UTC()
	session := domain.ChatSession{
		ID:        uuid.NewString(),
		UserID:    userID,
		Title:     fallbackSessionTitle("", firstMessage),
		Kind:      domain.ChatSessionKindChat,
		Pinned:    false,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := st.CreateChatSession(ctx, session); err != nil {
		return domain.ChatSession{}, err
	}
	return session, nil
}

func saveChatSources(ctx context.Context, st ConversationStore, messageID string, sources []domain.Evidence) error {
	records := make([]domain.ChatMessageSource, 0, len(sources))
	for _, source := range sources {
		records = append(records, domain.ChatMessageSource{
			ID:           uuid.NewString(),
			MessageID:    messageID,
			Provider:     source.Provider,
			ConnectionID: source.ConnectionID,
			DocumentID:   source.DocumentID,
			ChunkID:      source.ChunkID,
			Title:        source.Title,
			Repo:         source.Repo,
			URL:          source.URL,
			Snippet:      source.Snippet,
			MatchedLines: append([]string(nil), source.MatchedLines...),
			Score:        source.Score,
			CreatedAt:    time.Now().UTC(),
		})
	}
	return st.SaveChatMessageSources(ctx, messageID, records)
}

func fallbackSessionTitle(title, firstMessage string) string {
	if strings.TrimSpace(title) != "" {
		return title
	}
	if strings.TrimSpace(firstMessage) != "" {
		runes := []rune(strings.TrimSpace(firstMessage))
		if len(runes) > 24 {
			return string(runes[:24]) + "..."
		}
		return string(runes)
	}
	return "New chat"
}
