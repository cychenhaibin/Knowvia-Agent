package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/persistence"
)

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

	user, err := s.userStore.GetUserByAuthIdentity(ctx, domain.AuthProviderGoogle, identity.Subject)
	switch {
	case err == nil:
	case errors.Is(err, persistence.ErrNotFound):
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
		if err := s.userStore.UpsertUser(ctx, updated); err != nil {
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
	if err := s.authIdentityStore.UpsertAuthIdentity(ctx, authIdentity); err != nil {
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

	user, err := s.userStore.GetUserByAuthIdentity(ctx, domain.AuthProviderMicrosoft, identity.Subject)
	switch {
	case err == nil:
	case errors.Is(err, persistence.ErrNotFound):
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
		if err := s.userStore.UpsertUser(ctx, updated); err != nil {
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
	if err := s.authIdentityStore.UpsertAuthIdentity(ctx, authIdentity); err != nil {
		return TokenPair{}, err
	}

	return s.issueSession(ctx, user)
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
	if err := s.userStore.UpsertUser(ctx, user); err != nil {
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
		_, err := s.userStore.GetUserByUsername(ctx, candidate)
		if errors.Is(err, persistence.ErrNotFound) {
			return candidate, nil
		}
		if err != nil {
			return "", err
		}
	}
	return "", errors.New("unable to allocate unique username")
}

func authIdentityID(provider domain.AuthProvider, subject string) string {
	return string(provider) + ":" + strings.TrimSpace(subject)
}
