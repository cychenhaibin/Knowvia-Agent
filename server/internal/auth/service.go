package auth

import (
	"context"
	"errors"
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
	userStore          UserStore
	authIdentityStore  AuthIdentityStore
	sessionStore       SessionStore
	chatModelStore     ChatModelDefaultsStore
	cfg                config.Config
	googleVerifier     GoogleTokenVerifier
	microsoftVerifier  MicrosoftTokenVerifier
	fluxAVerifier      FluxAIdentityVerifier
	fluxAAuthenticator FluxACredentialAuthenticator
	fluxACredentials   FluxACredentialStore
	fluxACipher        FluxACredentialCipher
	fluxAModels        FluxAModelGroupsFetcher
	fluxABalance       FluxABalanceFetcher
}

var nonExpiringSessionExpiresAt = time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)

func NewService(deps ServiceDeps, cfg config.Config) *Service {
	return NewServiceWithVerifiers(
		deps,
		cfg,
		NewGoogleTokenVerifier(cfg.GoogleWebClientID),
		NewMicrosoftTokenVerifier(cfg.MicrosoftClientID, cfg.MicrosoftTenantID),
		resolveFluxAVerifier(deps, cfg),
	)
}

func NewServiceWithGoogleVerifier(deps ServiceDeps, cfg config.Config, verifier GoogleTokenVerifier) *Service {
	return NewServiceWithVerifiers(
		deps,
		cfg,
		verifier,
		NewMicrosoftTokenVerifier(cfg.MicrosoftClientID, cfg.MicrosoftTenantID),
		resolveFluxAVerifier(deps, cfg),
	)
}

func NewServiceWithMicrosoftVerifier(deps ServiceDeps, cfg config.Config, verifier MicrosoftTokenVerifier) *Service {
	return NewServiceWithVerifiers(
		deps,
		cfg,
		NewGoogleTokenVerifier(cfg.GoogleWebClientID),
		verifier,
		resolveFluxAVerifier(deps, cfg),
	)
}

func NewServiceWithVerifiers(
	deps ServiceDeps,
	cfg config.Config,
	googleVerifier GoogleTokenVerifier,
	microsoftVerifier MicrosoftTokenVerifier,
	fluxAVerifier FluxAIdentityVerifier,
) *Service {
	return &Service{
		userStore:          deps.Users,
		authIdentityStore:  deps.Identities,
		sessionStore:       deps.Sessions,
		chatModelStore:     deps.ChatModels,
		cfg:                cfg,
		googleVerifier:     googleVerifier,
		microsoftVerifier:  microsoftVerifier,
		fluxAVerifier:      fluxAVerifier,
		fluxAAuthenticator: resolveFluxAAuthenticator(deps, cfg),
		fluxACredentials:   deps.FluxACredentials,
		fluxACipher:        resolveFluxACredentialCipher(deps, cfg),
		fluxAModels:        resolveFluxAModelGroupsFetcher(deps, cfg),
		fluxABalance:       resolveFluxABalanceFetcher(deps, cfg),
	}
}

func resolveFluxABalanceFetcher(deps ServiceDeps, cfg config.Config) FluxABalanceFetcher {
	if deps.FluxABalance != nil {
		return deps.FluxABalance
	}
	return NewFluxABalanceFetcher(cfg.FluxAPaidOrigin, cfg.FluxAFreeOrigin)
}

func resolveFluxAModelGroupsFetcher(deps ServiceDeps, cfg config.Config) FluxAModelGroupsFetcher {
	if deps.FluxAModels != nil {
		return deps.FluxAModels
	}
	return NewFluxAModelGroupsFetcher(cfg.FluxAPaidOrigin, cfg.FluxAFreeOrigin)
}

func resolveFluxACredentialCipher(deps ServiceDeps, cfg config.Config) FluxACredentialCipher {
	if deps.FluxACipher != nil {
		return deps.FluxACipher
	}
	cipher, err := NewFluxACredentialCipher(cfg.FluxACredentialsKey)
	if err != nil {
		return nil
	}
	return cipher
}

func resolveFluxAVerifier(deps ServiceDeps, cfg config.Config) FluxAIdentityVerifier {
	if deps.FluxAVerifier != nil {
		return deps.FluxAVerifier
	}
	return NewFluxAIdentityVerifier(cfg.FluxAPaidOrigin, cfg.FluxAFreeOrigin)
}

func resolveFluxAAuthenticator(deps ServiceDeps, cfg config.Config) FluxACredentialAuthenticator {
	if deps.FluxAAuthenticator != nil {
		return deps.FluxAAuthenticator
	}
	return NewFluxACredentialAuthenticator(cfg.FluxAPaidOrigin, cfg.FluxAFreeOrigin)
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
	if user.PasswordHash != hashPassword(password) {
		return TokenPair{}, ErrInvalidCredentials
	}
	return s.issueSession(ctx, user)
}
