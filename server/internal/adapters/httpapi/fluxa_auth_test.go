package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/store"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/auth"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/config"
)

type recordingFluxACredentialAuthenticator struct {
	mu       sync.Mutex
	token    string
	err      error
	calls    int
	site     auth.FluxASite
	username string
	password string
}

func (a *recordingFluxACredentialAuthenticator) Login(_ context.Context, site auth.FluxASite, username, password string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls++
	a.site = site
	a.username = username
	a.password = password
	return a.token, a.err
}

func (a *recordingFluxACredentialAuthenticator) snapshot() (calls int, site auth.FluxASite, username, password string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.calls, a.site, a.username, a.password
}

type recordingFluxAVerifier struct {
	mu       sync.Mutex
	identity auth.VerifiedFluxAIdentity
	err      error
	calls    int
	site     auth.FluxASite
	token    string
}

func (v *recordingFluxAVerifier) Verify(_ context.Context, site auth.FluxASite, token string) (auth.VerifiedFluxAIdentity, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.calls++
	v.site = site
	v.token = token
	return v.identity, v.err
}

func (v *recordingFluxAVerifier) snapshot() (calls int, site auth.FluxASite, token string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.calls, v.site, v.token
}

func TestFluxALoginReturnsSessionPayloadWithoutInternalToken(t *testing.T) {
	const internalToken = "internal-upstream-token"
	authenticator := &recordingFluxACredentialAuthenticator{token: internalToken}
	verifier := &recordingFluxAVerifier{identity: auth.VerifiedFluxAIdentity{
		Subject:     "42",
		Username:    "fluxa-user",
		DisplayName: "FluxA User",
		Email:       "user@example.com",
		Group:       "gpt_td",
	}}

	rec := performFluxALogin(newFluxATestRouter(authenticator, verifier), `{"site":"  paid  ","username":"  fluxa-user  ","password":"  raw-password\t "}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var payload sessionDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode session payload: %v", err)
	}
	if payload.User.Email != "user@example.com" || payload.User.Username != "fluxa-user" || payload.User.FluxAGroup != "gpt_td" {
		t.Fatalf("user = %#v, want paid FluxA user", payload.User)
	}
	if payload.User.FluxASite == nil || *payload.User.FluxASite != auth.FluxASitePaid {
		t.Fatalf("fluxaSite = %#v, want %q", payload.User.FluxASite, auth.FluxASitePaid)
	}
	if payload.AccessToken == "" || payload.RefreshToken == "" {
		t.Fatalf("session = %#v, want issued tokens", payload)
	}
	if strings.Contains(rec.Body.String(), internalToken) {
		t.Fatal("response echoed the internal FluxA token")
	}
	if strings.Contains(rec.Body.String(), "raw-password") {
		t.Fatal("response echoed the password")
	}

	if calls, site, username, password := authenticator.snapshot(); calls != 1 || site != auth.FluxASitePaid || username != "fluxa-user" || password != "  raw-password\t " {
		t.Fatalf("authenticator call = (%d, %q, %q, %q), want password preserved byte-for-byte", calls, site, username, password)
	}
	if calls, site, token := verifier.snapshot(); calls != 1 || site != auth.FluxASitePaid || token != internalToken {
		t.Fatalf("verifier call = (%d, %q, %q), want (1, paid, %q)", calls, site, token, internalToken)
	}
}

func TestMapSessionOmitsFluxASiteForOrdinarySessions(t *testing.T) {
	payload := mapSession(auth.TokenPair{})

	if payload.User.FluxASite != nil {
		t.Fatalf("fluxaSite = %#v, want nil", payload.User.FluxASite)
	}
}

func TestFluxALoginRejectsLegacyAccessTokenAlongsideValidCredentials(t *testing.T) {
	authenticator := &recordingFluxACredentialAuthenticator{token: "internal-upstream-token"}
	rec := performFluxALogin(newFluxATestRouter(authenticator, &recordingFluxAVerifier{}), `{"site":"paid","username":"fluxa-user","password":"raw-password","accessToken":"legacy-token"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	assertFluxAError(t, rec, errorCodeInvalidRequestBody, "", "invalid request body")
	if calls, _, _, _ := authenticator.snapshot(); calls != 0 {
		t.Fatalf("authenticator calls = %d, want 0", calls)
	}
	if strings.Contains(rec.Body.String(), "raw-password") || strings.Contains(rec.Body.String(), "legacy-token") {
		t.Fatal("response exposed a submitted secret")
	}
}

func TestFluxALoginRejectsMissingCredentials(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantField string
	}{
		{name: "missing username", body: `{"site":"paid","password":"raw-password"}`, wantField: "username"},
		{name: "blank username", body: `{"site":"paid","username":"  ","password":"raw-password"}`, wantField: "username"},
		{name: "missing password", body: `{"site":"paid","username":"fluxa-user"}`, wantField: "password"},
		{name: "blank password", body: `{"site":"paid","username":"fluxa-user","password":"  "}`, wantField: "password"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authenticator := &recordingFluxACredentialAuthenticator{token: "internal-upstream-token"}
			rec := performFluxALogin(newFluxATestRouter(authenticator, &recordingFluxAVerifier{}), tt.body)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
			}
			assertFluxAError(t, rec, errorCodeValidationFailed, tt.wantField, "")
			if calls, _, _, _ := authenticator.snapshot(); calls != 0 {
				t.Fatalf("authenticator calls = %d, want 0", calls)
			}
			if strings.Contains(rec.Body.String(), "raw-password") || strings.Contains(rec.Body.String(), "legacy-token") {
				t.Fatal("response exposed a submitted secret")
			}
		})
	}
}

func TestFluxALoginRejectsOversizedRequestBodyWithoutAuthenticating(t *testing.T) {
	authenticator := &recordingFluxACredentialAuthenticator{token: "internal-upstream-token"}
	body := `{"site":"paid","username":"fluxa-user","password":"` + strings.Repeat("x", 20<<10) + `"}`
	rec := performFluxALogin(newFluxATestRouter(authenticator, &recordingFluxAVerifier{}), body)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	assertFluxAError(t, rec, errorCodeInvalidRequestBody, "", "invalid request body")
	if calls, _, _, _ := authenticator.snapshot(); calls != 0 {
		t.Fatalf("authenticator calls = %d, want 0", calls)
	}
}

func TestFluxALoginRejectsCredentialFieldsOverByteLimits(t *testing.T) {
	tests := []struct {
		name      string
		username  string
		password  string
		wantField string
	}{
		{name: "username", username: strings.Repeat("界", 86), password: "password", wantField: "username"},
		{name: "password", username: "fluxa-user", password: strings.Repeat("界", 1366), wantField: "password"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authenticator := &recordingFluxACredentialAuthenticator{token: "internal-upstream-token"}
			body, err := json.Marshal(map[string]string{
				"site":     "paid",
				"username": tt.username,
				"password": tt.password,
			})
			if err != nil {
				t.Fatalf("marshal request: %v", err)
			}
			rec := performFluxALogin(newFluxATestRouter(authenticator, &recordingFluxAVerifier{}), string(body))

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
			}
			assertFluxAError(t, rec, errorCodeValidationFailed, tt.wantField, "")
			if calls, _, _, _ := authenticator.snapshot(); calls != 0 {
				t.Fatalf("authenticator calls = %d, want 0", calls)
			}
		})
	}
}

func TestFluxALoginRejectsUnknownSiteWithoutAuthenticating(t *testing.T) {
	authenticator := &recordingFluxACredentialAuthenticator{token: "internal-upstream-token"}
	rec := performFluxALogin(newFluxATestRouter(authenticator, &recordingFluxAVerifier{}), `{"site":"https://attacker.example","username":"fluxa-user","password":"raw-password"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	assertFluxAError(t, rec, errorCodeValidationFailed, "site", "")
	if calls, _, _, _ := authenticator.snapshot(); calls != 0 {
		t.Fatalf("authenticator calls = %d, want 0", calls)
	}
	if strings.Contains(rec.Body.String(), "raw-password") {
		t.Fatal("response exposed the password")
	}
}

func TestFluxALoginMapsCredentialErrorsToSafeResponses(t *testing.T) {
	tests := []struct {
		name        string
		serviceErr  error
		wantStatus  int
		wantCode    string
		wantMessage string
	}{
		{name: "invalid credentials", serviceErr: auth.ErrFluxAInvalidCredentials, wantStatus: http.StatusUnauthorized, wantCode: errorCodeUnauthorized, wantMessage: "invalid FluxA credentials"},
		{name: "two factor required", serviceErr: auth.ErrFluxA2FARequired, wantStatus: http.StatusUnauthorized, wantCode: errorCodeUnauthorized, wantMessage: "complete two-factor authentication on the selected FluxA site"},
		{name: "upstream unavailable", serviceErr: auth.ErrFluxAUnavailable, wantStatus: http.StatusServiceUnavailable, wantCode: errorCodeServiceUnavailable, wantMessage: "FluxA identity service is unavailable"},
		{name: "unexpected failure", serviceErr: errors.New("database diagnostic with raw-password"), wantStatus: http.StatusInternalServerError, wantCode: errorCodeInternalError, wantMessage: "FluxA authentication failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authenticator := &recordingFluxACredentialAuthenticator{err: tt.serviceErr}
			rec := performFluxALogin(newFluxATestRouter(authenticator, &recordingFluxAVerifier{}), `{"site":"paid","username":"fluxa-user","password":"raw-password"}`)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			assertFluxAError(t, rec, tt.wantCode, "", tt.wantMessage)
			if strings.Contains(rec.Body.String(), "raw-password") {
				t.Fatal("response exposed the password")
			}
		})
	}
}

func assertFluxAError(t *testing.T, rec *httptest.ResponseRecorder, wantCode, wantField, wantMessage string) {
	t.Helper()
	var payload apiErrorPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error payload: %v", err)
	}
	if payload.Code != wantCode || payload.Field != wantField || (wantMessage != "" && payload.Error != wantMessage) {
		t.Fatalf("error = %#v, want code %q field %q message %q", payload, wantCode, wantField, wantMessage)
	}
}

func newFluxATestRouter(authenticator auth.FluxACredentialAuthenticator, verifier auth.FluxAIdentityVerifier) http.Handler {
	mem := store.NewMemoryStore()
	authService := auth.NewService(auth.ServiceDeps{
		Users:              mem,
		Identities:         mem,
		Sessions:           mem,
		ChatModels:         mem,
		FluxACredentials:   mem,
		FluxAVerifier:      verifier,
		FluxAAuthenticator: authenticator,
	}, config.Config{
		JWTSecret:           "test-secret",
		AccessTTL:           30 * time.Minute,
		RefreshTTL:          24 * time.Hour,
		FluxACredentialsKey: make([]byte, 32),
	})
	return NewRouter(authService, nil, nil, nil, nil, nil)
}

func performFluxALogin(router http.Handler, body string) *httptest.ResponseRecorder {
	return performFluxALoginFrom(router, body, "192.0.2.1:1234")
}

func performFluxALoginFrom(router http.Handler, body, remoteAddr string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/fluxa", strings.NewReader(body))
	req.RemoteAddr = remoteAddr
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}
