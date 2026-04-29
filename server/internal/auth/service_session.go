package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func (s *Service) Refresh(ctx context.Context, refreshToken string) (TokenPair, error) {
	session, err := s.sessionStore.GetSessionByRefreshToken(ctx, refreshToken)
	if err != nil {
		return TokenPair{}, ErrInvalidToken
	}
	if session.RevokedAt != nil || session.ExpiresAt.Before(time.Now().UTC()) {
		return TokenPair{}, ErrInvalidToken
	}
	user, err := s.userStore.GetUserByID(ctx, session.UserID)
	if err != nil {
		return TokenPair{}, ErrInvalidToken
	}
	if err := s.sessionStore.RevokeSession(ctx, session.ID); err != nil {
		return TokenPair{}, err
	}
	return s.issueSession(ctx, user)
}

func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	session, err := s.sessionStore.GetSessionByRefreshToken(ctx, refreshToken)
	if err != nil {
		return ErrInvalidToken
	}
	return s.sessionStore.RevokeSession(ctx, session.ID)
}

func (s *Service) Authenticate(accessToken string) (domain.User, error) {
	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(accessToken, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(s.cfg.JWTSecret), nil
	})
	if err != nil {
		return domain.User{}, ErrInvalidToken
	}
	userID, _ := claims["sub"].(string)
	if userID == "" {
		return domain.User{}, ErrInvalidToken
	}
	return s.userStore.GetUserByID(context.Background(), userID)
}

func (s *Service) issueSession(ctx context.Context, user domain.User) (TokenPair, error) {
	now := time.Now().UTC()
	if err := s.chatModelStore.EnsureUserChatModelDefaults(ctx, user.ID); err != nil {
		return TokenPair{}, err
	}
	accessToken, err := s.signToken(user.ID, nil)
	if err != nil {
		return TokenPair{}, err
	}
	refreshToken, err := s.signToken(user.ID, nil)
	if err != nil {
		return TokenPair{}, err
	}
	session := domain.Session{
		ID:           uuid.NewString(),
		UserID:       user.ID,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    nonExpiringSessionExpiresAt,
		CreatedAt:    now,
	}
	if err := s.sessionStore.CreateSession(ctx, session); err != nil {
		return TokenPair{}, err
	}
	return TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		AccessTTL:    0,
		RefreshTTL:   0,
		User:         user,
	}, nil
}

func (s *Service) signToken(userID string, expiresAt *time.Time) (string, error) {
	claims := jwt.MapClaims{
		"sub": userID,
		"iat": time.Now().UTC().Unix(),
	}
	if expiresAt != nil {
		claims["exp"] = expiresAt.Unix()
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.cfg.JWTSecret))
}

func hashPassword(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
