package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/auth"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

type ctxKey string

const userKey ctxKey = "user"

func (h *Handler) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer"))
		if token == "" {
			writeUnauthorized(w, "missing bearer token")
			return
		}
		user, err := h.authService.Authenticate(token)
		if err != nil {
			writeUnauthorized(w, "invalid access token")
			return
		}
		ctx := context.WithValue(r.Context(), userKey, user)
		next(w, r.WithContext(ctx))
	}
}

func currentUser(ctx context.Context) domain.User {
	user, _ := ctx.Value(userKey).(domain.User)
	return user
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeInvalidRequestBody(w)
		return
	}
	tokens, err := h.authService.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			writeUnauthorized(w, "invalid username or password")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeSessionPayload(w, tokens)
}

func (h *Handler) loginWithGoogle(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDToken string `json:"idToken"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeInvalidRequestBody(w)
		return
	}
	if strings.TrimSpace(req.IDToken) == "" {
		writeValidationError(w, "idToken is required", []string{"idToken"})
		return
	}
	tokens, err := h.authService.LoginWithGoogle(r.Context(), req.IDToken)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrGoogleAuthDisabled):
			writeError(w, http.StatusServiceUnavailable, "google auth is not configured")
		case errors.Is(err, auth.ErrGoogleKeysUnavailable):
			writeError(w, http.StatusServiceUnavailable, "backend cannot reach Google public keys")
		case errors.Is(err, auth.ErrInvalidGoogleToken):
			writeUnauthorized(w, "invalid google id token")
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeSessionPayload(w, tokens)
}

func (h *Handler) loginWithMicrosoft(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDToken string `json:"idToken"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeInvalidRequestBody(w)
		return
	}
	if strings.TrimSpace(req.IDToken) == "" {
		writeValidationError(w, "idToken is required", []string{"idToken"})
		return
	}
	tokens, err := h.authService.LoginWithMicrosoft(r.Context(), req.IDToken)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrMicrosoftAuthDisabled):
			writeError(w, http.StatusServiceUnavailable, "microsoft auth is not configured")
		case errors.Is(err, auth.ErrMicrosoftIdentityUnavailable):
			writeError(w, http.StatusServiceUnavailable, "backend cannot reach Microsoft identity services")
		case errors.Is(err, auth.ErrInvalidMicrosoftToken):
			writeUnauthorized(w, "invalid microsoft id token")
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeSessionPayload(w, tokens)
}

func writeSessionPayload(w http.ResponseWriter, tokens auth.TokenPair) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"user": map[string]interface{}{
			"id":          tokens.User.ID,
			"username":    tokens.User.Username,
			"displayName": tokens.User.DisplayName,
			"email":       tokens.User.Email,
			"avatarUrl":   tokens.User.AvatarURL,
		},
		"accessToken":  tokens.AccessToken,
		"refreshToken": tokens.RefreshToken,
		"expiresIn":    int(tokens.AccessTTL.Seconds()),
	})
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refreshToken"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeInvalidRequestBody(w)
		return
	}
	tokens, err := h.authService.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		writeUnauthorized(w, "invalid refresh token")
		return
	}
	writeSessionPayload(w, tokens)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refreshToken"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeInvalidRequestBody(w)
		return
	}
	if err := h.authService.Logout(r.Context(), req.RefreshToken); err != nil {
		writeUnauthorized(w, "invalid refresh token")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":          user.ID,
		"username":    user.Username,
		"displayName": user.DisplayName,
		"email":       user.Email,
		"avatarUrl":   user.AvatarURL,
	})
}
