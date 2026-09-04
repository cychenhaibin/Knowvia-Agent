package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/config"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
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
	userStore         UserStore
	authIdentityStore AuthIdentityStore
	sessionStore      SessionStore
	chatModelStore    ChatModelDefaultsStore
	cfg               config.Config
	googleVerifier    GoogleTokenVerifier
	microsoftVerifier MicrosoftTokenVerifier
}

func NewService(deps ServiceDeps, cfg config.Config) *Service {
	return NewServiceWithVerifiers(
		deps,
		cfg,
		NewGoogleTokenVerifier(cfg.GoogleWebClientID),
		NewMicrosoftTokenVerifier(cfg.MicrosoftClientID, cfg.MicrosoftTenantID),
	)
}

func NewServiceWithGoogleVerifier(deps ServiceDeps, cfg config.Config, verifier GoogleTokenVerifier) *Service {
	return NewServiceWithVerifiers(
		deps,
		cfg,
		verifier,
		NewMicrosoftTokenVerifier(cfg.MicrosoftClientID, cfg.MicrosoftTenantID),
	)
}

func NewServiceWithMicrosoftVerifier(deps ServiceDeps, cfg config.Config, verifier MicrosoftTokenVerifier) *Service {
	return NewServiceWithVerifiers(
		deps,
		cfg,
		NewGoogleTokenVerifier(cfg.GoogleWebClientID),
		verifier,
	)
}

func NewServiceWithVerifiers(
	deps ServiceDeps,
	cfg config.Config,
	googleVerifier GoogleTokenVerifier,
	microsoftVerifier MicrosoftTokenVerifier,
) *Service {
	return &Service{
		userStore:         deps.Users,
		authIdentityStore: deps.Identities,
		sessionStore:      deps.Sessions,
		chatModelStore:    deps.ChatModels,
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
		if existing, err := s.userStore.GetUserByUsername(ctx, devUser.Username); err == nil {
			user.ID = existing.ID
			user.CreatedAt = existing.CreatedAt
		}
		if err := s.userStore.UpsertUser(ctx, user); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) Login(ctx context.Context, username, password string) (TokenPair, error) {
	user, err := s.userStore.GetUserByUsername(ctx, username)
	if err != nil {
		return TokenPair{}, ErrInvalidCredentials
	}
	if !verifyPassword(password, user.PasswordHash) {
		return TokenPair{}, ErrInvalidCredentials
	}
	if !strings.HasPrefix(user.PasswordHash, "$2") {
		user.PasswordHash = hashPassword(password)
		if err := s.userStore.UpsertUser(ctx, user); err != nil {
			return TokenPair{}, err
		}
	}
	return s.issueSession(ctx, user)
}
