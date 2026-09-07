package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/store"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/auth"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/config"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

// This fails if the balance endpoint treats a user with no FluxA credentials as connected.
func TestFluxABalanceRejectsNonFluxAUser(t *testing.T) {
	memory := store.NewMemoryStore()
	service := auth.NewService(auth.ServiceDeps{
		Users:      memory,
		Identities: memory,
		Sessions:   memory,
		ChatModels: memory,
	}, config.Config{
		JWTSecret:  "test-secret",
		AccessTTL:  30 * time.Minute,
		RefreshTTL: 24 * time.Hour,
		DevUsers:   []config.DevUser{{Username: "password-user", Password: "password-user", DisplayName: "Password User"}},
	})
	if err := service.SeedDevUsers(context.Background()); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	tokens, err := service.Login(context.Background(), "password-user", "password-user")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/fluxa/balance", nil)
	req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	response := httptest.NewRecorder()
	NewRouter(service, nil, nil, nil, nil, nil).ServeHTTP(response, req)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusForbidden, response.Body.String())
	}
	payload := decodeAPIError(t, response.Body.Bytes())
	if payload.Error != "FluxA account is not connected" {
		t.Fatalf("error = %q, want safe not-connected message", payload.Error)
	}
}

// This fails if reauthentication errors are not translated to a safe unauthorized response.
func TestFluxABalanceMapsReauthenticationToUnauthorized(t *testing.T) {
	router, accessToken := newFluxABalanceRoute(t, staticFluxABalanceFetcher{err: auth.ErrFluxAReauthenticationRequired})
	req := httptest.NewRequest(http.MethodGet, "/v1/fluxa/balance", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusUnauthorized, response.Body.String())
	}
	payload := decodeAPIError(t, response.Body.Bytes())
	if payload.Error != "sign in to FluxA again" {
		t.Fatalf("error = %q, want %q", payload.Error, "sign in to FluxA again")
	}
	if strings.Contains(response.Body.String(), "upstream-token") {
		t.Fatalf("response exposed upstream credential: %s", response.Body.String())
	}
}

type staticFluxABalanceFetcher struct{ err error }

func (f staticFluxABalanceFetcher) Balance(context.Context, auth.FluxASite, string) (auth.FluxABalance, error) {
	return auth.FluxABalance{}, f.err
}

func newFluxABalanceRoute(t *testing.T, fetcher auth.FluxABalanceFetcher) (http.Handler, string) {
	t.Helper()
	memory := store.NewMemoryStore()
	user := domain.User{ID: "fluxa-balance-route-user", Username: "fluxa-balance-route-user", DisplayName: "FluxA Balance Route User", CreatedAt: time.Now().UTC()}
	if err := memory.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}
	cipher, err := auth.NewFluxACredentialCipher(make([]byte, 32))
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}
	ciphertext, err := cipher.Encrypt("upstream-token", []byte(user.ID+":free"))
	if err != nil {
		t.Fatalf("encrypt credential: %v", err)
	}
	if err := memory.UpsertFluxACredential(context.Background(), domain.FluxACredential{UserID: user.ID, Site: auth.FluxASiteFree, TokenCiphertext: ciphertext}); err != nil {
		t.Fatalf("upsert credential: %v", err)
	}
	service := auth.NewService(auth.ServiceDeps{
		Users:            memory,
		Identities:       memory,
		Sessions:         memory,
		ChatModels:       memory,
		FluxACredentials: memory,
		FluxACipher:      cipher,
		FluxABalance:     fetcher,
	}, config.Config{JWTSecret: "test-secret", FluxACredentialsKey: make([]byte, 32)})
	claims := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": user.ID})
	accessToken, err := claims.SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatalf("sign access token: %v", err)
	}
	return NewRouter(service, nil, nil, nil, nil, nil), accessToken
}
