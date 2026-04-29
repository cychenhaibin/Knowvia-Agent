package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/chat"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func (h *Handler) chatStream(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	message := strings.TrimSpace(r.URL.Query().Get("message"))
	if message == "" {
		writeValidationError(w, "message is required", []string{"message"})
		return
	}
	if h.chatService == nil {
		writeError(w, http.StatusServiceUnavailable, "chat service unavailable")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	skill := chat.NormalizeSkill(r.URL.Query().Get("skill"))
	customPrompt := strings.TrimSpace(r.URL.Query().Get("skill_prompt"))
	skillID := strings.TrimSpace(r.URL.Query().Get("skill_id"))
	skillDefinitionID := strings.TrimSpace(r.URL.Query().Get("skill_definition_id"))
	sessionID := strings.TrimSpace(r.URL.Query().Get("session_id"))
	connectionIDs := splitCommaQuery(r.URL.Query().Get("connection_ids"))
	useKnowledge := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("use_knowledge")), "true")
	enableSearch := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("enable_search")), "true")
	temperature, err := parseOptionalChatTemperature(r.URL.Query().Get("temperature"))
	if err != nil {
		writeValidationError(w, err.Error(), []string{"temperature"})
		return
	}
	requestedChatModel := strings.TrimSpace(r.URL.Query().Get("chat_model"))
	prepared, err := h.chatService.PrepareStreamConversation(
		r.Context(),
		chat.PrepareStreamInput{
			UserID:             user.ID,
			Message:            message,
			SessionID:          sessionID,
			ConnectionIDs:      connectionIDs,
			RequestedChatModel: requestedChatModel,
			Skill:              skill,
			SkillID:            skillID,
			SkillDefinitionID:  skillDefinitionID,
			CustomPrompt:       customPrompt,
			UseKnowledge:       useKnowledge,
			EnableSearch:       enableSearch,
			Temperature:        temperature,
		},
	)
	if err != nil {
		if errors.Is(err, chat.ErrKnowledgeForwardUnavailable) {
			streamError(w, flusher, errorCodeServiceUnavailable, "python knowledge service unavailable")
			return
		}
		if streamSkillSelectionError(w, flusher, err) {
			return
		}
		streamInternalError(w, flusher, err.Error())
		return
	}

	result, err := h.chatService.StreamConversation(
		r.Context(),
		prepared.Request,
		chat.StreamConversationHooks{
			OnSession: func(session domain.ChatSession) error {
				streamJSON(w, flusher, map[string]any{
					"type":       "session",
					"session_id": session.ID,
					"title":      session.Title,
				})
				return nil
			},
			OnRetrieval: func(items []domain.Evidence) error {
				streamJSON(w, flusher, map[string]any{
					"type":    "retrieval",
					"skill":   prepared.Skill,
					"sources": mapChatStreamSources(items),
				})
				return nil
			},
			OnChunk: func(chunk string) error {
				streamJSON(w, flusher, map[string]any{
					"type":    "chunk",
					"content": chunk,
				})
				return nil
			},
		},
	)
	if err != nil {
		streamChatExecutionError(w, flusher, err)
		return
	}

	donePayload := map[string]any{
		"type":    "done",
		"content": result.Answer,
		"sources": mapChatStreamSources(result.Sources),
	}
	if usage := mapChatUsage(result.Usage); usage != nil {
		donePayload["usage"] = usage
	}
	streamJSON(w, flusher, donePayload)
}

func parseOptionalChatTemperature(raw string) (*float64, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}
	temperature, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return nil, errors.New("temperature must be a number")
	}
	if temperature < 0 || temperature > 2 {
		return nil, errors.New("temperature must be between 0 and 2")
	}
	return &temperature, nil
}

func (h *Handler) createChatSession(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	var req struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeInvalidRequestBody(w)
		return
	}
	session, err := h.chatService.CreateSession(r.Context(), user.ID, req.Title)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, mapChatSession(session))
}

func (h *Handler) listChatSessions(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	sessions, err := h.chatService.ListSessions(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, chatSessionListDTO{Items: mapChatSessions(sessions)})
}

func (h *Handler) updateChatSession(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	sessionID := chi.URLParam(r, "sessionID")
	var req struct {
		Title  *string `json:"title"`
		Pinned *bool   `json:"pinned"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeInvalidRequestBody(w)
		return
	}
	session, err := h.chatService.UpdateSession(r.Context(), user.ID, sessionID, chat.UpdateSessionInput{
		Title:  req.Title,
		Pinned: req.Pinned,
	})
	if err != nil {
		if errors.Is(err, chat.ErrChatSessionNotFound) {
			writeNotFound(w, "chat session not found", "chat_session")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, mapChatSession(session))
}

func (h *Handler) deleteChatSession(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	sessionID := chi.URLParam(r, "sessionID")
	if err := h.chatService.DeleteSession(r.Context(), user.ID, sessionID); err != nil {
		if errors.Is(err, chat.ErrChatSessionNotFound) {
			writeNotFound(w, "chat session not found", "chat_session")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listChatMessages(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	sessionID := chi.URLParam(r, "sessionID")
	messages, err := h.chatService.ListMessages(r.Context(), user.ID, sessionID)
	if err != nil {
		if errors.Is(err, chat.ErrChatSessionNotFound) {
			writeNotFound(w, "chat session not found", "chat_session")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, chatMessageListDTO{Items: mapChatMessages(messages)})
}

func streamChatExecutionError(w http.ResponseWriter, flusher http.Flusher, err error) {
	if err == nil {
		streamInternalError(w, flusher, "unknown chat execution error")
		return
	}
	message := strings.TrimSpace(err.Error())
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "python forward client is not configured"),
		strings.Contains(lower, "forward chat client unavailable"),
		strings.Contains(lower, "python knowledge sync unavailable"):
		streamError(w, flusher, errorCodeServiceUnavailable, message)
	case strings.Contains(lower, "python knowledge stream request failed"),
		strings.Contains(lower, "python proxy request failed"),
		strings.Contains(lower, "python chat proxy"),
		strings.Contains(lower, "python backend unavailable"):
		streamError(w, flusher, errorCodeBadGateway, message)
	default:
		streamInternalError(w, flusher, message)
	}
}
