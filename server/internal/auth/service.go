package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/config"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/store"
)

var ErrInvalidCredentials = errors.New("invalid credentials")
var ErrInvalidToken = errors.New("invalid token")
var ErrGoogleAuthDisabled = errors.New("google auth is not configured")
var ErrInvalidGoogleToken = errors.New("invalid google id token")
var ErrGoogleKeysUnavailable = errors.New("google public keys are unavailable")
var ErrMicrosoftAuthDisabled = errors.New("microsoft auth is not configured")
var ErrInvalidMicrosoftToken = errors.New("invalid microsoft access token")
var ErrMicrosoftIdentityUnavailable = errors.New("microsoft identity service is unavailable")

type Service struct {
	store             store.Store
	cfg               config.Config
	googleVerifier    GoogleTokenVerifier
	microsoftVerifier MicrosoftTokenVerifier
}

type TokenPair struct {
	AccessToken  string        `json:"accessToken"`
	RefreshToken string        `json:"refreshToken"`
	AccessTTL    time.Duration `json:"accessTtl"`
	RefreshTTL   time.Duration `json:"refreshTtl"`
	User         domain.User   `json:"user"`
}

var nonExpiringSessionExpiresAt = time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)

func NewService(st store.Store, cfg config.Config) *Service {
	return NewServiceWithVerifiers(
		st,
		cfg,
		NewGoogleTokenVerifier(cfg.GoogleWebClientID),
		NewMicrosoftTokenVerifier(cfg.MicrosoftClientID, cfg.MicrosoftTenantID),
	)
}

func NewServiceWithGoogleVerifier(st store.Store, cfg config.Config, verifier GoogleTokenVerifier) *Service {
	return NewServiceWithVerifiers(
		st,
		cfg,
		verifier,
		NewMicrosoftTokenVerifier(cfg.MicrosoftClientID, cfg.MicrosoftTenantID),
	)
}

func NewServiceWithMicrosoftVerifier(st store.Store, cfg config.Config, verifier MicrosoftTokenVerifier) *Service {
	return NewServiceWithVerifiers(
		st,
		cfg,
		NewGoogleTokenVerifier(cfg.GoogleWebClientID),
		verifier,
	)
}

func NewServiceWithVerifiers(
	st store.Store,
	cfg config.Config,
	googleVerifier GoogleTokenVerifier,
	microsoftVerifier MicrosoftTokenVerifier,
) *Service {
	return &Service{
		store:             st,
		cfg:               cfg,
		googleVerifier:    googleVerifier,
		microsoftVerifier: microsoftVerifier,
	}
}

func (s *Service) SeedDevUsers(ctx context.Context) error {
	for _, devUser := range s.cfg.DevUsers {
		user := domain.User{
			ID:           uuid.NewString(),
			Username:     devUser.Username,
			DisplayName:  devUser.DisplayName,
			PasswordHash: hashPassword(devUser.Password),
			CreatedAt:    time.Now().UTC(),
		}
		if existing, err := s.store.GetUserByUsername(ctx, devUser.Username); err == nil {
			user.ID = existing.ID
			user.CreatedAt = existing.CreatedAt
		}
		if err := s.store.UpsertUser(ctx, user); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) Login(ctx context.Context, username, password string) (TokenPair, error) {
	user, err := s.store.GetUserByUsername(ctx, username)
	if err != nil {
		return TokenPair{}, ErrInvalidCredentials
	}
	if user.PasswordHash != hashPassword(password) {
		return TokenPair{}, ErrInvalidCredentials
	}
	return s.issueSession(ctx, user)
}

func (s *Service) LoginWithGoogle(ctx context.Context, idToken string) (TokenPair, error) {
	if s.googleVerifier == nil {
		return TokenPair{}, ErrGoogleAuthDisabled
	}
	identity, err := s.googleVerifier.VerifyIDToken(ctx, idToken)
	if err != nil {
		return TokenPair{}, err
	}
	if strings.TrimSpace(identity.Subject) == "" {
		return TokenPair{}, ErrInvalidGoogleToken
	}

	user, err := s.store.GetUserByAuthIdentity(ctx, domain.AuthProviderGoogle, identity.Subject)
	switch {
	case err == nil:
	case errors.Is(err, store.ErrNotFound):
		user, err = s.resolveOrCreateGoogleUser(ctx, identity)
		if err != nil {
			return TokenPair{}, err
		}
	default:
		return TokenPair{}, err
	}

	if identity.DisplayName != "" || identity.Email != "" || identity.AvatarURL != "" {
		updated := user
		if identity.DisplayName != "" {
			updated.DisplayName = identity.DisplayName
		}
		if identity.Email != "" {
			updated.Email = identity.Email
		}
		if identity.AvatarURL != "" {
			updated.AvatarURL = identity.AvatarURL
		}
		if err := s.store.UpsertUser(ctx, updated); err != nil {
			return TokenPair{}, err
		}
		user = updated
	}

	now := time.Now().UTC()
	authIdentity := domain.AuthIdentity{
		ID:              authIdentityID(domain.AuthProviderGoogle, identity.Subject),
		UserID:          user.ID,
		Provider:        domain.AuthProviderGoogle,
		ProviderSubject: identity.Subject,
		Email:           identity.Email,
		EmailVerified:   identity.EmailVerified,
		AvatarURL:       identity.AvatarURL,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.store.UpsertAuthIdentity(ctx, authIdentity); err != nil {
		return TokenPair{}, err
	}

	return s.issueSession(ctx, user)
}

func (s *Service) LoginWithMicrosoft(ctx context.Context, idToken string) (TokenPair, error) {
	if s.microsoftVerifier == nil {
		return TokenPair{}, ErrMicrosoftAuthDisabled
	}
	identity, err := s.microsoftVerifier.VerifyIDToken(ctx, idToken)
	if err != nil {
		return TokenPair{}, err
	}
	if strings.TrimSpace(identity.Subject) == "" {
		return TokenPair{}, ErrInvalidMicrosoftToken
	}

	user, err := s.store.GetUserByAuthIdentity(ctx, domain.AuthProviderMicrosoft, identity.Subject)
	switch {
	case err == nil:
	case errors.Is(err, store.ErrNotFound):
		user, err = s.resolveOrCreateMicrosoftUser(ctx, identity)
		if err != nil {
			return TokenPair{}, err
		}
	default:
		return TokenPair{}, err
	}

	if identity.DisplayName != "" || identity.Email != "" || identity.AvatarURL != "" {
		updated := user
		if identity.DisplayName != "" {
			updated.DisplayName = identity.DisplayName
		}
		if identity.Email != "" {
			updated.Email = identity.Email
		}
		if identity.AvatarURL != "" {
			updated.AvatarURL = identity.AvatarURL
		}
		if err := s.store.UpsertUser(ctx, updated); err != nil {
			return TokenPair{}, err
		}
		user = updated
	}

	now := time.Now().UTC()
	authIdentity := domain.AuthIdentity{
		ID:              authIdentityID(domain.AuthProviderMicrosoft, identity.Subject),
		UserID:          user.ID,
		Provider:        domain.AuthProviderMicrosoft,
		ProviderSubject: identity.Subject,
		Email:           identity.Email,
		EmailVerified:   identity.EmailVerified,
		AvatarURL:       identity.AvatarURL,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.store.UpsertAuthIdentity(ctx, authIdentity); err != nil {
		return TokenPair{}, err
	}

	return s.issueSession(ctx, user)
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (TokenPair, error) {
	session, err := s.store.GetSessionByRefreshToken(ctx, refreshToken)
	if err != nil {
		return TokenPair{}, ErrInvalidToken
	}
	if session.RevokedAt != nil || session.ExpiresAt.Before(time.Now().UTC()) {
		return TokenPair{}, ErrInvalidToken
	}
	user, err := s.store.GetUserByID(ctx, session.UserID)
	if err != nil {
		return TokenPair{}, ErrInvalidToken
	}
	if err := s.store.RevokeSession(ctx, session.ID); err != nil {
		return TokenPair{}, err
	}
	return s.issueSession(ctx, user)
}

func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	session, err := s.store.GetSessionByRefreshToken(ctx, refreshToken)
	if err != nil {
		return ErrInvalidToken
	}
	return s.store.RevokeSession(ctx, session.ID)
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
	return s.store.GetUserByID(context.Background(), userID)
}

func (s *Service) issueSession(ctx context.Context, user domain.User) (TokenPair, error) {
	now := time.Now().UTC()
	if err := s.store.EnsureUserChatModelDefaults(ctx, user.ID); err != nil {
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
	if err := s.store.CreateSession(ctx, session); err != nil {
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

func (s *Service) resolveOrCreateGoogleUser(ctx context.Context, identity VerifiedGoogleIdentity) (domain.User, error) {
	return s.resolveOrCreateExternalUser(ctx, externalIdentity{
		Subject:       identity.Subject,
		Email:         identity.Email,
		EmailVerified: identity.EmailVerified,
		DisplayName:   identity.DisplayName,
		AvatarURL:     identity.AvatarURL,
		UsernameBase:  "googleuser",
	})
}

func (s *Service) resolveOrCreateMicrosoftUser(ctx context.Context, identity VerifiedMicrosoftIdentity) (domain.User, error) {
	return s.resolveOrCreateExternalUser(ctx, externalIdentity{
		Subject:       identity.Subject,
		Email:         identity.Email,
		EmailVerified: identity.EmailVerified,
		DisplayName:   identity.DisplayName,
		AvatarURL:     identity.AvatarURL,
		UsernameBase:  "microsoftuser",
	})
}

type externalIdentity struct {
	Subject       string
	Email         string
	EmailVerified bool
	DisplayName   string
	AvatarURL     string
	UsernameBase  string
}

func (s *Service) resolveOrCreateExternalUser(ctx context.Context, identity externalIdentity) (domain.User, error) {
	username, err := s.allocateUsername(
		ctx,
		identity.Email,
		identity.DisplayName,
		identity.Subject,
		identity.UsernameBase,
	)
	if err != nil {
		return domain.User{}, err
	}
	displayName := strings.TrimSpace(identity.DisplayName)
	if displayName == "" {
		displayName = username
	}
	user := domain.User{
		ID:           uuid.NewString(),
		Username:     username,
		DisplayName:  displayName,
		Email:        strings.TrimSpace(identity.Email),
		AvatarURL:    strings.TrimSpace(identity.AvatarURL),
		PasswordHash: "",
		CreatedAt:    time.Now().UTC(),
	}
	if err := s.store.UpsertUser(ctx, user); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (s *Service) allocateUsername(
	ctx context.Context,
	email string,
	displayName string,
	subject string,
	fallbackBase string,
) (string, error) {
	candidates := []string{
		usernameFromEmail(email),
		usernameFromDisplayName(displayName),
		usernameFromSubject(subject),
	}

	base := ""
	for _, candidate := range candidates {
		if candidate != "" {
			base = candidate
			break
		}
	}
	if base == "" {
		base = fallbackBase
	}

	for i := range 100 {
		candidate := base
		if i > 0 {
			candidate = fmt.Sprintf("%s-%d", base, i+1)
		}
		_, err := s.store.GetUserByUsername(ctx, candidate)
		if errors.Is(err, store.ErrNotFound) {
			return candidate, nil
		}
		if err != nil {
			return "", err
		}
	}
	return "", errors.New("unable to allocate unique username")
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

func authIdentityID(provider domain.AuthProvider, subject string) string {
	return string(provider) + ":" + strings.TrimSpace(subject)
}
