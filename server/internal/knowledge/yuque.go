package knowledge

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
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

func (c *Client) ListDocs(ctx context.Context, token, namespace string) ([]DocMeta, error) {
	var response struct {
		Data []DocMeta `json:"data"`
	}
	if err := c.get(ctx, token, fmt.Sprintf("%s/repos/%s/docs", c.baseURL, namespace), &response); err != nil {
		return nil, err
	}
	return response.Data, nil
}

func (c *Client) GetDocBody(ctx context.Context, token, namespace, slug string) (string, error) {
	var response struct {
		Data struct {
			Body      string `json:"body"`
			UpdatedAt string `json:"updated_at"`
		} `json:"data"`
	}
	endpoint := fmt.Sprintf("%s/repos/%s/docs/%s", c.baseURL, namespace, url.PathEscape(slug))
	if err := c.get(ctx, token, endpoint, &response); err != nil {
		return "", err
	}
	return response.Data.Body, nil
}

func (c *Client) FetchRawPayload(
	ctx context.Context,
	token, groupLogin, namespace string,
) (map[string]any, error) {
	groupLogin = strings.TrimSpace(groupLogin)
	namespace = strings.Trim(strings.TrimSpace(namespace), "/")
	if token == "" {
		return nil, fmt.Errorf("yuque token is required")
	}
	if groupLogin == "" || namespace == "" {
		return nil, fmt.Errorf("yuque groupLogin and namespace are required")
	}

	heldGates := make([]struct {
		key  string
		lock *sync.Mutex
	}, 0, 2)
	for _, gateKey := range yuqueSyncKeys(token, groupLogin, namespace) {
		heldGates = append(heldGates, struct {
			key  string
			lock *sync.Mutex
		}{
			key:  gateKey,
			lock: acquireYuqueGate(gateKey),
		})
	}
	defer func() {
		for idx := len(heldGates) - 1; idx >= 0; idx-- {
			releaseYuqueGate(heldGates[idx].key, heldGates[idx].lock)
		}
	}()

	docsByNamespace := map[string]any{}
	bodiesByNamespace := map[string]any{}
	docs, err := c.ListDocs(ctx, token, namespace)
	if err != nil {
		return nil, err
	}
	docsByNamespace[namespace] = docs
	bodyMap := map[string]string{}
	for _, doc := range docs {
		slug := strings.TrimSpace(doc.Slug)
		if slug == "" {
			continue
		}
		body, err := c.GetDocBody(ctx, token, namespace, slug)
		if err != nil {
			return nil, err
		}
		bodyMap[slug] = body
	}
	bodiesByNamespace[namespace] = bodyMap

	return map[string]any{
		"group_login":         groupLogin,
		"namespace":           namespace,
		"repos":               []map[string]string{{"namespace": namespace}},
		"docs_by_namespace":   docsByNamespace,
		"bodies_by_namespace": bodiesByNamespace,
	}, nil
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
