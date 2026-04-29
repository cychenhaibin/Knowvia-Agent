package provider

import (
	"strings"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func InferForwardedModelProfile(runtime domain.ChatRuntimeConfig, purpose string) *ForwardedModelProfile {
	modelName := strings.TrimSpace(runtime.ModelName)
	baseURL := strings.TrimSpace(runtime.BaseURL)
	apiKey := strings.TrimSpace(runtime.APIKey)
	if modelName == "" && baseURL == "" && apiKey == "" && strings.TrimSpace(purpose) == "" {
		return nil
	}
	var temperature *float64
	if runtime.TemperatureSet {
		temperature = &runtime.Temperature
	}
	providerName := "fallback"
	baseLower := strings.ToLower(baseURL)
	switch {
	case strings.Contains(baseLower, "11434") || strings.EqualFold(apiKey, "ollama"):
		providerName = "ollama"
	case modelName != "" || baseURL != "" || apiKey != "":
		providerName = "openai_compatible"
	}
	return &ForwardedModelProfile{
		Purpose:     strings.TrimSpace(purpose),
		Provider:    providerName,
		Name:        modelName,
		BaseURL:     baseURL,
		APIKey:      apiKey,
		ModelName:   modelName,
		Temperature: temperature,
	}
}
