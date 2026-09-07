package chat

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/persistence"
)

var ErrChatSessionNotFound = errors.New("chat: session not found")

type UpdateSessionInput struct {
	Title  *string
	Pinned *bool
}

func (s *Service) CreateSession(ctx context.Context, userID, title string) (domain.ChatSession, error) {
	if s.sessionStore == nil {
		return domain.ChatSession{}, errors.New("chat session store unavailable")
	}
	now := time.Now().UTC()
	session := domain.ChatSession{
		ID:        uuid.NewString(),
		UserID:    userID,
		Title:     fallbackSessionTitle(strings.TrimSpace(title), ""),
		Kind:      domain.ChatSessionKindChat,
		Pinned:    false,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.sessionStore.CreateChatSession(ctx, session); err != nil {
		return domain.ChatSession{}, err
	}
	return session, nil
}

func (s *Service) ListSessions(ctx context.Context, userID string) ([]domain.ChatSession, error) {
	if s.sessionStore == nil {
		return nil, errors.New("chat session store unavailable")
	}
	return s.sessionStore.ListChatSessions(ctx, userID)
}

func (s *Service) UpdateSession(ctx context.Context, userID, sessionID string, input UpdateSessionInput) (domain.ChatSession, error) {
	if s.sessionStore == nil {
		return domain.ChatSession{}, errors.New("chat session store unavailable")
	}
	session, err := s.sessionStore.GetChatSession(ctx, userID, sessionID)
	if err != nil {
		if errors.Is(err, persistence.ErrNotFound) {
			return domain.ChatSession{}, ErrChatSessionNotFound
		}
		return domain.ChatSession{}, err
	}
	if input.Title != nil {
		session.Title = fallbackSessionTitle(strings.TrimSpace(*input.Title), "")
	}
	if input.Pinned != nil {
		session.Pinned = *input.Pinned
	}
	session.UpdatedAt = time.Now().UTC()
	if err := s.sessionStore.UpdateChatSession(ctx, session); err != nil {
		return domain.ChatSession{}, err
	}
	return session, nil
}

func (s *Service) DeleteSession(ctx context.Context, userID, sessionID string) error {
	if s.sessionStore == nil {
		return errors.New("chat session store unavailable")
	}
	if err := s.sessionStore.DeleteChatSession(ctx, userID, sessionID); err != nil {
		if errors.Is(err, persistence.ErrNotFound) {
			return ErrChatSessionNotFound
		}
		return err
	}
	return nil
}

func (s *Service) ListMessages(ctx context.Context, userID, sessionID string) ([]MessageView, error) {
	if s.sessionStore == nil {
		return nil, errors.New("chat session store unavailable")
	}
	messages, err := s.sessionStore.ListChatMessages(ctx, userID, sessionID)
	if err != nil {
		if errors.Is(err, persistence.ErrNotFound) {
			return nil, ErrChatSessionNotFound
		}
		return nil, err
	}

	items := make([]MessageView, 0, len(messages))
	for _, message := range messages {
		sources, err := s.sessionStore.ListChatMessageSources(ctx, message.ID)
		if err != nil {
			return nil, err
		}
		items = append(items, MessageView{
			ID:           message.ID,
			SessionID:    message.SessionID,
			Role:         message.Role,
			Content:      message.Content,
			Skill:        message.Skill,
			UseKnowledge: message.UseKnowledge,
			CreatedAt:    message.CreatedAt,
			CompletedAt:  message.CompletedAt,
			Sources:      toMessageSourceViews(sources),
			Usage:        cloneChatUsage(message.Usage),
		})
	}
	return items, nil
}

func cloneChatUsage(usage *domain.ChatUsage) *domain.ChatUsage {
	if usage == nil {
		return nil
	}
	next := *usage
	if next.IsZero() {
		return nil
	}
	return &next
}

func toMessageSourceViews(sources []domain.ChatMessageSource) []MessageSourceView {
	items := make([]MessageSourceView, 0, len(sources))
	for _, source := range sources {
		items = append(items, MessageSourceView{
			Provider:     source.Provider,
			Title:        source.Title,
			Repo:         source.Repo,
			URL:          source.URL,
			Snippet:      source.Snippet,
			MatchedLines: append([]string(nil), source.MatchedLines...),
			Score:        source.Score,
		})
	}
	return items
}
