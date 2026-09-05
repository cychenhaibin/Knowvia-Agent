package auth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/config"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/persistence"
)

var ErrFluxAInvalidToken = errors.New("invalid FluxA access token")
var ErrFluxAUnavailable = errors.New("FluxA identity service is unavailable")
var ErrFluxAUnsupportedSite = errors.New("unsupported FluxA site")

type FluxASite string

const (
	FluxASitePaid FluxASite = "paid"
	FluxASiteFree FluxASite = "free"
)

type VerifiedFluxAIdentity struct {
	Subject     string
	Username    string
	DisplayName string
	Email       string
	AvatarURL   string
}

type fluxAIdentityVerifier struct {
	paidOrigin string
	freeOrigin string
	httpClient *http.Client
}

type fluxAIdentityResponse struct {
	Success bool              `json:"success"`
	Data    fluxAIdentityData `json:"data"`
}

type fluxAIdentityData struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	AvatarURL   string `json:"avatar_url"`
}

func NewFluxAIdentityVerifier(paidOrigin, freeOrigin string) FluxAIdentityVerifier {
	return newFluxAIdentityVerifier(paidOrigin, freeOrigin, &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	})
}

func newFluxAIdentityVerifier(paidOrigin, freeOrigin string, httpClient *http.Client) *fluxAIdentityVerifier {
	paidOrigin, _ = normalizeFluxAOrigin(paidOrigin)
	freeOrigin, _ = normalizeFluxAOrigin(freeOrigin)
	return &fluxAIdentityVerifier{
		paidOrigin: paidOrigin,
		freeOrigin: freeOrigin,
		httpClient: httpClient,
	}
}

// normalizeFluxAOrigin accepts only an HTTPS origin. Configuration values
// must not include a path, credentials, query, or fragment because the
// verifier sends the upstream access token to this destination.
func normalizeFluxAOrigin(raw string) (string, bool) {
	normalized := config.NormalizeFluxAOrigin(raw)
	return normalized, normalized != ""
}

func (v *fluxAIdentityVerifier) Verify(ctx context.Context, site FluxASite, accessToken string) (VerifiedFluxAIdentity, error) {
	origin, err := v.origin(site)
	if err != nil {
		return VerifiedFluxAIdentity{}, err
	}

	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return VerifiedFluxAIdentity{}, ErrFluxAInvalidToken
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, origin+"/api/user/self", nil)
	if err != nil {
		return VerifiedFluxAIdentity{}, ErrFluxAUnavailable
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return VerifiedFluxAIdentity{}, ErrFluxAUnavailable
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized, http.StatusForbidden:
		return VerifiedFluxAIdentity{}, ErrFluxAInvalidToken
	default:
		return VerifiedFluxAIdentity{}, ErrFluxAUnavailable
	}

	decoder := json.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	var payload fluxAIdentityResponse
	if err := decoder.Decode(&payload); err != nil {
		return VerifiedFluxAIdentity{}, ErrFluxAUnavailable
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return VerifiedFluxAIdentity{}, ErrFluxAUnavailable
	}
	if !payload.Success {
		return VerifiedFluxAIdentity{}, ErrFluxAInvalidToken
	}

	identity := VerifiedFluxAIdentity{
		Subject:     strconv.FormatInt(payload.Data.ID, 10),
		Username:    strings.TrimSpace(payload.Data.Username),
		DisplayName: strings.TrimSpace(payload.Data.DisplayName),
		Email:       strings.TrimSpace(payload.Data.Email),
		AvatarURL:   strings.TrimSpace(payload.Data.AvatarURL),
	}
	if payload.Data.ID <= 0 || identity.Username == "" {
		return VerifiedFluxAIdentity{}, ErrFluxAInvalidToken
	}
	return identity, nil
}

func (v *fluxAIdentityVerifier) origin(site FluxASite) (string, error) {
	switch site {
	case FluxASitePaid:
		if v.paidOrigin == "" {
			return "", ErrFluxAUnavailable
		}
		return v.paidOrigin, nil
	case FluxASiteFree:
		if v.freeOrigin == "" {
			return "", ErrFluxAUnavailable
		}
		return v.freeOrigin, nil
	default:
		return "", ErrFluxAUnsupportedSite
	}
}

func fluxAProvider(site FluxASite) (domain.AuthProvider, error) {
	switch site {
	case FluxASitePaid:
		return domain.AuthProviderFluxAPaid, nil
	case FluxASiteFree:
		return domain.AuthProviderFluxAFree, nil
	default:
		return "", ErrFluxAUnsupportedSite
	}
}

func (s *Service) LoginWithFluxA(ctx context.Context, site FluxASite, accessToken string) (TokenPair, error) {
	provider, err := fluxAProvider(site)
	if err != nil {
		return TokenPair{}, err
	}
	if s.fluxAVerifier == nil {
		return TokenPair{}, ErrFluxAUnavailable
	}

	identity, err := s.fluxAVerifier.Verify(ctx, site, accessToken)
	if err != nil {
		return TokenPair{}, err
	}
	identity, err = normalizeFluxAIdentity(identity)
	if err != nil {
		return TokenPair{}, err
	}

	user, err := s.userStore.GetUserByAuthIdentity(ctx, provider, identity.Subject)
	switch {
	case err == nil:
	case errors.Is(err, persistence.ErrNotFound):
		user, err = s.resolveOrCreateExternalUser(ctx, externalIdentity{
			Subject:      "",
			UsernameBase: "fluxa-" + string(site) + "-" + identity.Subject,
		})
		if err != nil {
			return TokenPair{}, err
		}
	default:
		return TokenPair{}, err
	}

	displayName := identity.DisplayName
	if displayName == "" {
		displayName = identity.Username
	}
	updated := user
	if displayName != "" {
		updated.DisplayName = displayName
	}
	if identity.Email != "" {
		updated.Email = identity.Email
	}
	if identity.AvatarURL != "" {
		updated.AvatarURL = identity.AvatarURL
	}
	if updated != user {
		if err := s.userStore.UpsertUser(ctx, updated); err != nil {
			return TokenPair{}, err
		}
		user = updated
	}

	now := time.Now().UTC()
	if err := s.authIdentityStore.UpsertAuthIdentity(ctx, domain.AuthIdentity{
		ID:              authIdentityID(provider, identity.Subject),
		UserID:          user.ID,
		Provider:        provider,
		ProviderSubject: identity.Subject,
		Email:           identity.Email,
		EmailVerified:   false,
		AvatarURL:       identity.AvatarURL,
		CreatedAt:       now,
		UpdatedAt:       now,
	}); err != nil {
		return TokenPair{}, err
	}

	return s.issueSession(ctx, user)
}

func normalizeFluxAIdentity(identity VerifiedFluxAIdentity) (VerifiedFluxAIdentity, error) {
	identity.Subject = strings.TrimSpace(identity.Subject)
	identity.Username = strings.TrimSpace(identity.Username)
	identity.DisplayName = strings.TrimSpace(identity.DisplayName)
	identity.Email = strings.TrimSpace(identity.Email)
	identity.AvatarURL = strings.TrimSpace(identity.AvatarURL)

	subject, err := strconv.ParseInt(identity.Subject, 10, 64)
	if err != nil || subject <= 0 || identity.Username == "" {
		return VerifiedFluxAIdentity{}, ErrFluxAInvalidToken
	}
	identity.Subject = strconv.FormatInt(subject, 10)
	return identity, nil
}
