package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/store"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/auth"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/config"
)

type staticWeChatExchanger struct {
	identity auth.VerifiedWeChatIdentity
	err      error
}

func (e staticWeChatExchanger) ExchangeCode(context.Context, string) (auth.VerifiedWeChatIdentity, error) {
	return e.identity, e.err
}

func newWeChatAuthHandler(identity auth.VerifiedWeChatIdentity) *Handler {
	mem := store.NewMemoryStore()
	authService := auth.NewServiceWithWeChatExchanger(auth.ServiceDeps{
		Users:      mem,
		Identities: mem,
		Sessions:   mem,
		ChatModels: mem,
	}, config.Config{
		JWTSecret:  "test-secret",
		AccessTTL:  30 * time.Minute,
		RefreshTTL: 24 * time.Hour,
	}, staticWeChatExchanger{identity: identity})
	return &Handler{authService: authService}
}

func TestLoginWithWeChatReturnsSessionPayload(t *testing.T) {
	handler := newWeChatAuthHandler(auth.VerifiedWeChatIdentity{
		OpenID:    "wechat-openid-1",
		UnionID:   "wechat-unionid-1",
		Nickname:  "海滨",
		AvatarURL: "https://example.com/wechat-avatar.png",
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/wechat", strings.NewReader(`{"code":"wechat-code"}`))
	rec := httptest.NewRecorder()

	handler.loginWithWeChat(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	user, _ := payload["user"].(map[string]interface{})
	if user["displayName"] != "海滨" {
		t.Fatalf("expected display name in payload, got %#v", user["displayName"])
	}
	if user["avatarUrl"] != "https://example.com/wechat-avatar.png" {
		t.Fatalf("expected avatar in payload, got %#v", user["avatarUrl"])
	}
	if payload["accessToken"] == "" || payload["refreshToken"] == "" {
		t.Fatalf("expected session tokens, got %#v", payload)
	}
}

func TestLoginWithWeChatRequiresCode(t *testing.T) {
	handler := newWeChatAuthHandler(auth.VerifiedWeChatIdentity{
		OpenID:  "wechat-openid-1",
		UnionID: "wechat-unionid-1",
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/wechat", strings.NewReader(`{"code":"   "}`))
	rec := httptest.NewRecorder()

	handler.loginWithWeChat(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload apiErrorPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.Code != errorCodeValidationFailed || payload.Field != "code" {
		t.Fatalf("expected code validation error, got %#v", payload)
	}
}
