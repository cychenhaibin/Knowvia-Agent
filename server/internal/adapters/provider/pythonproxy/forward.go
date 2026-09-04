package pythonproxy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/provider"
)

func (c *Client) StreamKnowledgeChat(
	ctx context.Context,
	req provider.ForwardedChatRequest,
	onRetrieval func([]provider.ForwardedSource) error,
	onDelta func(string) error,
) (provider.ForwardedChatResult, error) {
	if c == nil {
		return provider.ForwardedChatResult{}, errors.New("python forward client is not configured")
	}
	return c.streamKnowledgeChatWithRetry(ctx, req, onRetrieval, onDelta)
}

func (c *Client) GenerateReport(
	ctx context.Context,
	req provider.ForwardedReportRequest,
) (provider.ForwardedReportResult, error) {
	if c == nil {
		return provider.ForwardedReportResult{}, errors.New("python forward client is not configured")
	}
	var result provider.ForwardedReportResult
	if err := c.doJSONWithRetryWithClient(
		ctx,
		c.syncClient,
		http.MethodPost,
		"/internal/v1/report/generate",
		req,
		&result,
	); err != nil {
		return provider.ForwardedReportResult{}, err
	}
	return result, nil
}

func (c *Client) RetrieveKnowledge(
	ctx context.Context,
	req provider.ForwardedRetrieveRequest,
) (provider.ForwardedRetrieveResult, error) {
	if c == nil {
		return provider.ForwardedRetrieveResult{}, errors.New("python forward client is not configured")
	}
	var result provider.ForwardedRetrieveResult
	if err := c.doJSONWithRetry(
		ctx,
		http.MethodPost,
		"/internal/v1/retrieve",
		req,
		&result,
	); err != nil {
		return provider.ForwardedRetrieveResult{}, err
	}
	return result, nil
}

func (c *Client) MergeEvidence(
	ctx context.Context,
	req provider.ForwardedEvidenceMergeRequest,
) (provider.ForwardedEvidenceMergeResult, error) {
	if c == nil {
		return provider.ForwardedEvidenceMergeResult{}, errors.New("python forward client is not configured")
	}
	var result provider.ForwardedEvidenceMergeResult
	if err := c.doJSONWithRetry(
		ctx,
		http.MethodPost,
		"/internal/v1/evidence/merge",
		req,
		&result,
	); err != nil {
		return provider.ForwardedEvidenceMergeResult{}, err
	}
	return result, nil
}

func (c *Client) streamKnowledgeChatWithRetry(
	ctx context.Context,
	req provider.ForwardedChatRequest,
	onRetrieval func([]provider.ForwardedSource) error,
	onDelta func(string) error,
) (provider.ForwardedChatResult, error) {
	return c.streamKnowledgeChat(ctx, req, onRetrieval, onDelta)
}

func (c *Client) streamKnowledgeChat(
	ctx context.Context,
	req provider.ForwardedChatRequest,
	onRetrieval func([]provider.ForwardedSource) error,
	onDelta func(string) error,
) (provider.ForwardedChatResult, error) {
	token, err := c.getToken(ctx)
	if err != nil {
		return provider.ForwardedChatResult{}, err
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return provider.ForwardedChatResult{}, err
	}
	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/internal/v1/chat/stream",
		bytes.NewReader(payload),
	)
	if err != nil {
		return provider.ForwardedChatResult{}, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return provider.ForwardedChatResult{}, c.normalizeTransportError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return provider.ForwardedChatResult{}, unauthorizedError(resp.Status)
	}
	if resp.StatusCode >= 300 {
		return provider.ForwardedChatResult{}, httpError("python knowledge stream request failed", resp)
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 1024), 1024*1024)

	var answer strings.Builder
	sources := []provider.ForwardedSource{}
	var usage *provider.ForwardedUsage
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") || !strings.HasPrefix(line, "data:") {
			continue
		}

		var event struct {
			Type    string                     `json:"type"`
			TraceID string                     `json:"traceId"`
			Content string                     `json:"content"`
			Error   any                        `json:"error"`
			Sources []provider.ForwardedSource `json:"sources"`
			Usage   *provider.ForwardedUsage   `json:"usage"`
			Metrics struct {
				RetrieveMS int `json:"retrieveMs"`
				GenerateMS int `json:"generateMs"`
				TotalMS    int `json:"totalMs"`
			} `json:"metrics"`
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return provider.ForwardedChatResult{}, err
		}
		if message := forwardedEventErrorMessage(event.Error); message != "" {
			return provider.ForwardedChatResult{}, errors.New(message)
		}

		switch event.Type {
		case "usage":
			usage = event.Usage
		case "retrieval":
			sources = append([]provider.ForwardedSource(nil), event.Sources...)
			if onRetrieval != nil {
				if err := onRetrieval(sources); err != nil {
					return provider.ForwardedChatResult{Answer: answer.String(), Sources: sources}, err
				}
			}
		case "chunk":
			if event.Content != "" {
				answer.WriteString(event.Content)
				if onDelta != nil {
					if err := onDelta(event.Content); err != nil {
						return provider.ForwardedChatResult{Answer: answer.String(), Sources: sources}, err
					}
				}
			}
		case "done":
			finalAnswer := strings.TrimSpace(answer.String())
			if strings.TrimSpace(event.Content) != "" && finalAnswer == "" {
				finalAnswer = strings.TrimSpace(event.Content)
			}
			if len(event.Sources) > 0 {
				sources = append([]provider.ForwardedSource(nil), event.Sources...)
			}
			return provider.ForwardedChatResult{
				TraceID: event.TraceID,
				Answer:  finalAnswer,
				Sources: sources,
				Metrics: provider.ForwardedMetrics{
					RetrieveMS: event.Metrics.RetrieveMS,
					GenerateMS: event.Metrics.GenerateMS,
					TotalMS:    event.Metrics.TotalMS,
				},
				Usage: firstForwardedUsage(event.Usage, usage),
			}, nil
		case "error":
			return provider.ForwardedChatResult{}, errors.New(forwardedEventErrorMessage(event.Error))
		}
	}
	if err := scanner.Err(); err != nil {
		return provider.ForwardedChatResult{}, err
	}
	return provider.ForwardedChatResult{}, errors.New("python knowledge stream ended without done event")
}

func firstForwardedUsage(values ...*provider.ForwardedUsage) *provider.ForwardedUsage {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func forwardedEventErrorMessage(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case map[string]any:
		if message, ok := typed["message"].(string); ok {
			return strings.TrimSpace(message)
		}
	case map[string]string:
		return strings.TrimSpace(typed["message"])
	}
	return ""
}
