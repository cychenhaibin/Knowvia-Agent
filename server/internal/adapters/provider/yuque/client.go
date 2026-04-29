package yuque

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Client fetches Yuque upstream payloads in Go so Python can focus on parsing,
// chunking, embedding and indexing.
type Client struct {
	httpClient *http.Client
	baseURL    string
	mu         sync.Mutex
	lastSlot   time.Time
}

type scopedGate struct {
	lock     sync.Mutex
	refCount int
}

var (
	yuqueGateGuard sync.Mutex
	yuqueGates     = map[string]*scopedGate{}
)

type DocMeta struct {
	ID        int64  `json:"id"`
	Slug      string `json:"slug"`
	Title     string `json:"title"`
	UpdatedAt string `json:"updated_at"`
}

type APIError struct {
	StatusCode int
	Message    string
	RetryAfter time.Duration
}

func (e *APIError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("yuque api returned %d", e.StatusCode)
	}
	return fmt.Sprintf("yuque api returned %d: %s", e.StatusCode, e.Message)
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    "https://www.yuque.com/api/v2",
	}
}

func (c *Client) get(ctx context.Context, token, endpoint string, target interface{}) error {
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		if err := c.waitTurn(ctx); err != nil {
			return err
		}
		err := c.getOnce(ctx, token, endpoint, target)
		if err == nil {
			return nil
		}
		lastErr = err

		var apiErr *APIError
		if !errors.As(err, &apiErr) || !isRetriableStatus(apiErr.StatusCode) || attempt == 3 {
			return err
		}

		delay := apiErr.RetryAfter
		if delay <= 0 {
			delay = time.Duration(1<<attempt) * time.Second
		}
		if err := sleepContext(ctx, delay); err != nil {
			return err
		}
	}
	return lastErr
}

func (c *Client) getOnce(ctx context.Context, token, endpoint string, target interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Auth-Token", token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		var payload struct {
			Message     string `json:"message"`
			Error       string `json:"error"`
			Description string `json:"description"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&payload)

		message := payload.Message
		if message == "" {
			message = payload.Error
		}
		if message == "" {
			message = payload.Description
		}
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    message,
			RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After")),
		}
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func (c *Client) waitTurn(ctx context.Context) error {
	const minInterval = 750 * time.Millisecond

	c.mu.Lock()
	if c.lastSlot.IsZero() {
		c.lastSlot = time.Now()
		c.mu.Unlock()
		return nil
	}
	slot := time.Now()
	if c.lastSlot.After(slot) {
		slot = c.lastSlot
	}
	slot = slot.Add(minInterval)
	c.lastSlot = slot
	c.mu.Unlock()

	delay := time.Until(slot)
	if delay <= 0 {
		return nil
	}
	return sleepContext(ctx, delay)
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func isRetriableStatus(statusCode int) bool {
	return statusCode >= http.StatusInternalServerError
}

func parseRetryAfter(raw string) time.Duration {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(raw); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(raw); err == nil {
		if delay := time.Until(when); delay > 0 {
			return delay
		}
	}
	return 0
}

func yuqueSyncKeys(token, groupLogin, namespace string) []string {
	keys := make([]string, 0, 2)
	if trimmedToken := strings.TrimSpace(token); trimmedToken != "" {
		digest := sha256.Sum256([]byte(trimmedToken))
		keys = append(keys, fmt.Sprintf("yuque:token:%x", digest[:8]))
	}
	if normalizedScope := normalizeYuqueScope(groupLogin, namespace); normalizedScope != "" {
		keys = append(keys, "yuque:scope:"+normalizedScope)
	}
	return keys
}

func normalizeYuqueScope(groupLogin, namespace string) string {
	normalizedGroup := strings.ToLower(strings.Trim(strings.TrimSpace(groupLogin), "/"))
	normalizedNamespace := strings.ToLower(strings.Trim(strings.TrimSpace(namespace), "/"))
	if normalizedNamespace != "" && !strings.Contains(normalizedNamespace, "/") && normalizedGroup != "" {
		normalizedNamespace = normalizedGroup + "/" + normalizedNamespace
	}
	if normalizedNamespace != "" {
		return normalizedNamespace
	}
	return normalizedGroup
}

func acquireYuqueGate(key string) *sync.Mutex {
	yuqueGateGuard.Lock()
	gate := yuqueGates[key]
	if gate == nil {
		gate = &scopedGate{}
		yuqueGates[key] = gate
	}
	gate.refCount++
	lock := &gate.lock
	yuqueGateGuard.Unlock()

	lock.Lock()
	return lock
}

func releaseYuqueGate(key string, lock *sync.Mutex) {
	lock.Unlock()
	yuqueGateGuard.Lock()
	defer yuqueGateGuard.Unlock()
	gate := yuqueGates[key]
	if gate == nil {
		return
	}
	gate.refCount--
	if gate.refCount <= 0 {
		delete(yuqueGates, key)
	}
}
