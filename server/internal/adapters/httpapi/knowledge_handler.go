package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/knowledge"
)

func (h *Handler) createKnowledgeConnection(w http.ResponseWriter, r *http.Request) {
	if h.knowledgeService == nil {
		writeError(w, http.StatusServiceUnavailable, "knowledge service unavailable")
		return
	}
	user := currentUser(r.Context())
	var req struct {
		Provider    string          `json:"provider"`
		Name        string          `json:"name"`
		SyncEnabled bool            `json:"syncEnabled"`
		Config      json.RawMessage `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeInvalidRequestBody(w)
		return
	}
	connection, err := h.knowledgeService.CreateConnection(r.Context(), knowledge.CreateConnectionInput{
		UserID:      user.ID,
		Provider:    req.Provider,
		Name:        req.Name,
		SyncEnabled: req.SyncEnabled,
		Config:      req.Config,
	})
	if err != nil {
		var validationErr *knowledge.ValidationError
		if errors.As(err, &validationErr) {
			writeValidationError(w, validationErr.Message, validationErr.Fields)
			return
		}
		if errors.Is(err, knowledge.ErrInvalidRequestBody) {
			writeInvalidRequestBody(w)
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, mapKnowledgeConnection(connection))
}

func (h *Handler) createYuqueConnection(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	var req struct {
		Name        string `json:"name"`
		Token       string `json:"token"`
		GroupLogin  string `json:"groupLogin"`
		Namespace   string `json:"namespace"`
		SyncEnabled bool   `json:"syncEnabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeInvalidRequestBody(w)
		return
	}
	connection, err := h.knowledgeService.CreateYuqueConnection(r.Context(), knowledge.CreateYuqueConnectionInput{
		UserID:      user.ID,
		Name:        req.Name,
		Token:       req.Token,
		GroupLogin:  req.GroupLogin,
		Namespace:   req.Namespace,
		SyncEnabled: req.SyncEnabled,
	})
	if err != nil {
		var validationErr *knowledge.ValidationError
		if errors.As(err, &validationErr) {
			writeValidationError(w, validationErr.Message, validationErr.Fields)
			return
		}
		if errors.Is(err, knowledge.ErrInvalidRequestBody) {
			writeInvalidRequestBody(w)
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, mapKnowledgeConnection(connection))
}

func (h *Handler) listKnowledgeConnections(w http.ResponseWriter, r *http.Request) {
	if h.knowledgeService == nil {
		writeError(w, http.StatusServiceUnavailable, "knowledge service unavailable")
		return
	}
	user := currentUser(r.Context())
	connections, err := h.knowledgeService.ListConnections(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, knowledgeConnectionListDTO{Items: mapKnowledgeConnections(connections)})
}

func (h *Handler) listYuqueConnections(w http.ResponseWriter, r *http.Request) {
	h.listKnowledgeConnections(w, r)
}

func (h *Handler) deleteKnowledgeConnection(w http.ResponseWriter, r *http.Request) {
	if h.knowledgeService == nil {
		writeError(w, http.StatusServiceUnavailable, "knowledge service unavailable")
		return
	}
	user := currentUser(r.Context())
	connectionID := chi.URLParam(r, "connectionID")
	if err := h.knowledgeService.DeleteConnection(r.Context(), user.ID, connectionID); err != nil {
		if errors.Is(err, knowledge.ErrActiveSyncJob) {
			writeConflict(w, "cannot delete a connection while sync is running", "knowledge_connection")
			return
		}
		var mirrorErr *knowledge.MirrorSyncError
		if errors.As(err, &mirrorErr) {
			writeMirrorError(w, "connection was deleted locally but python mirror failed", mirrorErr.Err)
			return
		}
		if errors.Is(err, knowledge.ErrConnectionNotFound) {
			writeNotFound(w, "connection not found", "knowledge_connection")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) deleteYuqueConnection(w http.ResponseWriter, r *http.Request) {
	h.deleteKnowledgeConnection(w, r)
}

func (h *Handler) listConnectionDocuments(w http.ResponseWriter, r *http.Request) {
	if h.knowledgeService == nil {
		writeError(w, http.StatusServiceUnavailable, "knowledge service unavailable")
		return
	}
	user := currentUser(r.Context())
	connectionID := chi.URLParam(r, "connectionID")
	result, err := h.knowledgeService.ListConnectionDocuments(r.Context(), user.ID, connectionID)
	if err != nil {
		if errors.Is(err, knowledge.ErrConnectionNotFound) {
			writeNotFound(w, "connection not found", "knowledge_connection")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, mapConnectionDocumentsResult(result))
}

func (h *Handler) triggerSync(w http.ResponseWriter, r *http.Request) {
	if h.knowledgeService == nil {
		writeError(w, http.StatusServiceUnavailable, "knowledge service unavailable")
		return
	}
	user := currentUser(r.Context())
	connectionID := chi.URLParam(r, "connectionID")
	job, err := h.knowledgeService.QueueSync(r.Context(), user.ID, connectionID)
	if err != nil {
		if errors.Is(err, knowledge.ErrConnectionNotFound) {
			writeNotFound(w, "connection not found", "knowledge_connection")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, mapKnowledgeSyncJob(job))
}

func (h *Handler) listSyncJobs(w http.ResponseWriter, r *http.Request) {
	if h.knowledgeService == nil {
		writeError(w, http.StatusServiceUnavailable, "knowledge service unavailable")
		return
	}
	user := currentUser(r.Context())
	connectionID := chi.URLParam(r, "connectionID")
	jobs, err := h.knowledgeService.ListSyncJobs(r.Context(), user.ID, connectionID)
	if err != nil {
		if errors.Is(err, knowledge.ErrConnectionNotFound) {
			writeNotFound(w, "connection not found", "knowledge_connection")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, knowledgeSyncJobListDTO{Items: mapKnowledgeSyncJobs(jobs)})
}
