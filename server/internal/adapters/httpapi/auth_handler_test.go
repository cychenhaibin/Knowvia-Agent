package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequireAuthReturnsUnauthorizedCodeOnMissingBearerToken(t *testing.T) {
	handler := &Handler{}
	called := false
	wrapped := handler.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	rec := httptest.NewRecorder()

	wrapped(rec, req)

	if called {
		t.Fatalf("expected wrapped handler not to be called")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
	payload := decodeAPIError(t, rec.Body.Bytes())
	if payload.Code != errorCodeUnauthorized {
		t.Fatalf("expected code %s, got %s", errorCodeUnauthorized, payload.Code)
	}
}

func TestLoginReturnsInvalidRequestBodyCode(t *testing.T) {
	handler := &Handler{}
	req := httptest.NewRequest(http.MethodPost, "/v1/login", strings.NewReader("{"))
	rec := httptest.NewRecorder()

	handler.login(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	payload := decodeAPIError(t, rec.Body.Bytes())
	if payload.Code != errorCodeInvalidRequestBody {
		t.Fatalf("expected code %s, got %s", errorCodeInvalidRequestBody, payload.Code)
	}
}
