package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/config"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func TestResolveRuntimeFallsBackToDefaults(t *testing.T) {
	t.Parallel()

	client := New(config.Config{
		OpenAIBaseURL:   "http://localhost:11434/v1/",
		OpenAIAPIKey:    "default-key",
		OpenAIChatModel: "default-model",
	})

	baseURL, apiKey, modelName := client.resolveRuntime(domain.ChatRuntimeConfig{})
	if baseURL != "http://localhost:11434/v1" || apiKey != "default-key" || modelName != "default-model" {
		t.Fatalf("resolveRuntime returned (%q, %q, %q)", baseURL, apiKey, modelName)
	}
}

func TestResolveRuntimePrefersExplicitOverrides(t *testing.T) {
	t.Parallel()

	client := New(config.Config{
		OpenAIBaseURL:   "http://localhost:11434/v1",
		OpenAIAPIKey:    "default-key",
		OpenAIChatModel: "default-model",
	})

	baseURL, apiKey, modelName := client.resolveRuntime(domain.ChatRuntimeConfig{
		BaseURL:   "https://api.example.com/v1/",
		APIKey:    "override-key",
		ModelName: "override-model",
	})
	if baseURL != "https://api.example.com/v1" || apiKey != "override-key" || modelName != "override-model" {
		t.Fatalf("resolveRuntime returned (%q, %q, %q)", baseURL, apiKey, modelName)
	}
}

func TestStreamWithConfigIncludesSearchAndUsageOptions(t *testing.T) {
	var requestBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":5,\"total_tokens\":8},\"choices\":[]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	client := New(config.Config{})
	var gotUsage *domain.ChatUsage
	answer, err := client.StreamWithConfig(
		context.Background(),
		"system",
		"user",
		domain.ChatRuntimeConfig{
			BaseURL:        server.URL,
			APIKey:         "test-key",
			ModelName:      "qwen-plus",
			Temperature:    0.7,
			TemperatureSet: true,
			EnableSearch:   true,
		},
		nil,
		func(usage domain.ChatUsage) error {
			gotUsage = &usage
			return nil
		},
	)
	if err != nil {
		t.Fatalf("stream with config: %v", err)
	}
	if answer != "hello" {
		t.Fatalf("answer = %q, want hello", answer)
	}
	if gotUsage == nil || gotUsage.PromptTokens != 3 || gotUsage.CompletionTokens != 5 || gotUsage.TotalTokens != 8 {
		t.Fatalf("usage = %#v, want prompt=3 completion=5 total=8", gotUsage)
	}
	if requestBody["temperature"] != float64(0.7) {
		t.Fatalf("temperature = %#v, want 0.7", requestBody["temperature"])
	}
	if requestBody["enable_search"] != true {
		t.Fatalf("enable_search = %#v, want true", requestBody["enable_search"])
	}
	streamOptions, ok := requestBody["stream_options"].(map[string]any)
	if !ok {
		t.Fatalf("stream_options = %#v, want object", requestBody["stream_options"])
	}
	if streamOptions["include_usage"] != true {
		t.Fatalf("include_usage = %#v, want true", streamOptions["include_usage"])
	}
}

func TestStreamWithConfigOmitsTemperatureWithoutExplicitInput(t *testing.T) {
	var requestBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	client := New(config.Config{})
	_, err := client.StreamWithConfig(
		context.Background(),
		"system",
		"user",
		domain.ChatRuntimeConfig{
			BaseURL:     server.URL,
			APIKey:      "test-key",
			ModelName:   "qwen-plus",
			Temperature: 0.05,
		},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("stream with config: %v", err)
	}
	if _, ok := requestBody["temperature"]; ok {
		t.Fatalf("temperature should be omitted when not explicitly provided, got %#v", requestBody["temperature"])
	}
}
