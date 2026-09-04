package auth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

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
	claims, err := s.parseToken(refreshToken, "refresh")
	if err != nil || claims["sub"] != session.UserID {
		return TokenPair{}, ErrInvalidToken
	}
	user, err := s.userStore.GetUserByID(ctx, session.UserID)
	if err != nil {
		return TokenPair{}, ErrInvalidToken
	}
	if err := s.sessionStore.ConsumeSession(ctx, session.ID); err != nil {
		return TokenPair{}, ErrInvalidToken
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
	claims, err := s.parseToken(accessToken, "access")
	if err != nil {
		return domain.User{}, ErrInvalidToken
	}
	userID, _ := claims["sub"].(string)
	if userID == "" {
		return domain.User{}, ErrInvalidToken
	}
	session, err := s.sessionStore.GetSessionByAccessToken(context.Background(), accessToken)
	if err != nil || session.UserID != userID || session.RevokedAt != nil || session.ExpiresAt.Before(time.Now().UTC()) {
		return domain.User{}, ErrInvalidToken
	}
	return s.userStore.GetUserByID(context.Background(), userID)
}

func (s *Service) issueSession(ctx context.Context, user domain.User) (TokenPair, error) {
	now := time.Now().UTC()
	accessExpiresAt := now.Add(s.cfg.AccessTTL)
	refreshExpiresAt := now.Add(s.cfg.RefreshTTL)
	if err := s.chatModelStore.EnsureUserChatModelDefaults(ctx, user.ID); err != nil {
		return TokenPair{}, err
	}
	accessToken, err := s.signToken(user.ID, accessExpiresAt, "access")
	if err != nil {
		return TokenPair{}, err
	}
	refreshToken, err := s.signToken(user.ID, refreshExpiresAt, "refresh")
	if err != nil {
		return TokenPair{}, err
	}
	session := domain.Session{
		ID:           uuid.NewString(),
		UserID:       user.ID,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    refreshExpiresAt,
		CreatedAt:    now,
	}
	if err := s.sessionStore.CreateSession(ctx, session); err != nil {
		return TokenPair{}, err
	}
	return TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		AccessTTL:    s.cfg.AccessTTL,
		RefreshTTL:   s.cfg.RefreshTTL,
		User:         user,
	}, nil
}

func (s *Service) signToken(userID string, expiresAt time.Time, tokenUse string) (string, error) {
	claims := jwt.MapClaims{
		"sub":       userID,
		"iat":       time.Now().UTC().Unix(),
		"exp":       expiresAt.Unix(),
		"jti":       uuid.NewString(),
		"token_use": tokenUse,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.cfg.JWTSecret))
}

func hashPassword(raw string) string {
	hash, err := bcrypt.GenerateFromPassword(sha256Digest(raw), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	return string(hash)
}

func verifyPassword(raw, encoded string) bool {
	if strings.HasPrefix(encoded, "$2") {
		return bcrypt.CompareHashAndPassword([]byte(encoded), sha256Digest(raw)) == nil
	}
	legacy, err := hex.DecodeString(encoded)
	if err != nil || len(legacy) != 32 {
		return false
	}
	digest := sha256Digest(raw)
	return subtle.ConstantTimeCompare(legacy, digest) == 1
}

func sha256Digest(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}

func (s *Service) parseToken(raw, expectedUse string) (jwt.MapClaims, error) {
	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, ErrInvalidToken
		}
		return []byte(s.cfg.JWTSecret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return nil, err
	}
	if use, _ := claims["token_use"].(string); use != expectedUse {
		return nil, errors.New("unexpected token use")
	}
	return claims, nil
}
