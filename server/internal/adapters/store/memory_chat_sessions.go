package store

import (
	"context"
	"sort"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func (s *MemoryStore) CreateChatSession(_ context.Context, session domain.ChatSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if session.Kind == "" {
		session.Kind = domain.ChatSessionKindChat
	}
	s.chatSessions[session.ID] = session
	return nil
}

func (s *MemoryStore) GetChatSession(_ context.Context, userID, sessionID string) (domain.ChatSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.chatSessions[sessionID]
	if !ok || session.UserID != userID {
		return domain.ChatSession{}, ErrNotFound
	}
	if session.Kind == "" {
		session.Kind = domain.ChatSessionKindChat
	}
	return session, nil
}

func (s *MemoryStore) UpdateChatSession(_ context.Context, session domain.ChatSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.chatSessions[session.ID]; !ok {
		return ErrNotFound
	}
	s.chatSessions[session.ID] = session
	return nil
}

func (s *MemoryStore) ListChatSessions(_ context.Context, userID string) ([]domain.ChatSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sessions := []domain.ChatSession{}
	for _, session := range s.chatSessions {
		if session.Kind == "" {
			session.Kind = domain.ChatSessionKindChat
		}
		if session.UserID == userID && session.Kind == domain.ChatSessionKindChat {
			sessions = append(sessions, session)
		}
	}
	sort.Slice(sessions, func(i, j int) bool {
		if sessions[i].Pinned != sessions[j].Pinned {
			return sessions[i].Pinned
		}
		left := sessions[i].UpdatedAt
		if sessions[i].LastMessageAt != nil {
			left = *sessions[i].LastMessageAt
		}
		right := sessions[j].UpdatedAt
		if sessions[j].LastMessageAt != nil {
			right = *sessions[j].LastMessageAt
		}
		return left.After(right)
	})
	return sessions, nil
}

func (s *MemoryStore) DeleteChatSession(_ context.Context, userID, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.chatSessions[sessionID]
	if !ok || session.UserID != userID {
		return ErrNotFound
	}
	messages := append([]domain.ChatMessage(nil), s.chatMessages[sessionID]...)
	delete(s.chatSessions, sessionID)
	delete(s.chatMessages, sessionID)
	for _, message := range messages {
		delete(s.messageSources, message.ID)
	}
	return nil
}

func (s *MemoryStore) SaveChatMessage(_ context.Context, message domain.ChatMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	messages := s.chatMessages[message.SessionID]
	for i := range messages {
		if messages[i].ID == message.ID {
			messages[i] = message
			s.chatMessages[message.SessionID] = messages
			return nil
		}
	}
	s.chatMessages[message.SessionID] = append(messages, message)
	return nil
}

func (s *MemoryStore) ListChatMessages(_ context.Context, userID, sessionID string) ([]domain.ChatMessage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.chatSessions[sessionID]
	if !ok || session.UserID != userID {
		return nil, ErrNotFound
	}
	messages := append([]domain.ChatMessage(nil), s.chatMessages[sessionID]...)
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].CreatedAt.Before(messages[j].CreatedAt)
	})
	return messages, nil
}

func (s *MemoryStore) SaveChatMessageSources(_ context.Context, messageID string, sources []domain.ChatMessageSource) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messageSources[messageID] = append([]domain.ChatMessageSource(nil), sources...)
	return nil
}

func (s *MemoryStore) ListChatMessageSources(_ context.Context, messageID string) ([]domain.ChatMessageSource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sources := append([]domain.ChatMessageSource(nil), s.messageSources[messageID]...)
	sort.Slice(sources, func(i, j int) bool {
		return sources[i].CreatedAt.Before(sources[j].CreatedAt)
	})
	return sources, nil
}
