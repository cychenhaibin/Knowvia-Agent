package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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

// This fails if the HTTP boundary exposes the domain object's snake_case JSON tags.
func TestFluxABalanceSerializesCamelCaseResponseFields(t *testing.T) {
	balance := auth.FluxABalance{
		Quota:                      12.5,
		QuotaPerUnit:               100,
		DisplayType:                "usd",
		USDExchangeRate:            7.2,
		CustomCurrencySymbol:       "¥",
		CustomCurrencyExchangeRate: 1.5,
	}
	router, accessToken := newFluxABalanceRouteWithCredentialSites(t, staticFluxABalanceFetcher{balance: balance}, auth.FluxASitePaid)
	response := serveFluxABalanceRequest(router, accessToken)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	wantKeys := map[string]bool{
		"quota": true, "quotaPerUnit": true, "quotaDisplayType": true,
		"usdExchangeRate": true, "customCurrencySymbol": true, "customCurrencyExchangeRate": true,
	}
	if len(payload) != len(wantKeys) {
		t.Fatalf("response keys = %v, want exactly %v", mapKeys(payload), wantKeys)
	}
	for key := range wantKeys {
		if _, ok := payload[key]; !ok {
			t.Fatalf("response missing %q: %s", key, response.Body.String())
		}
	}
}

// This fails if a missing paid credential does not retry against the free account.
func TestFluxABalanceFallsBackFromPaidToFree(t *testing.T) {
	fetcher := &recordingFluxABalanceFetcher{balances: map[auth.FluxASite]auth.FluxABalance{
		auth.FluxASiteFree: {Quota: 42},
	}}
	router, accessToken := newFluxABalanceRouteWithCredentialSites(t, fetcher, auth.FluxASiteFree)
	response := serveFluxABalanceRequest(router, accessToken)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	if got := fetcher.Sites(); len(got) != 1 || got[0] != auth.FluxASiteFree {
		t.Fatalf("fetch sites = %v, want [%s]", got, auth.FluxASiteFree)
	}
}

// This fails if a successful paid account query needlessly queries the free account.
func TestFluxABalanceUsesPaidWithoutFreeFallback(t *testing.T) {
	fetcher := &recordingFluxABalanceFetcher{balances: map[auth.FluxASite]auth.FluxABalance{
		auth.FluxASitePaid: {Quota: 84},
		auth.FluxASiteFree: {Quota: 42},
	}}
	router, accessToken := newFluxABalanceRouteWithCredentialSites(t, fetcher, auth.FluxASitePaid, auth.FluxASiteFree)
	response := serveFluxABalanceRequest(router, accessToken)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	if got := fetcher.Sites(); len(got) != 1 || got[0] != auth.FluxASitePaid {
		t.Fatalf("fetch sites = %v, want [%s]", got, auth.FluxASitePaid)
	}
}

// This fails if upstream outages are returned as a non-service-unavailable response.
func TestFluxABalanceMapsUnavailableToServiceUnavailable(t *testing.T) {
	router, accessToken := newFluxABalanceRouteWithCredentialSites(t, staticFluxABalanceFetcher{err: auth.ErrFluxAUnavailable}, auth.FluxASitePaid)
	response := serveFluxABalanceRequest(router, accessToken)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusServiceUnavailable, response.Body.String())
	}
}

type staticFluxABalanceFetcher struct {
	balance auth.FluxABalance
	err     error
}

func (f staticFluxABalanceFetcher) Balance(context.Context, auth.FluxASite, string) (auth.FluxABalance, error) {
	return f.balance, f.err
}

type recordingFluxABalanceFetcher struct {
	balances map[auth.FluxASite]auth.FluxABalance
	mu       sync.Mutex
	sites    []auth.FluxASite
}

func (f *recordingFluxABalanceFetcher) Balance(_ context.Context, site auth.FluxASite, _ string) (auth.FluxABalance, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sites = append(f.sites, site)
	return f.balances[site], nil
}

func (f *recordingFluxABalanceFetcher) Sites() []auth.FluxASite {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]auth.FluxASite(nil), f.sites...)
}

func newFluxABalanceRoute(t *testing.T, fetcher auth.FluxABalanceFetcher) (http.Handler, string) {
	t.Helper()
	return newFluxABalanceRouteWithCredentialSites(t, fetcher, auth.FluxASiteFree)
}

func newFluxABalanceRouteWithCredentialSites(t *testing.T, fetcher auth.FluxABalanceFetcher, sites ...auth.FluxASite) (http.Handler, string) {
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
	for _, site := range sites {
		ciphertext, err := cipher.Encrypt("upstream-token", []byte(user.ID+":"+string(site)))
		if err != nil {
			t.Fatalf("encrypt %s credential: %v", site, err)
		}
		if err := memory.UpsertFluxACredential(context.Background(), domain.FluxACredential{UserID: user.ID, Site: site, TokenCiphertext: ciphertext}); err != nil {
			t.Fatalf("upsert %s credential: %v", site, err)
		}
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

func serveFluxABalanceRequest(router http.Handler, accessToken string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/v1/fluxa/balance", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	return response
}

func mapKeys(values map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}
