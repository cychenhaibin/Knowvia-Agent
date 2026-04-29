package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/chat"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func (h *Handler) listChatModels(w http.ResponseWriter, r *http.Request) {
	if h.chatService == nil {
		writeServiceUnavailable(w, "chat service unavailable")
		return
	}
	user := currentUser(r.Context())
	models, err := h.chatService.ListAvailableModels(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, mapChatModelsResponse(models))
}

func (h *Handler) createChatModel(w http.ResponseWriter, r *http.Request) {
	if h.chatService == nil {
		writeServiceUnavailable(w, "chat service unavailable")
		return
	}
	user := currentUser(r.Context())
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

	stored, err := h.chatService.CreateModel(r.Context(), chat.CreateModelInput{
		UserID:      user.ID,
		Purpose:     purpose,
		Name:        name,
		BaseURL:     baseURL,
		APIKey:      normalizeChatModelAPIKey(req.APIKey),
		ModelName:   modelName,
		Temperature: temperature,
	})
	if err != nil {
		switch {
		case errors.Is(err, chat.ErrChatModelConflict):
			writeValidationError(w, "model configuration already exists for this purpose", []string{"name"})
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	stored.Available = true
	writeJSON(w, http.StatusCreated, mapChatModel(stored))
}

func (h *Handler) updateChatModel(w http.ResponseWriter, r *http.Request) {
	if h.chatService == nil {
		writeServiceUnavailable(w, "chat service unavailable")
		return
	}
	user := currentUser(r.Context())
	modelID := strings.TrimSpace(chi.URLParam(r, "modelID"))
	if modelID == "" {
		writeValidationError(w, "modelID is required", []string{"modelID"})
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
	if _, err := normalizeChatModelTemperature(req.Temperature, domain.DefaultChatTemperatureForPurpose(domain.ChatModelPurposeGeneral)); err != nil {
		writeValidationError(w, err.Error(), []string{"temperature"})
		return
	}

	stored, err := h.chatService.UpdateModel(r.Context(), chat.UpdateModelInput{
		UserID:      user.ID,
		ModelID:     modelID,
		Name:        name,
		BaseURL:     baseURL,
		APIKey:      normalizeChatModelAPIKey(req.APIKey),
		ModelName:   modelName,
		Temperature: req.Temperature,
	})
	if err != nil {
		switch {
		case errors.Is(err, chat.ErrChatModelNotFound):
			writeNotFound(w, "chat model not found", "modelID")
		case errors.Is(err, chat.ErrDefaultChatModelImmutable):
			writeValidationError(w, "default chat models cannot be edited", []string{"modelID"})
		case errors.Is(err, chat.ErrChatModelConflict):
			writeValidationError(w, "model configuration already exists for this purpose", []string{"name"})
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	stored.Available = true
	writeJSON(w, http.StatusOK, mapChatModel(stored))
}

func (h *Handler) selectChatModel(w http.ResponseWriter, r *http.Request) {
	if h.chatService == nil {
		writeServiceUnavailable(w, "chat service unavailable")
		return
	}
	user := currentUser(r.Context())
	modelID := strings.TrimSpace(chi.URLParam(r, "modelID"))
	if modelID == "" {
		writeValidationError(w, "modelID is required", []string{"modelID"})
		return
	}

	selected, err := h.chatService.SelectModel(r.Context(), user.ID, modelID)
	if err != nil {
		if errors.Is(err, chat.ErrChatModelNotFound) {
			writeNotFound(w, "chat model not found", "modelID")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	selected.Available = true
	writeJSON(w, http.StatusOK, mapChatModel(selected))
}

func (h *Handler) deleteChatModel(w http.ResponseWriter, r *http.Request) {
	if h.chatService == nil {
		writeServiceUnavailable(w, "chat service unavailable")
		return
	}
	user := currentUser(r.Context())
	modelID := strings.TrimSpace(chi.URLParam(r, "modelID"))
	if modelID == "" {
		writeValidationError(w, "modelID is required", []string{"modelID"})
		return
	}

	if err := h.chatService.DeleteModel(r.Context(), user.ID, modelID); err != nil {
		switch {
		case errors.Is(err, chat.ErrChatModelNotFound):
			writeNotFound(w, "chat model not found", "modelID")
		case errors.Is(err, chat.ErrDefaultChatModelImmutable):
			writeValidationError(w, "default chat models cannot be deleted", []string{"modelID"})
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
