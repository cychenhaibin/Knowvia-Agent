package httpapi

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/auth"
)

const (
	defaultFluxALoginMaxAttempts = 5
	defaultFluxALoginWindow      = time.Minute
	defaultFluxALoginMaxEntries  = 4096
)

type fluxALoginLimitKey struct {
	sourceIP string
	site     auth.FluxASite
	username string
}

type fluxALoginLimitEntry struct {
	attempts int
	resetAt  time.Time
}

type fluxALoginLimiter struct {
	mu          sync.Mutex
	entries     map[fluxALoginLimitKey]fluxALoginLimitEntry
	maxAttempts int
	window      time.Duration
	maxEntries  int
	now         func() time.Time
}

func newDefaultFluxALoginLimiter() *fluxALoginLimiter {
	return newFluxALoginLimiter(
		defaultFluxALoginMaxAttempts,
		defaultFluxALoginWindow,
		defaultFluxALoginMaxEntries,
		time.Now,
	)
}

func newFluxALoginLimiter(maxAttempts int, window time.Duration, maxEntries int, now func() time.Time) *fluxALoginLimiter {
	return &fluxALoginLimiter{
		entries:     make(map[fluxALoginLimitKey]fluxALoginLimitEntry),
		maxAttempts: maxAttempts,
		window:      window,
		maxEntries:  maxEntries,
		now:         now,
	}
}

func (l *fluxALoginLimiter) allow(key fluxALoginLimitKey) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	if entry, ok := l.entries[key]; ok {
		if now.Before(entry.resetAt) {
			if entry.attempts >= l.maxAttempts {
				return false, entry.resetAt.Sub(now)
			}
			entry.attempts++
			l.entries[key] = entry
			return true, 0
		}
		delete(l.entries, key)
	}

	l.removeExpired(now)
	if len(l.entries) >= l.maxEntries {
		l.removeOldest()
	}
	l.entries[key] = fluxALoginLimitEntry{attempts: 1, resetAt: now.Add(l.window)}
	return true, 0
}

func (l *fluxALoginLimiter) removeExpired(now time.Time) {
	for key, entry := range l.entries {
		if !now.Before(entry.resetAt) {
			delete(l.entries, key)
		}
	}
}

func (l *fluxALoginLimiter) removeOldest() {
	var oldestKey fluxALoginLimitKey
	var oldestReset time.Time
	for key, entry := range l.entries {
		if oldestReset.IsZero() || entry.resetAt.Before(oldestReset) {
			oldestKey = key
			oldestReset = entry.resetAt
		}
	}
	if !oldestReset.IsZero() {
		delete(l.entries, oldestKey)
	}
}

func fluxALoginKey(r *http.Request, site auth.FluxASite, username string) fluxALoginLimitKey {
	sourceIP := strings.TrimSpace(formatRemoteAddr(r.RemoteAddr))
	if sourceIP == "" {
		sourceIP = "unknown"
	}
	return fluxALoginLimitKey{
		sourceIP: sourceIP,
		site:     site,
		username: strings.ToLower(strings.TrimSpace(username)),
	}
}

func writeFluxALoginRateLimited(w http.ResponseWriter, retryAfter time.Duration) {
	seconds := int64((retryAfter + time.Second - 1) / time.Second)
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.FormatInt(seconds, 10))
	writeErrorCode(w, http.StatusTooManyRequests, errorCodeRateLimited, "too many FluxA login attempts")
}
