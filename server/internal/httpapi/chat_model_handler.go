package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/store"
)

func (h *Handler) ensureUserChatModels(ctx context.Context, userID string) error {
	return h.store.EnsureUserChatModelDefaults(ctx, userID)
}

func (h *Handler) listChatModels(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	if err := h.ensureUserChatModels(r.Context(), user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	models, err := h.store.ListUserChatModels(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	models = annotateChatModelAvailability(r.Context(), models)
	writeJSON(w, http.StatusOK, chatModelsResponse(models))
}

func (h *Handler) createChatModel(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	if err := h.ensureUserChatModels(r.Context(), user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var req struct {
		Purpose     string   `json:"purpose"`
		Name        string   `json:"name"`
		BaseURL     string   `json:"baseUrl"`
		APIKey      string   `json:"apiKey"`
		ModelName   string   `json:"modelName"`
		Temperature *float64 `json:"temperature"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeInvalidRequestBody(w)
		return
	}

	purpose, err := normalizeChatModelPurpose(req.Purpose)
	if err != nil {
		writeValidationError(w, err.Error(), []string{"purpose"})
		return
	}
	name := normalizeChatModelDisplayName(req.Name)
	if name == "" {
		writeValidationError(w, "name is required", []string{"name"})
		return
	}
	baseURL := normalizeChatModelBaseURL(req.BaseURL)
	if baseURL == "" {
		writeValidationError(w, "baseUrl is required", []string{"baseUrl"})
		return
	}
	modelName := normalizeModelName(req.ModelName)
	if modelName == "" {
		writeValidationError(w, "modelName is required", []string{"modelName"})
		return
	}
	temperature, err := normalizeChatModelTemperature(
		req.Temperature,
		domain.DefaultChatTemperatureForPurpose(purpose),
	)
	if err != nil {
		writeValidationError(w, err.Error(), []string{"temperature"})
		return
	}

	model := domain.UserChatModel{
		ID:          uuid.NewString(),
		UserID:      user.ID,
		Purpose:     purpose,
		Origin:      domain.ChatModelOriginCustom,
		Name:        name,
		BaseURL:     baseURL,
		APIKey:      normalizeChatModelAPIKey(req.APIKey),
		ModelName:   modelName,
		Temperature: temperature,
		IsSelected:  false,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	if err := h.store.CreateUserChatModel(r.Context(), model); err != nil {
		switch {
		case errors.Is(err, store.ErrConflict):
			writeValidationError(w, "model configuration already exists for this purpose", []string{"name"})
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	stored, err := h.store.GetUserChatModel(r.Context(), user.ID, model.ID)
	if err != nil {
		writeError(w, http.StatusCreated, "chat model created but could not be reloaded")
		return
	}
	stored.Available = true
	writeJSON(w, http.StatusCreated, stored)
}

func (h *Handler) updateChatModel(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	if err := h.ensureUserChatModels(r.Context(), user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	modelID := strings.TrimSpace(chi.URLParam(r, "modelID"))
	if modelID == "" {
		writeValidationError(w, "modelID is required", []string{"modelID"})
		return
	}

	model, err := h.store.GetUserChatModel(r.Context(), user.ID, modelID)
	if err != nil {
		writeNotFound(w, "chat model not found", "modelID")
		return
	}
	if model.Origin == domain.ChatModelOriginDefault {
		writeValidationError(w, "default chat models cannot be edited", []string{"modelID"})
		return
	}

	var req struct {
		Name        string   `json:"name"`
		BaseURL     string   `json:"baseUrl"`
		APIKey      string   `json:"apiKey"`
		ModelName   string   `json:"modelName"`
		Temperature *float64 `json:"temperature"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeInvalidRequestBody(w)
		return
	}

	name := normalizeChatModelDisplayName(req.Name)
	if name == "" {
		writeValidationError(w, "name is required", []string{"name"})
		return
	}
	baseURL := normalizeChatModelBaseURL(req.BaseURL)
	if baseURL == "" {
		writeValidationError(w, "baseUrl is required", []string{"baseUrl"})
		return
	}
	modelName := normalizeModelName(req.ModelName)
	if modelName == "" {
		writeValidationError(w, "modelName is required", []string{"modelName"})
		return
	}
	temperature, err := normalizeChatModelTemperature(req.Temperature, model.Temperature)
	if err != nil {
		writeValidationError(w, err.Error(), []string{"temperature"})
		return
	}

	model.Name = name
	model.BaseURL = baseURL
	model.APIKey = normalizeChatModelAPIKey(req.APIKey)
	model.ModelName = modelName
	model.Temperature = temperature
	model.UpdatedAt = time.Now().UTC()

	if err := h.store.UpdateUserChatModel(r.Context(), model); err != nil {
		switch {
		case errors.Is(err, store.ErrConflict):
			writeValidationError(w, "model configuration already exists for this purpose", []string{"name"})
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	stored, err := h.store.GetUserChatModel(r.Context(), user.ID, modelID)
	if err != nil {
		writeError(w, http.StatusOK, "chat model updated but could not be reloaded")
		return
	}
	stored.Available = true
	writeJSON(w, http.StatusOK, stored)
}

func (h *Handler) selectChatModel(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	if err := h.ensureUserChatModels(r.Context(), user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	modelID := strings.TrimSpace(chi.URLParam(r, "modelID"))
	if modelID == "" {
		writeValidationError(w, "modelID is required", []string{"modelID"})
		return
	}

	model, err := h.store.GetUserChatModel(r.Context(), user.ID, modelID)
	if err != nil {
		writeNotFound(w, "chat model not found", "modelID")
		return
	}
	if err := h.store.SelectUserChatModel(r.Context(), user.ID, model.ID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeNotFound(w, "chat model not found", "modelID")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	selected, err := h.store.GetSelectedUserChatModel(r.Context(), user.ID, model.Purpose)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	selected.Available = true
	writeJSON(w, http.StatusOK, selected)
}

func (h *Handler) deleteChatModel(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	if err := h.ensureUserChatModels(r.Context(), user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	modelID := strings.TrimSpace(chi.URLParam(r, "modelID"))
	if modelID == "" {
		writeValidationError(w, "modelID is required", []string{"modelID"})
		return
	}

	model, err := h.store.GetUserChatModel(r.Context(), user.ID, modelID)
	if err != nil {
		writeNotFound(w, "chat model not found", "modelID")
		return
	}
	if model.Origin == domain.ChatModelOriginDefault {
		writeValidationError(w, "default chat models cannot be deleted", []string{"modelID"})
		return
	}

	if err := h.store.DeleteUserChatModel(r.Context(), user.ID, modelID); err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeNotFound(w, "chat model not found", "modelID")
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func chatModelsResponse(models []domain.UserChatModel) map[string]any {
	general := make([]domain.UserChatModel, 0)
	knowledge := make([]domain.UserChatModel, 0)
	for _, model := range models {
		switch model.Purpose {
		case domain.ChatModelPurposeKnowledge:
			knowledge = append(knowledge, model)
		default:
			general = append(general, model)
		}
	}
	return map[string]any{
		"generalModels":   general,
		"knowledgeModels": knowledge,
	}
}

func annotateChatModelAvailability(ctx context.Context, models []domain.UserChatModel) []domain.UserChatModel {
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

func normalizeChatModelPurpose(raw string) (domain.ChatModelPurpose, error) {
	switch domain.ChatModelPurpose(strings.ToLower(strings.TrimSpace(raw))) {
	case domain.ChatModelPurposeGeneral:
		return domain.ChatModelPurposeGeneral, nil
	case domain.ChatModelPurposeKnowledge:
		return domain.ChatModelPurposeKnowledge, nil
	default:
		return "", errors.New("purpose must be general or knowledge")
	}
}

func normalizeChatModelDisplayName(raw string) string {
	return strings.TrimSpace(raw)
}

func normalizeChatModelBaseURL(raw string) string {
	return strings.TrimRight(strings.TrimSpace(raw), "/")
}

func normalizeChatModelAPIKey(raw string) string {
	return strings.TrimSpace(raw)
}

func normalizeModelName(raw string) string {
	return strings.TrimSpace(raw)
}

func normalizeChatModelTemperature(raw *float64, fallback float64) (float64, error) {
	if raw == nil {
		return fallback, nil
	}
	value := *raw
	if value < 0 || value > 2 {
		return 0, errors.New("temperature must be between 0 and 2")
	}
	return value, nil
}

func (h *Handler) resolveChatModel(
	ctx context.Context,
	userID string,
	useKnowledge bool,
	requestedModel string,
) (domain.UserChatModel, error) {
	purpose := domain.ChatModelPurposeGeneral
	if useKnowledge {
		purpose = domain.ChatModelPurposeKnowledge
	}

	models, err := h.store.ListUserChatModels(ctx, userID)
	if err != nil {
		return domain.UserChatModel{}, err
	}
	var selected *domain.UserChatModel
	for _, model := range models {
		if model.Purpose != purpose {
			continue
		}
		if requestedModel != "" &&
			(strings.EqualFold(model.ID, requestedModel) || strings.EqualFold(model.Name, requestedModel)) {
			return model, nil
		}
		if model.IsSelected && selected == nil {
			candidate := model
			selected = &candidate
		}
	}
	if selected != nil {
		return *selected, nil
	}
	name, runtime := domain.DefaultChatModelConfigForPurpose(purpose)
	return domain.UserChatModel{
		ID:          "",
		UserID:      userID,
		Purpose:     purpose,
		Origin:      domain.ChatModelOriginDefault,
		Name:        name,
		BaseURL:     runtime.BaseURL,
		APIKey:      runtime.APIKey,
		ModelName:   runtime.ModelName,
		Temperature: runtime.Temperature,
		IsSelected:  true,
		Available:   true,
	}, nil
}
