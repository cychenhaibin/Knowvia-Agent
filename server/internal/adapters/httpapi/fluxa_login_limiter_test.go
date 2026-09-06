package httpapi

import (
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/auth"
)

func TestFluxALoginRateLimitsSourceSiteAndNormalizedUsername(t *testing.T) {
	authenticator := &recordingFluxACredentialAuthenticator{err: auth.ErrFluxAInvalidCredentials}
	router := newFluxATestRouter(authenticator, &recordingFluxAVerifier{})

	for attempt := 1; attempt <= 5; attempt++ {
		username := "fluxa-user"
		if attempt == 1 {
			username = "  FLUXA-USER  "
		}
		rec := performFluxALoginFrom(router, `{"site":"paid","username":"`+username+`","password":"raw-password"}`, "203.0.113.10:4321")
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status = %d, want 401: %s", attempt, rec.Code, rec.Body.String())
		}
	}

	blocked := performFluxALoginFrom(router, `{"site":"paid","username":"fluxa-user","password":"raw-password"}`, "203.0.113.10:9876")
	if blocked.Code != http.StatusTooManyRequests {
		t.Fatalf("blocked status = %d, want 429: %s", blocked.Code, blocked.Body.String())
	}
	assertFluxAError(t, blocked, errorCodeRateLimited, "", "too many FluxA login attempts")
	if blocked.Header().Get("Retry-After") == "" {
		t.Fatal("rate-limited response omitted Retry-After")
	}
	if calls, _, _, _ := authenticator.snapshot(); calls != 5 {
		t.Fatalf("authenticator calls = %d, want 5", calls)
	}
	if body := blocked.Body.String(); containsAny(body, "fluxa-user", "raw-password") {
		t.Fatalf("rate-limit response exposed credentials: %s", body)
	}

	independent := []struct {
		remoteAddr string
		site       string
		username   string
	}{
		{remoteAddr: "203.0.113.11:4321", site: "paid", username: "fluxa-user"},
		{remoteAddr: "203.0.113.10:4321", site: "free", username: "fluxa-user"},
		{remoteAddr: "203.0.113.10:4321", site: "paid", username: "other-user"},
	}
	for _, tt := range independent {
		rec := performFluxALoginFrom(router, `{"site":"`+tt.site+`","username":"`+tt.username+`","password":"raw-password"}`, tt.remoteAddr)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("independent key %#v status = %d, want 401: %s", tt, rec.Code, rec.Body.String())
		}
	}
}

func TestFluxALoginLimiterExpiresAttemptsAndBoundsStorage(t *testing.T) {
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	limiter := newFluxALoginLimiter(2, time.Minute, 2, func() time.Time { return now })
	key := fluxALoginLimitKey{sourceIP: "203.0.113.10", site: auth.FluxASitePaid, username: "user"}

	if allowed, _ := limiter.allow(key); !allowed {
		t.Fatal("first attempt was unexpectedly limited")
	}
	if allowed, _ := limiter.allow(key); !allowed {
		t.Fatal("second attempt was unexpectedly limited")
	}
	if allowed, _ := limiter.allow(key); allowed {
		t.Fatal("third attempt was allowed before window expiry")
	}
	now = now.Add(time.Minute)
	if allowed, _ := limiter.allow(key); !allowed {
		t.Fatal("attempt remained limited after window expiry")
	}

	limiter.allow(fluxALoginLimitKey{sourceIP: "203.0.113.11", site: auth.FluxASitePaid, username: "user"})
	limiter.allow(fluxALoginLimitKey{sourceIP: "203.0.113.12", site: auth.FluxASitePaid, username: "user"})
	if got := len(limiter.entries); got > 2 {
		t.Fatalf("limiter retained %d entries, want at most 2", got)
	}
}

func TestFluxALoginLimiterIsConcurrencySafe(t *testing.T) {
	limiter := newFluxALoginLimiter(5, time.Minute, 100, time.Now)
	key := fluxALoginLimitKey{sourceIP: "203.0.113.10", site: auth.FluxASitePaid, username: "user"}
	var allowed atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ok, _ := limiter.allow(key); ok {
				allowed.Add(1)
			}
		}()
	}
	wg.Wait()
	if got := allowed.Load(); got != 5 {
		t.Fatalf("allowed attempts = %d, want 5", got)
	}
}

func containsAny(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if candidate != "" && strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}
