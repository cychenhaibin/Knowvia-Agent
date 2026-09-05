package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/store"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

type fluxATestTransport struct{ base http.RoundTripper }

func (t fluxATestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	urlCopy := *req.URL
	urlCopy.Scheme = "http"
	clone.URL = &urlCopy
	return t.base.RoundTrip(clone)
}

func newTestFluxAIdentityVerifier(paidOrigin, freeOrigin string, base *http.Client) *fluxAIdentityVerifier {
	client := &http.Client{}
	if base != nil {
		copy := *base
		client = &copy
	}
	transport := client.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	client.Transport = fluxATestTransport{base: transport}
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return newFluxAIdentityVerifier(
		strings.Replace(paidOrigin, "http://", "https://", 1),
		strings.Replace(freeOrigin, "http://", "https://", 1),
		client,
	)
}

func TestFluxAVerifierRequestsSelectedSiteIdentity(t *testing.T) {
	var paidCalls atomic.Int32
	paidServer := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		paidCalls.Add(1)
	}))
	t.Cleanup(paidServer.Close)

	freeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if r.URL.Path != "/api/user/self" {
			t.Errorf("path = %q, want /api/user/self", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer upstream-token" {
			t.Errorf("Authorization = %q, want bearer token", got)
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Errorf("Accept = %q, want application/json", got)
		}
		if got := r.Header.Get("Content-Type"); got != "" {
			t.Errorf("Content-Type = %q, want empty", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"id":42,"username":"  fluxa-user  ","display_name":"  Fluxa User  ","email":"  user@example.com  ","avatar_url":"  https://example.com/avatar.png  "}}`))
	}))
	t.Cleanup(freeServer.Close)

	verifier := newTestFluxAIdentityVerifier(paidServer.URL, freeServer.URL, freeServer.Client())
	identity, err := verifier.Verify(context.Background(), FluxASiteFree, "  upstream-token  ")
	if err != nil {
		t.Fatalf("verify: %v", err)
	}

	want := VerifiedFluxAIdentity{
		Subject:     "42",
		Username:    "fluxa-user",
		DisplayName: "Fluxa User",
		Email:       "user@example.com",
		AvatarURL:   "https://example.com/avatar.png",
	}
	if identity != want {
		t.Fatalf("identity = %#v, want %#v", identity, want)
	}
	if got := paidCalls.Load(); got != 0 {
		t.Fatalf("paid site received %d calls, want 0", got)
	}
}

func TestNewFluxAVerifierRejectsNonHTTPSOrNonOriginConfiguration(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls.Add(1)
	}))
	t.Cleanup(server.Close)
	host := strings.TrimPrefix(server.URL, "http://")
	tests := []struct {
		name       string
		paidOrigin string
		freeOrigin string
		site       FluxASite
	}{
		{name: "http scheme", paidOrigin: server.URL, freeOrigin: "https://free.example", site: FluxASitePaid},
		{name: "path", paidOrigin: "https://" + host + "/api", freeOrigin: "https://free.example", site: FluxASitePaid},
		{name: "userinfo", paidOrigin: "https://user:pass@" + host, freeOrigin: "https://free.example", site: FluxASitePaid},
		{name: "query", paidOrigin: "https://" + host + "?token=secret", freeOrigin: "https://free.example", site: FluxASitePaid},
		{name: "fragment", paidOrigin: "https://" + host + "#section", freeOrigin: "https://free.example", site: FluxASitePaid},
		{name: "invalid free origin", paidOrigin: "https://paid.example", freeOrigin: "https://" + host + "/base", site: FluxASiteFree},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verifier := NewFluxAIdentityVerifier(tt.paidOrigin, tt.freeOrigin)
			_, err := verifier.Verify(context.Background(), tt.site, "upstream-token")
			if !errors.Is(err, ErrFluxAUnavailable) {
				t.Fatalf("error = %v, want ErrFluxAUnavailable", err)
			}
			if got := calls.Load(); got != 0 {
				t.Fatalf("unsafe origin received %d requests, want 0", got)
			}
		})
	}
}

func TestNewFluxAVerifierNormalizesValidatedOrigin(t *testing.T) {
	verifier, ok := NewFluxAIdentityVerifier("  HTTPS://Paid.Example:443/ ", "https://free.example/").(*fluxAIdentityVerifier)
	if !ok {
		t.Fatalf("verifier has type %T, want *fluxAIdentityVerifier", verifier)
	}
	if verifier.paidOrigin != "https://Paid.Example:443" {
		t.Fatalf("paid origin = %q, want normalized origin", verifier.paidOrigin)
	}
	if verifier.freeOrigin != "https://free.example" {
		t.Fatalf("free origin = %q, want normalized origin", verifier.freeOrigin)
	}
}

func TestFluxAVerifierRejectsUnknownSiteWithoutRequest(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls.Add(1)
	}))
	t.Cleanup(server.Close)

	verifier := newTestFluxAIdentityVerifier(server.URL, server.URL, server.Client())
	_, err := verifier.Verify(context.Background(), FluxASite("https://attacker.example"), "upstream-token")
	if !errors.Is(err, ErrFluxAUnsupportedSite) {
		t.Fatalf("error = %v, want ErrFluxAUnsupportedSite", err)
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("server received %d calls, want 0", got)
	}
}

func TestFluxAVerifierDoesNotFollowRedirects(t *testing.T) {
	var redirectedCalls atomic.Int32
	var redirectedAuthorization atomic.Value
	redirectedAuthorization.Store("")
	redirectedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectedCalls.Add(1)
		redirectedAuthorization.Store(r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"id":42,"username":"redirected-user"}}`))
	}))
	t.Cleanup(redirectedServer.Close)

	originServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, redirectedServer.URL+"/stolen-token", http.StatusFound)
	}))
	t.Cleanup(originServer.Close)

	verifier := newTestFluxAIdentityVerifier(originServer.URL, originServer.URL, originServer.Client())
	_, err := verifier.Verify(context.Background(), FluxASitePaid, "redirect-secret-token")
	if !errors.Is(err, ErrFluxAUnavailable) {
		t.Errorf("error = %v, want ErrFluxAUnavailable", err)
	}
	if got := redirectedCalls.Load(); got != 0 {
		t.Errorf("redirect target received %d calls, want 0", got)
	}
	if got := redirectedAuthorization.Load().(string); got != "" {
		t.Errorf("redirect target received Authorization %q, want empty", got)
	}
}

func TestFluxAVerifierReturnsSafeTimeoutError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)

	client := server.Client()
	client.Timeout = 25 * time.Millisecond
	verifier := newTestFluxAIdentityVerifier(server.URL, server.URL, client)
	const accessToken = "timeout-secret-token"
	_, err := verifier.Verify(context.Background(), FluxASitePaid, accessToken)
	if !errors.Is(err, ErrFluxAUnavailable) {
		t.Fatalf("error = %v, want ErrFluxAUnavailable", err)
	}
	if strings.Contains(err.Error(), accessToken) {
		t.Fatalf("error exposed access token: %v", err)
	}
}

func TestFluxAVerifierRejectsMalformedIdentityPayloads(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "unsuccessful response", body: `{"success":false,"data":{"id":42,"username":"user"}}`},
		{name: "zero subject", body: `{"success":true,"data":{"id":0,"username":"user"}}`},
		{name: "negative subject", body: `{"success":true,"data":{"id":-7,"username":"user"}}`},
		{name: "missing username", body: `{"success":true,"data":{"id":42}}`},
		{name: "blank username", body: `{"success":true,"data":{"id":42,"username":"   "}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.body))
			}))
			t.Cleanup(server.Close)

			verifier := newTestFluxAIdentityVerifier(server.URL, server.URL, server.Client())
			_, err := verifier.Verify(context.Background(), FluxASitePaid, "upstream-token")
			if !errors.Is(err, ErrFluxAInvalidToken) {
				t.Fatalf("error = %v, want ErrFluxAInvalidToken", err)
			}
		})
	}
}

func TestFluxAVerifierRejectsTrailingJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"id":42,"username":"user"}} {"unexpected":true}`))
	}))
	t.Cleanup(server.Close)

	verifier := newTestFluxAIdentityVerifier(server.URL, server.URL, server.Client())
	_, err := verifier.Verify(context.Background(), FluxASitePaid, "upstream-token")
	if !errors.Is(err, ErrFluxAUnavailable) {
		t.Fatalf("error = %v, want ErrFluxAUnavailable", err)
	}
}

func TestFluxAVerifierDoesNotExposeResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("upstream-secret-diagnostic"))
	}))
	t.Cleanup(server.Close)

	verifier := newTestFluxAIdentityVerifier(server.URL, server.URL, server.Client())
	_, err := verifier.Verify(context.Background(), FluxASitePaid, "upstream-token")
	if !errors.Is(err, ErrFluxAUnavailable) {
		t.Fatalf("error = %v, want ErrFluxAUnavailable", err)
	}
	if strings.Contains(err.Error(), "upstream-secret-diagnostic") {
		t.Fatalf("error exposed response body: %v", err)
	}
}

func TestNewFluxAVerifierUsesTenSecondTimeout(t *testing.T) {
	verifier, ok := NewFluxAIdentityVerifier("https://paid.example", "https://free.example").(*fluxAIdentityVerifier)
	if !ok {
		t.Fatalf("verifier has type %T, want *fluxAIdentityVerifier", verifier)
	}
	if got := verifier.httpClient.Timeout; got != 10*time.Second {
		t.Fatalf("timeout = %v, want 10s", got)
	}
}

type staticFluxAVerifier struct {
	mu       sync.Mutex
	identity VerifiedFluxAIdentity
	err      error
	calls    int
	site     FluxASite
	token    string
}

func (v *staticFluxAVerifier) Verify(_ context.Context, site FluxASite, token string) (VerifiedFluxAIdentity, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.calls++
	v.site = site
	v.token = token
	return v.identity, v.err
}

func TestLoginWithFluxACreatesSiteScopedIdentityAndSession(t *testing.T) {
	mem := store.NewMemoryStore()
	verifier := &staticFluxAVerifier{identity: VerifiedFluxAIdentity{
		Subject:     "42",
		Username:    "shared-user",
		DisplayName: "FluxA User",
		Email:       "user@example.com",
		AvatarURL:   "https://example.com/avatar.png",
	}}
	service := NewService(ServiceDeps{
		Users:         mem,
		Identities:    mem,
		Sessions:      mem,
		ChatModels:    mem,
		FluxAVerifier: verifier,
	}, testAuthConfig())

	tokens, err := service.LoginWithFluxA(context.Background(), FluxASitePaid, "upstream-token")
	if err != nil {
		t.Fatalf("login with FluxA: %v", err)
	}
	if tokens.User.ID == "" || tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Fatalf("expected issued session, got %#v", tokens)
	}
	if tokens.User.Username != "fluxa-paid-42" {
		t.Fatalf("username = %q, want site-scoped username", tokens.User.Username)
	}
	if tokens.User.DisplayName != "FluxA User" || tokens.User.Email != "user@example.com" || tokens.User.AvatarURL != "https://example.com/avatar.png" {
		t.Fatalf("profile = %#v, want verified FluxA fields", tokens.User)
	}

	linkedUser, err := mem.GetUserByAuthIdentity(context.Background(), domain.AuthProviderFluxAPaid, "42")
	if err != nil {
		t.Fatalf("lookup FluxA auth identity: %v", err)
	}
	if linkedUser.ID != tokens.User.ID {
		t.Fatalf("identity user = %q, want %q", linkedUser.ID, tokens.User.ID)
	}
	models, err := mem.ListUserChatModels(context.Background(), tokens.User.ID)
	if err != nil {
		t.Fatalf("list chat defaults: %v", err)
	}
	if len(models) == 0 {
		t.Fatal("expected chat defaults to be created")
	}
	if verifier.calls != 1 || verifier.site != FluxASitePaid || verifier.token != "upstream-token" {
		t.Fatalf("verifier call = (%d, %q, %q), want (1, paid, upstream-token)", verifier.calls, verifier.site, verifier.token)
	}
}

func TestLoginWithFluxADoesNotMergeByUsernameOrEmail(t *testing.T) {
	mem := store.NewMemoryStore()
	existing := domain.User{
		ID:           "existing-user",
		Username:     "shared-user",
		DisplayName:  "Existing User",
		Email:        "shared@example.com",
		PasswordHash: "password-hash",
		CreatedAt:    time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), existing); err != nil {
		t.Fatalf("seed existing user: %v", err)
	}

	verifier := &staticFluxAVerifier{identity: VerifiedFluxAIdentity{
		Subject:   "91",
		Username:  "shared-user",
		Email:     "shared@example.com",
		AvatarURL: "https://example.com/new.png",
	}}
	service := NewService(ServiceDeps{
		Users:         mem,
		Identities:    mem,
		Sessions:      mem,
		ChatModels:    mem,
		FluxAVerifier: verifier,
	}, testAuthConfig())

	tokens, err := service.LoginWithFluxA(context.Background(), FluxASiteFree, "upstream-token")
	if err != nil {
		t.Fatalf("login with FluxA: %v", err)
	}
	if tokens.User.ID == existing.ID {
		t.Fatal("FluxA identity merged with username/email owner")
	}
	if tokens.User.Username != "fluxa-free-91" {
		t.Fatalf("username = %q, want fluxa-free-91", tokens.User.Username)
	}
	if tokens.User.DisplayName != "shared-user" {
		t.Fatalf("display name = %q, want FluxA username fallback", tokens.User.DisplayName)
	}
	linkedUser, err := mem.GetUserByAuthIdentity(context.Background(), domain.AuthProviderFluxAFree, "91")
	if err != nil {
		t.Fatalf("lookup FluxA auth identity: %v", err)
	}
	if linkedUser.ID != tokens.User.ID {
		t.Fatalf("identity user = %q, want %q", linkedUser.ID, tokens.User.ID)
	}
}

func TestLoginWithFluxAUpdatesExistingIdentityProfile(t *testing.T) {
	mem := store.NewMemoryStore()
	verifier := &staticFluxAVerifier{identity: VerifiedFluxAIdentity{
		Subject:  "7",
		Username: "first-name",
	}}
	service := NewService(ServiceDeps{
		Users:         mem,
		Identities:    mem,
		Sessions:      mem,
		ChatModels:    mem,
		FluxAVerifier: verifier,
	}, testAuthConfig())

	first, err := service.LoginWithFluxA(context.Background(), FluxASitePaid, "first-token")
	if err != nil {
		t.Fatalf("first login: %v", err)
	}
	verifier.identity = VerifiedFluxAIdentity{
		Subject:     "7",
		Username:    "second-name",
		DisplayName: "Updated Name",
		Email:       "updated@example.com",
		AvatarURL:   "https://example.com/updated.png",
	}

	second, err := service.LoginWithFluxA(context.Background(), FluxASitePaid, "second-token")
	if err != nil {
		t.Fatalf("second login: %v", err)
	}
	if second.User.ID != first.User.ID {
		t.Fatalf("second user = %q, want existing %q", second.User.ID, first.User.ID)
	}
	if second.User.DisplayName != "Updated Name" || second.User.Email != "updated@example.com" || second.User.AvatarURL != "https://example.com/updated.png" {
		t.Fatalf("updated profile = %#v", second.User)
	}
}

func TestLoginWithFluxARejectsUnknownSiteBeforeVerification(t *testing.T) {
	mem := store.NewMemoryStore()
	verifier := &staticFluxAVerifier{identity: VerifiedFluxAIdentity{Subject: "42", Username: "user"}}
	service := NewService(ServiceDeps{
		Users:         mem,
		Identities:    mem,
		Sessions:      mem,
		ChatModels:    mem,
		FluxAVerifier: verifier,
	}, testAuthConfig())

	_, err := service.LoginWithFluxA(context.Background(), FluxASite("other"), "upstream-token")
	if !errors.Is(err, ErrFluxAUnsupportedSite) {
		t.Fatalf("error = %v, want ErrFluxAUnsupportedSite", err)
	}
	if verifier.calls != 0 {
		t.Fatalf("verifier calls = %d, want 0", verifier.calls)
	}
}

func TestLoginWithFluxAKeepsSameSubjectSeparateAcrossSites(t *testing.T) {
	mem := store.NewMemoryStore()
	verifier := &staticFluxAVerifier{identity: VerifiedFluxAIdentity{
		Subject:  "42",
		Username: "same-user",
	}}
	service := NewService(ServiceDeps{
		Users:         mem,
		Identities:    mem,
		Sessions:      mem,
		ChatModels:    mem,
		FluxAVerifier: verifier,
	}, testAuthConfig())

	paid, err := service.LoginWithFluxA(context.Background(), FluxASitePaid, "paid-token")
	if err != nil {
		t.Fatalf("paid login: %v", err)
	}
	free, err := service.LoginWithFluxA(context.Background(), FluxASiteFree, "free-token")
	if err != nil {
		t.Fatalf("free login: %v", err)
	}

	if paid.User.ID == free.User.ID {
		t.Fatalf("paid and free user IDs both equal %q", paid.User.ID)
	}
	paidIdentity, err := mem.GetUserByAuthIdentity(context.Background(), domain.AuthProviderFluxAPaid, "42")
	if err != nil {
		t.Fatalf("lookup paid identity: %v", err)
	}
	freeIdentity, err := mem.GetUserByAuthIdentity(context.Background(), domain.AuthProviderFluxAFree, "42")
	if err != nil {
		t.Fatalf("lookup free identity: %v", err)
	}
	if paidIdentity.ID != paid.User.ID || freeIdentity.ID != free.User.ID {
		t.Fatalf("identity users = (%q, %q), want (%q, %q)", paidIdentity.ID, freeIdentity.ID, paid.User.ID, free.User.ID)
	}
}
