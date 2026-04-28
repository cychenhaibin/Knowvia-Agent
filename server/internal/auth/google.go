package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const googleJWKSURL = "https://www.googleapis.com/oauth2/v3/certs"

type VerifiedGoogleIdentity struct {
	Subject       string
	Email         string
	EmailVerified bool
	DisplayName   string
	AvatarURL     string
}

type GoogleTokenVerifier interface {
	VerifyIDToken(ctx context.Context, rawToken string) (VerifiedGoogleIdentity, error)
}

type googleTokenVerifier struct {
	webClientID string
	httpClient  *http.Client

	mu        sync.RWMutex
	keys      map[string]*rsa.PublicKey
	expiresAt time.Time
	etag      string
}

type googleJWKS struct {
	Keys []googleJWK `json:"keys"`
}

type googleJWK struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func NewGoogleTokenVerifier(webClientID string) GoogleTokenVerifier {
	webClientID = strings.TrimSpace(webClientID)
	if webClientID == "" {
		return nil
	}
	return &googleTokenVerifier{
		webClientID: webClientID,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (v *googleTokenVerifier) VerifyIDToken(ctx context.Context, rawToken string) (VerifiedGoogleIdentity, error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return VerifiedGoogleIdentity{}, ErrInvalidGoogleToken
	}

	parser := jwt.NewParser(jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}))
	token, err := parser.Parse(rawToken, func(token *jwt.Token) (interface{}, error) {
		kid, _ := token.Header["kid"].(string)
		if strings.TrimSpace(kid) == "" {
			return nil, ErrInvalidGoogleToken
		}
		return v.lookupKey(ctx, kid)
	})
	if err != nil {
		if errors.Is(err, ErrGoogleKeysUnavailable) {
			return VerifiedGoogleIdentity{}, ErrGoogleKeysUnavailable
		}
		return VerifiedGoogleIdentity{}, ErrInvalidGoogleToken
	}
	if token == nil || !token.Valid {
		return VerifiedGoogleIdentity{}, ErrInvalidGoogleToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return VerifiedGoogleIdentity{}, ErrInvalidGoogleToken
	}
	if !claimAudienceContains(claims["aud"], v.webClientID) {
		return VerifiedGoogleIdentity{}, ErrInvalidGoogleToken
	}
	issuer, _ := claims["iss"].(string)
	if issuer != "accounts.google.com" && issuer != "https://accounts.google.com" {
		return VerifiedGoogleIdentity{}, ErrInvalidGoogleToken
	}

	identity := VerifiedGoogleIdentity{
		Subject:       strings.TrimSpace(claimString(claims, "sub")),
		Email:         strings.TrimSpace(claimString(claims, "email")),
		EmailVerified: claimBool(claims, "email_verified"),
		DisplayName:   strings.TrimSpace(claimString(claims, "name")),
		AvatarURL:     strings.TrimSpace(claimString(claims, "picture")),
	}
	if identity.Subject == "" {
		return VerifiedGoogleIdentity{}, ErrInvalidGoogleToken
	}
	return identity, nil
}

func (v *googleTokenVerifier) lookupKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.RLock()
	if time.Now().UTC().Before(v.expiresAt) {
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
		return nil, ErrInvalidGoogleToken
	}
	return key, nil
}

func (v *googleTokenVerifier) refreshKeys(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, googleJWKSURL, nil)
	if err != nil {
		return err
	}

	v.mu.RLock()
	if v.etag != "" {
		req.Header.Set("If-None-Match", v.etag)
	}
	v.mu.RUnlock()

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return ErrGoogleKeysUnavailable
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotModified:
		v.mu.Lock()
		v.expiresAt = time.Now().UTC().Add(cacheDuration(resp.Header.Get("Cache-Control")))
		v.mu.Unlock()
		return nil
	default:
		return ErrGoogleKeysUnavailable
	}

	var payload googleJWKS
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return ErrGoogleKeysUnavailable
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
		return ErrGoogleKeysUnavailable
	}

	v.mu.Lock()
	v.keys = keys
	v.etag = resp.Header.Get("ETag")
	v.expiresAt = time.Now().UTC().Add(cacheDuration(resp.Header.Get("Cache-Control")))
	v.mu.Unlock()
	return nil
}

func rsaPublicKeyFromJWK(nValue, eValue string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nValue)
	if err != nil {
		return nil, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eValue)
	if err != nil {
		return nil, err
	}

	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes).Int64()
	if n.Sign() <= 0 || e <= 0 {
		return nil, errors.New("invalid rsa key")
	}

	return &rsa.PublicKey{
		N: n,
		E: int(e),
	}, nil
}

func cacheDuration(cacheControl string) time.Duration {
	const fallback = 15 * time.Minute
	for _, part := range strings.Split(cacheControl, ",") {
		part = strings.TrimSpace(part)
		if !strings.HasPrefix(strings.ToLower(part), "max-age=") {
			continue
		}
		seconds, err := time.ParseDuration(strings.TrimPrefix(strings.ToLower(part), "max-age=") + "s")
		if err == nil && seconds > 0 {
			return seconds
		}
	}
	return fallback
}

func claimString(claims map[string]interface{}, key string) string {
	value, _ := claims[key].(string)
	return value
}

func claimBool(claims map[string]interface{}, key string) bool {
	switch value := claims[key].(type) {
	case bool:
		return value
	case string:
		return strings.EqualFold(value, "true")
	default:
		return false
	}
}

func claimAudienceContains(raw interface{}, audience string) bool {
	audience = strings.TrimSpace(audience)
	if audience == "" {
		return false
	}
	switch value := raw.(type) {
	case string:
		return strings.TrimSpace(value) == audience
	case []string:
		for _, item := range value {
			if strings.TrimSpace(item) == audience {
				return true
			}
		}
	case []interface{}:
		for _, item := range value {
			text, _ := item.(string)
			if strings.TrimSpace(text) == audience {
				return true
			}
		}
	}
	return false
}

var usernameSanitizer = regexp.MustCompile(`[^a-z0-9._-]+`)

func usernameFromEmail(email string) string {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return ""
	}
	local := email
	if at := strings.Index(email, "@"); at > 0 {
		local = email[:at]
	}
	return sanitizeUsername(local)
}

func usernameFromDisplayName(displayName string) string {
	displayName = strings.TrimSpace(strings.ToLower(displayName))
	if displayName == "" {
		return ""
	}
	displayName = strings.ReplaceAll(displayName, " ", ".")
	return sanitizeUsername(displayName)
}

func usernameFromSubject(subject string) string {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return ""
	}
	return sanitizeUsername("google-" + subject)
}

func sanitizeUsername(value string) string {
	value = usernameSanitizer.ReplaceAllString(strings.ToLower(strings.TrimSpace(value)), "-")
	value = strings.Trim(value, "-._")
	if value == "" {
		return ""
	}
	if len(value) > 48 {
		value = value[:48]
		value = strings.Trim(value, "-._")
	}
	if value == "" {
		return ""
	}
	return value
}

var _ GoogleTokenVerifier = (*googleTokenVerifier)(nil)
