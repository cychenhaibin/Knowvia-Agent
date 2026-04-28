package auth

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const microsoftOpenIDConfigURLTemplate = "https://login.microsoftonline.com/%s/v2.0/.well-known/openid-configuration"

type VerifiedMicrosoftIdentity struct {
	Subject       string
	Email         string
	EmailVerified bool
	DisplayName   string
	AvatarURL     string
}

type MicrosoftTokenVerifier interface {
	VerifyIDToken(ctx context.Context, rawToken string) (VerifiedMicrosoftIdentity, error)
}

type microsoftTokenVerifier struct {
	clientID   string
	tenantID   string
	httpClient *http.Client

	mu          sync.RWMutex
	issuer      string
	jwksURI     string
	keys        map[string]*rsa.PublicKey
	configETag  string
	configUntil time.Time
	keysETag    string
	keysUntil   time.Time
}

type microsoftOpenIDConfiguration struct {
	Issuer  string `json:"issuer"`
	JWKSURI string `json:"jwks_uri"`
}

func NewMicrosoftTokenVerifier(clientID, tenantID string) MicrosoftTokenVerifier {
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		return nil
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		tenantID = "consumers"
	}
	return &microsoftTokenVerifier{
		clientID: clientID,
		tenantID: tenantID,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (v *microsoftTokenVerifier) VerifyIDToken(ctx context.Context, rawToken string) (VerifiedMicrosoftIdentity, error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return VerifiedMicrosoftIdentity{}, ErrInvalidMicrosoftToken
	}

	if err := v.ensureMetadata(ctx); err != nil {
		return VerifiedMicrosoftIdentity{}, err
	}

	parser := jwt.NewParser(jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}))
	token, err := parser.Parse(rawToken, func(token *jwt.Token) (interface{}, error) {
		kid, _ := token.Header["kid"].(string)
		if strings.TrimSpace(kid) == "" {
			return nil, ErrInvalidMicrosoftToken
		}
		return v.lookupKey(ctx, kid)
	})
	if err != nil {
		if errors.Is(err, ErrMicrosoftIdentityUnavailable) {
			return VerifiedMicrosoftIdentity{}, ErrMicrosoftIdentityUnavailable
		}
		return VerifiedMicrosoftIdentity{}, ErrInvalidMicrosoftToken
	}
	if token == nil || !token.Valid {
		return VerifiedMicrosoftIdentity{}, ErrInvalidMicrosoftToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return VerifiedMicrosoftIdentity{}, ErrInvalidMicrosoftToken
	}
	if !claimAudienceContains(claims["aud"], v.clientID) {
		return VerifiedMicrosoftIdentity{}, ErrInvalidMicrosoftToken
	}

	issuer, _ := claims["iss"].(string)
	if strings.TrimSpace(issuer) != strings.TrimSpace(v.issuer) {
		return VerifiedMicrosoftIdentity{}, ErrInvalidMicrosoftToken
	}

	subject := strings.TrimSpace(claimString(claims, "sub"))
	if subject == "" {
		return VerifiedMicrosoftIdentity{}, ErrInvalidMicrosoftToken
	}

	email := strings.TrimSpace(claimString(claims, "email"))
	emailVerified := claimBool(claims, "email_verified")
	if email == "" {
		email = strings.TrimSpace(claimString(claims, "preferred_username"))
	}
	if email != "" && !emailVerified {
		emailVerified = true
	}

	return VerifiedMicrosoftIdentity{
		Subject:       subject,
		Email:         email,
		EmailVerified: emailVerified,
		DisplayName:   strings.TrimSpace(claimString(claims, "name")),
		AvatarURL:     "",
	}, nil
}

func (v *microsoftTokenVerifier) ensureMetadata(ctx context.Context) error {
	v.mu.RLock()
	if time.Now().UTC().Before(v.configUntil) && v.issuer != "" && v.jwksURI != "" {
		v.mu.RUnlock()
		return nil
	}
	v.mu.RUnlock()

	return v.refreshConfiguration(ctx)
}

func (v *microsoftTokenVerifier) refreshConfiguration(ctx context.Context) error {
	configURL := strings.Replace(microsoftOpenIDConfigURLTemplate, "%s", strings.TrimSpace(v.tenantID), 1)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, configURL, nil)
	if err != nil {
		return ErrMicrosoftIdentityUnavailable
	}

	v.mu.RLock()
	if v.configETag != "" {
		req.Header.Set("If-None-Match", v.configETag)
	}
	v.mu.RUnlock()

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return ErrMicrosoftIdentityUnavailable
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotModified:
		v.mu.Lock()
		v.configUntil = time.Now().UTC().Add(cacheDuration(resp.Header.Get("Cache-Control")))
		v.mu.Unlock()
		return nil
	default:
		return ErrMicrosoftIdentityUnavailable
	}

	var config microsoftOpenIDConfiguration
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return ErrMicrosoftIdentityUnavailable
	}
	if strings.TrimSpace(config.Issuer) == "" || strings.TrimSpace(config.JWKSURI) == "" {
		return ErrMicrosoftIdentityUnavailable
	}

	v.mu.Lock()
	v.issuer = strings.TrimSpace(config.Issuer)
	v.jwksURI = strings.TrimSpace(config.JWKSURI)
	v.configETag = resp.Header.Get("ETag")
	v.configUntil = time.Now().UTC().Add(cacheDuration(resp.Header.Get("Cache-Control")))
	v.mu.Unlock()
	return nil
}

func (v *microsoftTokenVerifier) lookupKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.RLock()
	if time.Now().UTC().Before(v.keysUntil) {
		if key, ok := v.keys[kid]; ok {
			v.mu.RUnlock()
			return key, nil
		}
	}
	v.mu.RUnlock()

	if err := v.refreshKeys(ctx); err != nil {
		return nil, err
	}

	v.mu.RLock()
	defer v.mu.RUnlock()
	key, ok := v.keys[kid]
	if !ok {
		return nil, ErrInvalidMicrosoftToken
	}
	return key, nil
}

func (v *microsoftTokenVerifier) refreshKeys(ctx context.Context) error {
	if err := v.ensureMetadata(ctx); err != nil {
		return err
	}

	v.mu.RLock()
	jwksURI := v.jwksURI
	etag := v.keysETag
	v.mu.RUnlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURI, nil)
	if err != nil {
		return ErrMicrosoftIdentityUnavailable
	}
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return ErrMicrosoftIdentityUnavailable
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotModified:
		v.mu.Lock()
		v.keysUntil = time.Now().UTC().Add(cacheDuration(resp.Header.Get("Cache-Control")))
		v.mu.Unlock()
		return nil
	default:
		return ErrMicrosoftIdentityUnavailable
	}

	var payload googleJWKS
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return ErrMicrosoftIdentityUnavailable
	}

	keys := make(map[string]*rsa.PublicKey, len(payload.Keys))
	for _, jwk := range payload.Keys {
		if jwk.Kty != "RSA" || jwk.Kid == "" || jwk.N == "" || jwk.E == "" {
			continue
		}
		key, err := rsaPublicKeyFromJWK(jwk.N, jwk.E)
		if err != nil {
			continue
		}
		keys[jwk.Kid] = key
	}
	if len(keys) == 0 {
		return ErrMicrosoftIdentityUnavailable
	}

	v.mu.Lock()
	v.keys = keys
	v.keysETag = resp.Header.Get("ETag")
	v.keysUntil = time.Now().UTC().Add(cacheDuration(resp.Header.Get("Cache-Control")))
	v.mu.Unlock()
	return nil
}
