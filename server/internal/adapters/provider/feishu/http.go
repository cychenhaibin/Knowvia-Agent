package feishu

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	neturl "net/url"
	"strings"
)

func (c *Client) doJSON(
	ctx context.Context,
	method, url, bearer string,
	params neturl.Values,
	body any,
	target any,
) error {
	var reqBody []byte
	var err error
	if body != nil {
		reqBody, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	if len(params) > 0 {
		url += "?" + params.Encode()
	}
	log.Printf(
		"feishu http request method=%s url=%s query=%s body=%s bearer=%s",
		method,
		url,
		redactQuery(params),
		redactBody(reqBody),
		redactBearer(bearer),
	)
	req, err := http.NewRequestWithContext(ctx, method, url, strings.NewReader(string(reqBody)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("feishu http transport error method=%s url=%s error=%v", method, url, err)
		return normalizeFeishuTransportError(err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("feishu http read body error method=%s url=%s error=%v", method, url, err)
		return normalizeFeishuTransportError(err)
	}
	log.Printf(
		"feishu http response method=%s url=%s status=%d body=%s",
		method,
		url,
		resp.StatusCode,
		redactBody(bodyBytes),
	)

	var payload map[string]any
	if len(bodyBytes) > 0 {
		if err := json.Unmarshal(bodyBytes, &payload); err != nil {
			return err
		}
	}
	if payload == nil {
		payload = map[string]any{}
	}
	if resp.StatusCode >= 300 {
		return normalizeFeishuAPIError(resp.StatusCode, payload)
	}
	if code, ok := payload["code"].(float64); ok && int(code) != 0 {
		return normalizeFeishuAPIError(resp.StatusCode, payload)
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, target)
}

func redactBearer(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	if len(token) <= 8 {
		return "***"
	}
	return token[:4] + "***" + token[len(token)-4:]
}

func redactQuery(values neturl.Values) string {
	if len(values) == 0 {
		return ""
	}
	cloned := neturl.Values{}
	for key, items := range values {
		copyItems := append([]string(nil), items...)
		lower := strings.ToLower(strings.TrimSpace(key))
		if strings.Contains(lower, "token") || strings.Contains(lower, "secret") {
			for idx := range copyItems {
				copyItems[idx] = "***"
			}
		}
		cloned[key] = copyItems
	}
	return cloned.Encode()
}

func redactBody(raw []byte) string {
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return ""
	}
	var payload any
	if err := json.Unmarshal(raw, &payload); err == nil {
		redacted := redactJSONValue(payload)
		if normalized, marshalErr := json.Marshal(redacted); marshalErr == nil {
			return string(normalized)
		}
	}
	if len(text) > 1200 {
		return text[:1200] + "...(truncated)"
	}
	return text
}

func redactJSONValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := map[string]any{}
		for key, item := range typed {
			lower := strings.ToLower(strings.TrimSpace(key))
			if strings.Contains(lower, "secret") || strings.Contains(lower, "token") {
				out[key] = "***"
				continue
			}
			out[key] = redactJSONValue(item)
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, redactJSONValue(item))
		}
		return out
	default:
		return value
	}
}
