package auth

import (
	"bytes"
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
var ErrFluxAInvalidCredentials = errors.New("invalid FluxA credentials")
var ErrFluxA2FARequired = errors.New("FluxA two-factor authentication is required")
var ErrFluxAUnavailable = errors.New("FluxA identity service is unavailable")
var ErrFluxAUnsupportedSite = errors.New("unsupported FluxA site")
var ErrFluxANotConnected = errors.New("FluxA account is not connected")
var ErrFluxAReauthenticationRequired = errors.New("FluxA re-authentication is required")

type FluxASite = domain.FluxASite

const (
	FluxASitePaid = domain.FluxASitePaid
	FluxASiteFree = domain.FluxASiteFree
)

type VerifiedFluxAIdentity struct {
	Subject     string
	Username    string
	DisplayName string
	Email       string
	AvatarURL   string
	Group       string
}

type fluxAIdentityVerifier struct {
	paidOrigin string
	freeOrigin string
	httpClient *http.Client
}

type fluxACredentialAuthenticator struct {
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
	Group       string `json:"group"`
}

type fluxACredentialResponse struct {
	Success bool                        `json:"success"`
	Data    fluxACredentialResponseData `json:"data"`
}

type fluxACredentialResponseData struct {
	AccessToken string `json:"access_token"`
	Require2FA  bool   `json:"require_2fa"`
}

func NewFluxAIdentityVerifier(paidOrigin, freeOrigin string) FluxAIdentityVerifier {
	return newFluxAIdentityVerifier(paidOrigin, freeOrigin, newFluxAHTTPClient())
}

func NewFluxACredentialAuthenticator(paidOrigin, freeOrigin string) FluxACredentialAuthenticator {
	return newFluxACredentialAuthenticator(paidOrigin, freeOrigin, newFluxAHTTPClient())
}

func newFluxAHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
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

func newFluxACredentialAuthenticator(paidOrigin, freeOrigin string, httpClient *http.Client) *fluxACredentialAuthenticator {
	paidOrigin, _ = normalizeFluxAOrigin(paidOrigin)
	freeOrigin, _ = normalizeFluxAOrigin(freeOrigin)
	return &fluxACredentialAuthenticator{
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
		Group:       strings.TrimSpace(payload.Data.Group),
	}
	if payload.Data.ID <= 0 || identity.Username == "" {
		return VerifiedFluxAIdentity{}, ErrFluxAInvalidToken
	}
	return identity, nil
}

func (v *fluxAIdentityVerifier) origin(site FluxASite) (string, error) {
	return fluxAOrigin(site, v.paidOrigin, v.freeOrigin)
}

func (a *fluxACredentialAuthenticator) origin(site FluxASite) (string, error) {
	return fluxAOrigin(site, a.paidOrigin, a.freeOrigin)
}

func fluxAOrigin(site FluxASite, paidOrigin, freeOrigin string) (string, error) {
	switch site {
	case FluxASitePaid:
		if paidOrigin == "" {
			return "", ErrFluxAUnavailable
		}
		return paidOrigin, nil
	case FluxASiteFree:
		if freeOrigin == "" {
			return "", ErrFluxAUnavailable
		}
		return freeOrigin, nil
	default:
		return "", ErrFluxAUnsupportedSite
	}
}

func (a *fluxACredentialAuthenticator) Login(ctx context.Context, site FluxASite, username, password string) (string, error) {
	origin, err := a.origin(site)
	if err != nil {
		return "", err
	}

	username = strings.TrimSpace(username)
	if username == "" || strings.TrimSpace(password) == "" {
		return "", ErrFluxAInvalidCredentials
	}

	body, err := json.Marshal(struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}{Username: username, Password: password})
	if err != nil {
		return "", ErrFluxAUnavailable
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, origin+"/api/user/login", bytes.NewReader(body))
	if err != nil {
		return "", ErrFluxAUnavailable
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", ErrFluxAUnavailable
	}
	defer resp.Body.Close()

	decoder := json.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	var payload fluxACredentialResponse
	decodeErr := decoder.Decode(&payload)
	trailingErr := decoder.Decode(&struct{}{})
	validPayload := decodeErr == nil && errors.Is(trailingErr, io.EOF)
	if validPayload && payload.Data.Require2FA {
		return "", ErrFluxA2FARequired
	}

	switch resp.StatusCode {
	case http.StatusOK:
		if !validPayload {
			return "", ErrFluxAUnavailable
		}
	case http.StatusUnauthorized, http.StatusForbidden:
		return "", ErrFluxAInvalidCredentials
	default:
		return "", ErrFluxAUnavailable
	}

	if !payload.Success {
		return "", ErrFluxAInvalidCredentials
	}
	accessToken := strings.TrimSpace(payload.Data.AccessToken)
	if accessToken == "" {
		return "", ErrFluxAInvalidCredentials
	}
	return accessToken, nil
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
	user, identity, err := s.resolveFluxAUser(ctx, site, accessToken)
	if err != nil {
		return TokenPair{}, err
	}
	tokens, err := s.issueSession(ctx, user)
	if err == nil {
		tokens.FluxAGroup = identity.Group
		tokens.FluxAUsername = identity.Username
	}
	return tokens, err
}

func (s *Service) resolveFluxAUser(ctx context.Context, site FluxASite, accessToken string) (domain.User, VerifiedFluxAIdentity, error) {
	provider, err := fluxAProvider(site)
	if err != nil {
		return domain.User{}, VerifiedFluxAIdentity{}, err
	}
	if s.fluxAVerifier == nil {
		return domain.User{}, VerifiedFluxAIdentity{}, ErrFluxAUnavailable
	}

	identity, err := s.fluxAVerifier.Verify(ctx, site, accessToken)
	if err != nil {
		return domain.User{}, VerifiedFluxAIdentity{}, err
	}
	identity, err = normalizeFluxAIdentity(identity)
	if err != nil {
		return domain.User{}, VerifiedFluxAIdentity{}, err
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
			return domain.User{}, VerifiedFluxAIdentity{}, err
		}
	default:
		return domain.User{}, VerifiedFluxAIdentity{}, err
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
			return domain.User{}, VerifiedFluxAIdentity{}, err
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
		return domain.User{}, VerifiedFluxAIdentity{}, err
	}

	return user, identity, nil
}

func (s *Service) LoginWithFluxACredentials(ctx context.Context, site FluxASite, username, password string) (TokenPair, error) {
	if _, err := fluxAProvider(site); err != nil {
		return TokenPair{}, err
	}
	if s.fluxACredentials == nil || s.fluxACipher == nil {
		return TokenPair{}, ErrFluxAUnavailable
	}
	if s.fluxAAuthenticator == nil {
		return TokenPair{}, ErrFluxAUnavailable
	}
	accessToken, err := s.fluxAAuthenticator.Login(ctx, site, strings.TrimSpace(username), password)
	if err != nil {
		return TokenPair{}, err
	}
	user, identity, err := s.resolveFluxAUser(ctx, site, accessToken)
	if err != nil {
		return TokenPair{}, err
	}
	ciphertext, err := s.fluxACipher.Encrypt(accessToken, fluxACredentialAdditionalData(user.ID, site))
	if err != nil {
		return TokenPair{}, err
	}
	now := time.Now().UTC()
	if err := s.fluxACredentials.UpsertFluxACredential(ctx, domain.FluxACredential{
		UserID:          user.ID,
		Site:            site,
		TokenCiphertext: ciphertext,
		CreatedAt:       now,
		UpdatedAt:       now,
	}); err != nil {
		return TokenPair{}, err
	}
	tokens, err := s.issueSession(ctx, user)
	if err != nil {
		return TokenPair{}, err
	}
	tokens.FluxASite = &site
	tokens.FluxAGroup = identity.Group
	tokens.FluxAUsername = identity.Username
	return tokens, nil
}

func fluxACredentialAdditionalData(userID string, site FluxASite) []byte {
	return []byte(userID + ":" + string(site))
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
