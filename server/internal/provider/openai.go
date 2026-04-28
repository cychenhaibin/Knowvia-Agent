package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/config"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

type ChatClient interface {
	Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

type ModelSelectableChatClient interface {
	CompleteWithConfig(
		ctx context.Context,
		systemPrompt string,
		userPrompt string,
		runtime domain.ChatRuntimeConfig,
	) (string, error)
	StreamWithConfig(
		ctx context.Context,
		systemPrompt string,
		userPrompt string,
		runtime domain.ChatRuntimeConfig,
		onDelta func(string) error,
	) (string, error)
}

type StreamingChatClient interface {
	ChatClient
	Stream(
		ctx context.Context,
		systemPrompt string,
		userPrompt string,
		onDelta func(string) error,
	) (string, error)
}

type OpenAICompatibleClient struct {
	baseURL    string
	apiKey     string
	chatModel  string
	httpClient *http.Client
}

func NewOpenAICompatibleClient(cfg config.Config) *OpenAICompatibleClient {
	return &OpenAICompatibleClient{
		baseURL:   strings.TrimRight(cfg.OpenAIBaseURL, "/"),
		apiKey:    cfg.OpenAIAPIKey,
		chatModel: cfg.OpenAIChatModel,
		httpClient: &http.Client{
			Timeout: 45 * time.Second,
		},
	}
}

func (c *OpenAICompatibleClient) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return c.CompleteWithConfig(ctx, systemPrompt, userPrompt, domain.ChatRuntimeConfig{})
}

func (c *OpenAICompatibleClient) CompleteWithConfig(
	ctx context.Context,
	systemPrompt string,
	userPrompt string,
	runtime domain.ChatRuntimeConfig,
) (string, error) {
	baseURL, apiKey, modelName := c.resolveRuntime(runtime)
	if baseURL == "" || modelName == "" {
		return "", errors.New("openai-compatible client is not configured")
	}

	body := map[string]interface{}{
		"model": modelName,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature": 0.2,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		baseURL+"/chat/completions",
		bytes.NewReader(payload),
	)
	if err != nil {
		return "", err
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return "", errors.New("chat completion request failed")
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 {
		return "", errors.New("chat completion returned no choices")
	}
	return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
}

func (c *OpenAICompatibleClient) Stream(
	ctx context.Context,
	systemPrompt string,
	userPrompt string,
	onDelta func(string) error,
) (string, error) {
	return c.StreamWithConfig(ctx, systemPrompt, userPrompt, domain.ChatRuntimeConfig{}, onDelta)
}

func (c *OpenAICompatibleClient) StreamWithConfig(
	ctx context.Context,
	systemPrompt string,
	userPrompt string,
	runtime domain.ChatRuntimeConfig,
	onDelta func(string) error,
) (string, error) {
	baseURL, apiKey, modelName := c.resolveRuntime(runtime)
	if baseURL == "" || modelName == "" {
		return "", errors.New("openai-compatible client is not configured")
	}

	body := map[string]interface{}{
		"model": modelName,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature": 0.2,
		"stream":      true,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		baseURL+"/chat/completions",
		bytes.NewReader(payload),
	)
	if err != nil {
		return "", err
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return "", errors.New("chat completion stream request failed")
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 1024), 1024*1024)

	var answer strings.Builder
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}

		var parsed struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &parsed); err != nil {
			return "", err
		}
		if len(parsed.Choices) == 0 {
			continue
		}

		chunk := parsed.Choices[0].Delta.Content
		if chunk == "" {
			chunk = parsed.Choices[0].Message.Content
		}
		if chunk == "" {
			continue
		}

		answer.WriteString(chunk)
		if onDelta != nil {
			if err := onDelta(chunk); err != nil {
				return strings.TrimSpace(answer.String()), err
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	result := strings.TrimSpace(answer.String())
	if result == "" {
		return "", errors.New("chat completion stream returned no content")
	}
	return result, nil
}

func (c *OpenAICompatibleClient) resolveRuntime(runtime domain.ChatRuntimeConfig) (baseURL string, apiKey string, modelName string) {
	baseURL = strings.TrimRight(strings.TrimSpace(runtime.BaseURL), "/")
	if baseURL == "" {
		baseURL = c.baseURL
	}

	apiKey = strings.TrimSpace(runtime.APIKey)
	if apiKey == "" {
		apiKey = c.apiKey
	}

	modelName = strings.TrimSpace(runtime.ModelName)
	if modelName == "" {
		modelName = c.chatModel
	}
	return baseURL, apiKey, modelName
}
