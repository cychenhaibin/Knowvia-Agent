package chat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func annotateModelAvailability(ctx context.Context, models []domain.UserChatModel) []domain.UserChatModel {
	annotated := append([]domain.UserChatModel(nil), models...)
	cache := make(map[string]bool)

	for index, model := range annotated {
		annotated[index].Available = true
		if !shouldProbeLocalOllamaModel(model) {
			continue
		}

		cacheKey := strings.Join([]string{model.BaseURL, model.APIKey, model.ModelName}, "|")
		available, ok := cache[cacheKey]
		if !ok {
			available = probeOllamaModelAvailability(ctx, model)
			cache[cacheKey] = available
		}
		annotated[index].Available = available
	}

	return annotated
}

func shouldProbeLocalOllamaModel(model domain.UserChatModel) bool {
	if model.Origin != domain.ChatModelOriginDefault {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(model.APIKey), domain.DefaultChatAPIKey) {
		return false
	}

	parsed, err := url.Parse(model.BaseURL)
	if err != nil {
		return false
	}

	switch strings.ToLower(parsed.Hostname()) {
	case "127.0.0.1", "localhost":
		return true
	default:
		return false
	}
}

func probeOllamaModelAvailability(ctx context.Context, model domain.UserChatModel) bool {
	probeCtx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()

	if checkOpenAICompatibleModels(probeCtx, model) {
		return true
	}

	return checkNativeOllamaTags(probeCtx, model)
}

func checkOpenAICompatibleModels(ctx context.Context, model domain.UserChatModel) bool {
	endpoint := strings.TrimRight(model.BaseURL, "/") + "/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return false
	}
	if strings.TrimSpace(model.APIKey) != "" {
		req.Header.Set("Authorization", "Bearer "+model.APIKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false
	}

	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return false
	}

	for _, item := range payload.Data {
		if strings.EqualFold(strings.TrimSpace(item.ID), model.ModelName) {
			return true
		}
	}

	return false
}

func checkNativeOllamaTags(ctx context.Context, model domain.UserChatModel) bool {
	parsed, err := url.Parse(model.BaseURL)
	if err != nil {
		return false
	}

	basePath := strings.TrimRight(parsed.Path, "/")
	if strings.HasSuffix(strings.ToLower(basePath), "/v1") {
		basePath = strings.TrimSuffix(basePath, "/v1")
	}
	parsed.Path = basePath + "/api/tags"
	parsed.RawQuery = ""
	parsed.Fragment = ""

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return false
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false
	}

	var payload struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return false
	}

	for _, item := range payload.Models {
		if strings.EqualFold(strings.TrimSpace(item.Name), model.ModelName) {
			return true
		}
	}

	return false
}
