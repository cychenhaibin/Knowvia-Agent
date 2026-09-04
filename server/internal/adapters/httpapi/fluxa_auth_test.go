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

func TestFluxAExchangeReturnsSessionPayload(t *testing.T) {
	verifier := &recordingFluxAVerifier{identity: auth.VerifiedFluxAIdentity{
		Subject:     "42",
		Username:    "fluxa-user",
		DisplayName: "FluxA User",
		Email:       "user@example.com",
	}}
	router := newFluxATestRouter(verifier)

	rec := performFluxAExchange(router, `{"site":"  paid  ","accessToken":"  upstream-secret-token  "}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var payload sessionDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode session payload: %v", err)
	}
	if payload.User.Email != "user@example.com" || payload.User.Username != "fluxa-paid-42" {
		t.Fatalf("user = %#v, want paid FluxA user", payload.User)
	}
	if payload.AccessToken == "" || payload.RefreshToken == "" {
		t.Fatalf("session = %#v, want issued tokens", payload)
	}
	if verifier.calls != 1 || verifier.site != auth.FluxASitePaid || verifier.token != "upstream-secret-token" {
		t.Fatalf("verifier call = (%d, %q, %q), want (1, paid, upstream-secret-token)", verifier.calls, verifier.site, verifier.token)
	}
	if strings.Contains(rec.Body.String(), "upstream-secret-token") {
		t.Fatal("response echoed the upstream access token")
	}
}

func TestFluxAExchangeRejectsMissingFieldsWithoutVerification(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantField string
	}{
		{name: "missing site", body: `{"accessToken":"upstream-secret-token"}`, wantField: "site"},
		{name: "blank site", body: `{"site":"  ","accessToken":"upstream-secret-token"}`, wantField: "site"},
		{name: "missing access token", body: `{"site":"paid"}`, wantField: "accessToken"},
		{name: "blank access token", body: `{"site":"paid","accessToken":"  "}`, wantField: "accessToken"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verifier := &recordingFluxAVerifier{}
			rec := performFluxAExchange(newFluxATestRouter(verifier), tt.body)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
			}
			var payload apiErrorPayload
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode error payload: %v", err)
			}
			if payload.Code != errorCodeValidationFailed || payload.Field != tt.wantField {
				t.Fatalf("error = %#v, want validation error for %q", payload, tt.wantField)
			}
			if verifier.calls != 0 {
				t.Fatalf("verifier calls = %d, want 0", verifier.calls)
			}
			if strings.Contains(rec.Body.String(), "upstream-secret-token") {
				t.Fatal("response echoed the upstream access token")
			}
		})
	}
}

func TestFluxAExchangeRejectsUnknownSiteWithoutVerification(t *testing.T) {
	verifier := &recordingFluxAVerifier{}
	rec := performFluxAExchange(newFluxATestRouter(verifier), `{"site":"https://attacker.example","accessToken":"upstream-secret-token"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	var payload apiErrorPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error payload: %v", err)
	}
	if payload.Code != errorCodeValidationFailed || payload.Field != "site" {
		t.Fatalf("error = %#v, want site validation error", payload)
	}
	if verifier.calls != 0 {
		t.Fatalf("verifier calls = %d, want 0", verifier.calls)
	}
	if strings.Contains(rec.Body.String(), "upstream-secret-token") {
		t.Fatal("response echoed the upstream access token")
	}
}

func TestFluxAExchangeMapsServiceErrorsToSafeResponses(t *testing.T) {
	tests := []struct {
		name        string
		serviceErr  error
		wantStatus  int
		wantCode    string
		wantMessage string
	}{
		{name: "invalid token", serviceErr: auth.ErrFluxAInvalidToken, wantStatus: http.StatusUnauthorized, wantCode: errorCodeUnauthorized, wantMessage: "invalid FluxA access token"},
		{name: "upstream unavailable", serviceErr: auth.ErrFluxAUnavailable, wantStatus: http.StatusServiceUnavailable, wantCode: errorCodeServiceUnavailable, wantMessage: "FluxA identity service is unavailable"},
		{name: "unexpected failure", serviceErr: errors.New("database diagnostic with upstream-secret-token"), wantStatus: http.StatusInternalServerError, wantCode: errorCodeInternalError, wantMessage: "FluxA authentication failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verifier := &recordingFluxAVerifier{err: tt.serviceErr}
			rec := performFluxAExchange(newFluxATestRouter(verifier), `{"site":"paid","accessToken":"upstream-secret-token"}`)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			var payload apiErrorPayload
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode error payload: %v", err)
			}
			if payload.Code != tt.wantCode || payload.Error != tt.wantMessage {
				t.Fatalf("error = %#v, want code %q message %q", payload, tt.wantCode, tt.wantMessage)
			}
			if strings.Contains(rec.Body.String(), "upstream-secret-token") {
				t.Fatal("response exposed the upstream access token")
			}
		})
	}
}

func TestFluxAExchangeKeepsPaidAndFreeIdentitiesSeparate(t *testing.T) {
	verifier := &recordingFluxAVerifier{identity: auth.VerifiedFluxAIdentity{Subject: "42", Username: "same-user"}}
	router := newFluxATestRouter(verifier)

	paid := performFluxAExchange(router, `{"site":"paid","accessToken":"paid-token"}`)
	free := performFluxAExchange(router, `{"site":"free","accessToken":"free-token"}`)
	if paid.Code != http.StatusOK || free.Code != http.StatusOK {
		t.Fatalf("statuses = (%d, %d), want (200, 200): paid=%s free=%s", paid.Code, free.Code, paid.Body.String(), free.Body.String())
	}

	var paidSession, freeSession sessionDTO
	if err := json.Unmarshal(paid.Body.Bytes(), &paidSession); err != nil {
		t.Fatalf("decode paid session: %v", err)
	}
	if err := json.Unmarshal(free.Body.Bytes(), &freeSession); err != nil {
		t.Fatalf("decode free session: %v", err)
	}
	if paidSession.User.ID == freeSession.User.ID {
		t.Fatalf("paid and free user IDs both equal %q", paidSession.User.ID)
	}
	if paidSession.User.Username != "fluxa-paid-42" || freeSession.User.Username != "fluxa-free-42" {
		t.Fatalf("usernames = (%q, %q), want site-scoped usernames", paidSession.User.Username, freeSession.User.Username)
	}
}

func newFluxATestRouter(verifier auth.FluxAIdentityVerifier) http.Handler {
	mem := store.NewMemoryStore()
	authService := auth.NewService(auth.ServiceDeps{
		Users:         mem,
		Identities:    mem,
		Sessions:      mem,
		ChatModels:    mem,
		FluxAVerifier: verifier,
	}, config.Config{
		JWTSecret:  "test-secret",
		AccessTTL:  30 * time.Minute,
		RefreshTTL: 24 * time.Hour,
	})
	return NewRouter(authService, nil, nil, nil, nil, nil)
}

func performFluxAExchange(router http.Handler, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/fluxa", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}
