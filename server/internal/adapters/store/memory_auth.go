package store

import (
	"context"
	"strings"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func authIdentityKey(provider domain.AuthProvider, subject string) string {
	return string(provider) + ":" + strings.TrimSpace(subject)
}

func (s *MemoryStore) UpsertUser(_ context.Context, user domain.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.usersByID[user.ID]; ok {
		if existing.Username != "" {
			delete(s.usersByUsername, existing.Username)
		}
	}
	s.usersByID[user.ID] = user
	s.usersByUsername[user.Username] = user.ID
	return nil
}

func (s *MemoryStore) GetUserByUsername(_ context.Context, username string) (domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	userID, ok := s.usersByUsername[username]
	if !ok {
		return domain.User{}, ErrNotFound
	}
	return s.usersByID[userID], nil
}

func (s *MemoryStore) GetUserByID(_ context.Context, userID string) (domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.usersByID[userID]
	if !ok {
		return domain.User{}, ErrNotFound
	}
	return user, nil
}

func (s *MemoryStore) GetUserByAuthIdentity(_ context.Context, provider domain.AuthProvider, subject string) (domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	identityID, ok := s.authIdentityLookup[authIdentityKey(provider, subject)]
	if !ok {
		return domain.User{}, ErrNotFound
	}
	identity, ok := s.authIdentities[identityID]
	if !ok {
		return domain.User{}, ErrNotFound
	}
	user, ok := s.usersByID[identity.UserID]
	if !ok {
		return domain.User{}, ErrNotFound
	}
	return user, nil
}

func (s *MemoryStore) UpsertAuthIdentity(_ context.Context, identity domain.AuthIdentity) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := authIdentityKey(identity.Provider, identity.ProviderSubject)
	if existingID, ok := s.authIdentityLookup[key]; ok && existingID != identity.ID {
		return ErrConflict
	}
	if existing, ok := s.authIdentities[identity.ID]; ok {
		delete(s.authIdentityLookup, authIdentityKey(existing.Provider, existing.ProviderSubject))
	}
	s.authIdentities[identity.ID] = identity
	s.authIdentityLookup[key] = identity.ID
	return nil
}

func (s *MemoryStore) CreateSession(_ context.Context, session domain.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[session.ID] = session
	s.sessionsByToken[session.RefreshToken] = session.ID
	s.sessionsByAccess[session.AccessToken] = session.ID
	return nil
}

func (s *MemoryStore) GetSessionByAccessToken(_ context.Context, accessToken string) (domain.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sessionID, ok := s.sessionsByAccess[accessToken]
	if !ok {
		return domain.Session{}, ErrNotFound
	}
	return s.sessions[sessionID], nil
}

func (s *MemoryStore) GetSessionByRefreshToken(_ context.Context, refreshToken string) (domain.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sessionID, ok := s.sessionsByToken[refreshToken]
	if !ok {
		return domain.Session{}, ErrNotFound
	}
	return s.sessions[sessionID], nil
}

func (s *MemoryStore) RevokeSession(_ context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[sessionID]
	if !ok {
		return ErrNotFound
	}
	now := time.Now().UTC()
	session.RevokedAt = &now
	s.sessions[sessionID] = session
	return nil
}

func (s *MemoryStore) ConsumeSession(_ context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[sessionID]
	if !ok || session.RevokedAt != nil || session.ExpiresAt.Before(time.Now().UTC()) {
		return ErrNotFound
	}
	now := time.Now().UTC()
	session.RevokedAt = &now
	s.sessions[sessionID] = session
	return nil
}
