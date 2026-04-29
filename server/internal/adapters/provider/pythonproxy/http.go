package pythonproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	neturl "net/url"
	"strings"
)

func (c *Client) doJSONWithRetry(
	ctx context.Context,
	method string,
	path string,
	body any,
	target any,
) error {
	return c.doJSONWithRetryWithClient(ctx, c.httpClient, method, path, body, target)
}

func (c *Client) doJSONWithRetryWithClient(
	ctx context.Context,
	client *http.Client,
	method string,
	path string,
	body any,
	target any,
) error {
	return c.doJSONWithClient(ctx, client, method, path, body, target)
}

func (c *Client) doJSONWithClient(
	ctx context.Context,
	client *http.Client,
	method string,
	path string,
	body any,
	target any,
) error {
	token, err := c.getToken(ctx)
	if err != nil {
		return err
	}

	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return c.normalizeTransportError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return unauthorizedError(resp.Status)
	}
	if resp.StatusCode >= 300 {
		return httpError("python proxy request failed", resp)
	}
	if target == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func (c *Client) getToken(ctx context.Context) (string, error) {
	_ = ctx
	if strings.TrimSpace(c.staticToken) == "" {
		return "", errors.New("python internal proxy token is not configured")
	}
	return strings.TrimSpace(c.staticToken), nil
}

func (c *Client) clearCachedToken() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cachedToken = ""
}

func (c *Client) normalizeTransportError(err error) error {
	if errors.Is(err, context.Canceled) {
		return err
	}
	if !isTransportError(err) {
		return err
	}
	return fmt.Errorf(
		"python backend unavailable at %s; start quickque-agent/llm/run_server.sh or update QQA_PYTHON_PROXY_BASE_URL",
		c.baseURL,
	)
}

func unauthorizedError(status string) error {
	return fmt.Errorf("python chat proxy unauthorized: %s", status)
}

func isUnauthorized(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "unauthorized")
}

func isTransportError(err error) bool {
	if err == nil {
		return false
	}

	var urlErr *neturl.Error
	if errors.As(err, &urlErr) {
		return true
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}

	var netErr net.Error
	return errors.As(err, &netErr)
}

func httpError(prefix string, resp *http.Response) error {
	defer resp.Body.Close()
	var payload struct {
		Error   string `json:"error"`
		Detail  string `json:"detail"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err == nil {
		message := strings.TrimSpace(payload.Error)
		if message == "" {
			message = strings.TrimSpace(payload.Detail)
		}
		if message == "" {
			message = strings.TrimSpace(payload.Message)
		}
		if message != "" {
			return fmt.Errorf("%s: %s", prefix, message)
		}
	}
	return fmt.Errorf("%s: %s", prefix, resp.Status)
}
