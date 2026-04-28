package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/chat"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skillruntime"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/store"
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
	requestedChatModel := strings.TrimSpace(r.URL.Query().Get("chat_model"))
	var selectedSkill *domain.Skill
	if useKnowledge && !h.chatService.CanForward(true) {
		streamError(w, flusher, errorCodeServiceUnavailable, "python knowledge service unavailable")
		return
	}
	if err := h.ensureUserChatModels(r.Context(), user.ID); err != nil {
		streamInternalError(w, flusher, err.Error())
		return
	}
	chatModel, err := h.resolveChatModel(r.Context(), user.ID, useKnowledge, requestedChatModel)
	if err != nil {
		streamInternalError(w, flusher, err.Error())
		return
	}

	if skillID != "" || skillDefinitionID != "" {
		resolvedSkillID, storedSkill, err := h.resolveChatSkillSelection(r.Context(), user.ID, skillID, skillDefinitionID)
		if err != nil {
			if streamSkillSelectionError(w, flusher, err, skillID, skillDefinitionID) {
				return
			}
			streamError(w, flusher, errorCodeNotFound, "skill not found")
			return
		}
		skillID = resolvedSkillID
		if mode := chat.NormalizeSkill(storedSkill.Mode); mode != "" {
			skill = mode
		}
		if strings.TrimSpace(storedSkill.Prompt) != "" {
			customPrompt = strings.TrimSpace(storedSkill.Prompt)
		}
		selectedSkill = &storedSkill
	}

	session, err := h.ensureChatSession(r.Context(), user.ID, sessionID, message)
	if err != nil {
		streamInternalError(w, flusher, err.Error())
		return
	}

	streamJSON(w, flusher, map[string]any{
		"type":       "session",
		"session_id": session.ID,
		"title":      session.Title,
	})

	now := time.Now().UTC()
	userMessage := domain.ChatMessage{
		ID:           uuid.NewString(),
		SessionID:    session.ID,
		UserID:       user.ID,
		Role:         domain.ChatRoleUser,
		Content:      message,
		Skill:        string(skill),
		UseKnowledge: useKnowledge,
		CreatedAt:    now,
		CompletedAt:  &now,
	}
	if err := h.store.SaveChatMessage(r.Context(), userMessage); err != nil {
		streamInternalError(w, flusher, err.Error())
		return
	}

	assistantMessage := domain.ChatMessage{
		ID:           uuid.NewString(),
		SessionID:    session.ID,
		UserID:       user.ID,
		Role:         domain.ChatRoleAssistant,
		Skill:        string(skill),
		UseKnowledge: useKnowledge,
		CreatedAt:    time.Now().UTC(),
	}
	if err := h.store.SaveChatMessage(r.Context(), assistantMessage); err != nil {
		streamInternalError(w, flusher, err.Error())
		return
	}
	var skillSnapshot *domain.SkillRuntimeSnapshot
	if selectedSkill != nil {
		snapshot, err := skillruntime.BuildSnapshot(
			*selectedSkill,
			domain.SkillRuntimeScopeChat,
			assistantMessage.ID,
			uuid.NewString(),
			time.Now().UTC(),
		)
		if err != nil {
			streamInternalError(w, flusher, err.Error())
			return
		}
		if err := h.store.CreateSkillRuntimeSnapshot(r.Context(), snapshot); err != nil {
			streamInternalError(w, flusher, err.Error())
			return
		}
		skillSnapshot = &snapshot
	}

	sources := []domain.Evidence{}
	answer := ""
	useForward := h.chatService.CanForward(useKnowledge)
	if useForward {
		answer, sources, err = h.chatService.StreamForwarded(
			r.Context(),
			chat.ForwardRequest{
				UserID:        user.ID,
				Message:       message,
				ConnectionIDs: connectionIDs,
				Runtime:       chatModel.RuntimeConfig(),
				MessageID:     assistantMessage.ID,
				SkillID:       skillID,
				Skill:         skill,
				CustomPrompt:  customPrompt,
				SkillSnapshot: skillSnapshot,
			},
			func(items []domain.Evidence) error {
				sources = append([]domain.Evidence(nil), items...)
				streamJSON(w, flusher, map[string]any{
					"type":    "retrieval",
					"skill":   skill,
					"sources": toChatSources(sources),
				})
				return nil
			},
			func(chunk string) error {
				streamJSON(w, flusher, map[string]any{
					"type":    "chunk",
					"content": chunk,
				})
				return nil
			},
		)
	} else {
		if useKnowledge {
			sources, err = h.chatService.Retrieve(r.Context(), user.ID, message, connectionIDs)
			if err != nil {
				streamInternalError(w, flusher, err.Error())
				return
			}
		}

		streamJSON(w, flusher, map[string]any{
			"type":    "retrieval",
			"skill":   skill,
			"sources": toChatSources(sources),
		})

		answer, err = h.chatService.Stream(
			r.Context(),
			chatModel.RuntimeConfig(),
			message,
			skill,
			customPrompt,
			sources,
			func(chunk string) error {
				streamJSON(w, flusher, map[string]any{
					"type":    "chunk",
					"content": chunk,
				})
				return nil
			},
		)
	}
	if err != nil {
		finishedAt := time.Now().UTC()
		assistantMessage.Content = err.Error()
		assistantMessage.CompletedAt = &finishedAt
		_ = h.store.SaveChatMessage(r.Context(), assistantMessage)
		streamChatExecutionError(w, flusher, err)
		return
	}

	if len(sources) > 0 {
		if err := h.saveChatSources(r.Context(), assistantMessage.ID, sources); err != nil {
			streamInternalError(w, flusher, err.Error())
			return
		}
	}

	if useForward && len(sources) == 0 {
		streamJSON(w, flusher, map[string]any{
			"type":    "retrieval",
			"skill":   skill,
			"sources": []map[string]any{},
		})
	}

	finishedAt := time.Now().UTC()
	assistantMessage.Content = answer
	assistantMessage.CompletedAt = &finishedAt
	if err := h.store.SaveChatMessage(r.Context(), assistantMessage); err != nil {
		streamInternalError(w, flusher, err.Error())
		return
	}
	session.LastMessageAt = &finishedAt
	session.UpdatedAt = finishedAt
	if err := h.store.UpdateChatSession(r.Context(), session); err != nil {
		streamInternalError(w, flusher, err.Error())
		return
	}

	streamJSON(w, flusher, map[string]any{
		"type":    "done",
		"content": answer,
		"sources": toChatSources(sources),
	})
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
	now := time.Now().UTC()
	session := domain.ChatSession{
		ID:        uuid.NewString(),
		UserID:    user.ID,
		Title:     fallbackSessionTitle(strings.TrimSpace(req.Title), ""),
		Pinned:    false,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := h.store.CreateChatSession(r.Context(), session); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, session)
}

func (h *Handler) listChatSessions(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	sessions, err := h.store.ListChatSessions(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": sessions})
}

func (h *Handler) updateChatSession(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	sessionID := chi.URLParam(r, "sessionID")
	session, err := h.store.GetChatSession(r.Context(), user.ID, sessionID)
	if err != nil {
		writeNotFound(w, "chat session not found", "chat_session")
		return
	}

	var req struct {
		Title  *string `json:"title"`
		Pinned *bool   `json:"pinned"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeInvalidRequestBody(w)
		return
	}
	if req.Title != nil {
		session.Title = fallbackSessionTitle(strings.TrimSpace(*req.Title), "")
	}
	if req.Pinned != nil {
		session.Pinned = *req.Pinned
	}
	session.UpdatedAt = time.Now().UTC()
	if err := h.store.UpdateChatSession(r.Context(), session); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (h *Handler) deleteChatSession(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	sessionID := chi.URLParam(r, "sessionID")
	if err := h.store.DeleteChatSession(r.Context(), user.ID, sessionID); err != nil {
		writeNotFound(w, "chat session not found", "chat_session")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listChatMessages(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	sessionID := chi.URLParam(r, "sessionID")
	messages, err := h.store.ListChatMessages(r.Context(), user.ID, sessionID)
	if err != nil {
		writeNotFound(w, "chat session not found", "chat_session")
		return
	}
	items := make([]map[string]any, 0, len(messages))
	for _, message := range messages {
		sources, err := h.store.ListChatMessageSources(r.Context(), message.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		items = append(items, map[string]any{
			"id":           message.ID,
			"sessionId":    message.SessionID,
			"role":         message.Role,
			"content":      message.Content,
			"skill":        message.Skill,
			"useKnowledge": message.UseKnowledge,
			"createdAt":    message.CreatedAt,
			"completedAt":  message.CompletedAt,
			"sources":      toChatSourcesFromStored(sources),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) ensureChatSession(ctx context.Context, userID, sessionID, firstMessage string) (domain.ChatSession, error) {
	if strings.TrimSpace(sessionID) != "" {
		session, err := h.store.GetChatSession(ctx, userID, sessionID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return domain.ChatSession{}, errors.New("chat session not found")
			}
			return domain.ChatSession{}, err
		}
		return session, nil
	}

	now := time.Now().UTC()
	session := domain.ChatSession{
		ID:        uuid.NewString(),
		UserID:    userID,
		Title:     fallbackSessionTitle("", firstMessage),
		Pinned:    false,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := h.store.CreateChatSession(ctx, session); err != nil {
		return domain.ChatSession{}, err
	}
	return session, nil
}

func fallbackSessionTitle(title, firstMessage string) string {
	if strings.TrimSpace(title) != "" {
		return title
	}
	if strings.TrimSpace(firstMessage) != "" {
		runes := []rune(strings.TrimSpace(firstMessage))
		if len(runes) > 24 {
			return string(runes[:24]) + "..."
		}
		return string(runes)
	}
	return "New chat"
}

func (h *Handler) saveChatSources(ctx context.Context, messageID string, sources []domain.Evidence) error {
	records := make([]domain.ChatMessageSource, 0, len(sources))
	for _, source := range sources {
		records = append(records, domain.ChatMessageSource{
			ID:           uuid.NewString(),
			MessageID:    messageID,
			Provider:     source.Provider,
			ConnectionID: source.ConnectionID,
			DocumentID:   source.DocumentID,
			ChunkID:      source.ChunkID,
			Title:        source.Title,
			Repo:         source.Repo,
			URL:          source.URL,
			Snippet:      source.Snippet,
			MatchedLines: append([]string(nil), source.MatchedLines...),
			Score:        source.Score,
			CreatedAt:    time.Now().UTC(),
		})
	}
	return h.store.SaveChatMessageSources(ctx, messageID, records)
}

func toChatSources(evidences []domain.Evidence) []map[string]any {
	items := make([]map[string]any, 0, len(evidences))
	for _, evidence := range evidences {
		items = append(items, map[string]any{
			"provider":             evidence.Provider,
			"title":                evidence.Title,
			"repo":                 evidence.Repo,
			"url":                  evidence.URL,
			"snippet":              evidence.Snippet,
			"matched_answer_lines": evidence.MatchedLines,
			"score":                evidence.Score,
		})
	}
	return items
}

func toChatSourcesFromStored(sources []domain.ChatMessageSource) []map[string]any {
	items := make([]map[string]any, 0, len(sources))
	for _, source := range sources {
		items = append(items, map[string]any{
			"provider":             source.Provider,
			"title":                source.Title,
			"repo":                 source.Repo,
			"url":                  source.URL,
			"snippet":              source.Snippet,
			"matched_answer_lines": source.MatchedLines,
			"score":                source.Score,
		})
	}
	return items
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
