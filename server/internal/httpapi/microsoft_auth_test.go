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

type staticMicrosoftVerifier struct {
	identity auth.VerifiedMicrosoftIdentity
	err      error
}

func (v staticMicrosoftVerifier) VerifyIDToken(context.Context, string) (auth.VerifiedMicrosoftIdentity, error) {
	return v.identity, v.err
}

func TestLoginWithMicrosoftReturnsSessionPayload(t *testing.T) {
	mem := store.NewMemoryStore()
	authService := auth.NewServiceWithMicrosoftVerifier(mem, config.Config{
		JWTSecret:  "test-secret",
		AccessTTL:  30 * time.Minute,
		RefreshTTL: 24 * time.Hour,
	}, staticMicrosoftVerifier{
		identity: auth.VerifiedMicrosoftIdentity{
			Subject:       "microsoft-subject-1",
			Email:         "haibinchenleo@outlook.com",
			EmailVerified: true,
			DisplayName:   "Haibin Chen",
			AvatarURL:     "https://example.com/avatar.png",
		},
	})
	handler := &Handler{authService: authService}

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/microsoft", strings.NewReader(`{"idToken":"token"}`))
	rec := httptest.NewRecorder()

	handler.loginWithMicrosoft(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	user, _ := payload["user"].(map[string]interface{})
	if user["email"] != "haibinchenleo@outlook.com" {
		t.Fatalf("expected email in payload, got %#v", user["email"])
	}
	if payload["accessToken"] == "" || payload["refreshToken"] == "" {
		t.Fatalf("expected session tokens, got %#v", payload)
	}
}
