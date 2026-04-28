package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/auth"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/config"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/store"
)

type staticGoogleVerifier struct {
	identity auth.VerifiedGoogleIdentity
	err      error
}

func (v staticGoogleVerifier) VerifyIDToken(context.Context, string) (auth.VerifiedGoogleIdentity, error) {
	return v.identity, v.err
}

func TestLoginWithGoogleReturnsSessionPayload(t *testing.T) {
	mem := store.NewMemoryStore()
	authService := auth.NewServiceWithGoogleVerifier(mem, config.Config{
		JWTSecret:  "test-secret",
		AccessTTL:  30 * time.Minute,
		RefreshTTL: 24 * time.Hour,
	}, staticGoogleVerifier{
		identity: auth.VerifiedGoogleIdentity{
			Subject:       "google-subject-1",
			Email:         "haibinchenleo@gmail.com",
			EmailVerified: true,
			DisplayName:   "Haibin Chen",
			AvatarURL:     "https://example.com/avatar.png",
		},
	})
	handler := &Handler{authService: authService}

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/google", strings.NewReader(`{"idToken":"token"}`))
	rec := httptest.NewRecorder()

	handler.loginWithGoogle(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	user, _ := payload["user"].(map[string]interface{})
	if user["email"] != "haibinchenleo@gmail.com" {
		t.Fatalf("expected email in payload, got %#v", user["email"])
	}
	if payload["accessToken"] == "" || payload["refreshToken"] == "" {
		t.Fatalf("expected session tokens, got %#v", payload)
	}
}
