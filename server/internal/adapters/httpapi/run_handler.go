package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/run"
)

func (h *Handler) listRuns(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	runs, err := h.runService.ListRuns(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, runListDTO{Items: mapRuns(runs)})
}

func (h *Handler) createRun(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	var req struct {
		Title                  string         `json:"title"`
		Goal                   string         `json:"goal"`
		Mode                   domain.RunMode `json:"mode"`
		KnowledgeConnectionIDs []string       `json:"knowledge_connection_ids"`
		SkillInstallationID    string         `json:"skill_installation_id"`
		SkillDefinitionID      string         `json:"skill_definition_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeInvalidRequestBody(w)
		return
	}
	if strings.TrimSpace(req.Goal) == "" {
		writeValidationError(w, "goal is required", []string{"goal"})
		return
	}
	created, err := h.runService.CreateRun(r.Context(), user.ID, run.CreateRunInput{
		Title:                  req.Title,
		Goal:                   req.Goal,
		Mode:                   req.Mode,
		KnowledgeConnectionIDs: append([]string(nil), req.KnowledgeConnectionIDs...),
		SkillInstallationID:    req.SkillInstallationID,
		SkillDefinitionID:      req.SkillDefinitionID,
	})
	if err != nil {
		if writeSkillSelectionError(w, err) {
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, mapRun(created))
}

func (h *Handler) getRun(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	details, err := h.runService.GetRunDetails(r.Context(), user.ID, chi.URLParam(r, "runID"))
	if err != nil {
		if errors.Is(err, run.ErrRunNotFound) {
			writeNotFound(w, "run not found", "run")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, mapRunDetails(details))
}

func (h *Handler) runEvents(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	runID := chi.URLParam(r, "runID")
	if err := h.runService.EnsureRunAccess(r.Context(), user.ID, runID); err != nil {
		if errors.Is(err, run.ErrRunNotFound) {
			writeNotFound(w, "run not found", "run")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
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

	stream, unsubscribe := h.runService.SubscribeEvents(runID)
	defer unsubscribe()

	fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			fmt.Fprint(w, ": keepalive\n\n")
			flusher.Flush()
		case event := <-stream:
			payload, _ := json.Marshal(mapRunEvent(event))
			fmt.Fprintf(w, "data: %s\n\n", payload)
			flusher.Flush()
		}
	}
}
