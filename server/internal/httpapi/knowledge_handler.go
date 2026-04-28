package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/store"
)

func normalizeFeishuDocumentID(values ...string) string {
	for _, raw := range values {
		candidate := strings.TrimSpace(raw)
		if candidate == "" {
			continue
		}
		if !strings.Contains(candidate, "://") {
			return candidate
		}

		parsed, err := url.Parse(candidate)
		if err != nil {
			continue
		}

		if queryValue := strings.TrimSpace(parsed.Query().Get("document_id")); queryValue != "" {
			return queryValue
		}

		path := strings.Trim(parsed.Path, "/")
		segments := strings.Split(path, "/")
		for index, segment := range segments {
			if index+1 >= len(segments) {
				continue
			}
			token := strings.TrimSpace(segments[index+1])
			if token == "" {
				continue
			}
			switch segment {
			case "docx":
				return token
			case "wiki":
				return "wiki:" + token
			}
		}
	}

	return ""
}

func normalizeFeishuWikiToken(values ...string) string {
	for _, raw := range values {
		candidate := strings.TrimSpace(raw)
		if candidate == "" {
			continue
		}
		if !strings.Contains(candidate, "://") {
			return strings.TrimPrefix(candidate, "wiki:")
		}

		parsed, err := url.Parse(candidate)
		if err != nil {
			continue
		}

		if queryValue := strings.TrimSpace(parsed.Query().Get("wiki")); queryValue != "" {
			return queryValue
		}

		path := strings.Trim(parsed.Path, "/")
		segments := strings.Split(path, "/")
		for index, segment := range segments {
			if segment != "wiki" || index+1 >= len(segments) {
				continue
			}
			token := strings.TrimSpace(segments[index+1])
			if token != "" {
				return token
			}
		}
	}

	return ""
}

func inferFeishuEntryType(entryType, entryToken string) string {
	normalizedType := strings.TrimSpace(strings.ToLower(entryType))
	if normalizedType == "" {
		normalizedType = "docx"
	}
	if normalizedType == "docx" {
		if wikiToken := normalizeFeishuWikiToken(entryToken); wikiToken != "" {
			return "wiki_node"
		}
	}
	return normalizedType
}

func (h *Handler) createKnowledgeConnection(w http.ResponseWriter, r *http.Request) {
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
	now := time.Now().UTC()
	providerName := domain.Provider(strings.TrimSpace(req.Provider))
	connection := domain.KnowledgeConnection{
		ID:          uuid.NewString(),
		UserID:      user.ID,
		Provider:    providerName,
		Name:        strings.TrimSpace(req.Name),
		SyncEnabled: req.SyncEnabled,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	switch providerName {
	case domain.ProviderYuque:
		var cfg struct {
			Token      string `json:"token"`
			GroupLogin string `json:"groupLogin"`
			Namespace  string `json:"namespace"`
		}
		if err := json.Unmarshal(req.Config, &cfg); err != nil {
			writeInvalidRequestBody(w)
			return
		}
		if connection.Name == "" {
			connection.Name = "Yuque Connection"
		}
		connection.Yuque = &domain.KnowledgeConnectionYuqueConfig{
			Token:      strings.TrimSpace(cfg.Token),
			GroupLogin: strings.TrimSpace(cfg.GroupLogin),
			Namespace:  strings.TrimSpace(cfg.Namespace),
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		missingFields := make([]string, 0, 3)
		if connection.Yuque.Token == "" {
			missingFields = append(missingFields, "config.token")
		}
		if connection.Yuque.GroupLogin == "" {
			missingFields = append(missingFields, "config.groupLogin")
		}
		if connection.Yuque.Namespace == "" {
			missingFields = append(missingFields, "config.namespace")
		}
		if len(missingFields) > 0 {
			writeValidationError(w, "yuque token, groupLogin and namespace are required", missingFields)
			return
		}
	case domain.ProviderFeishu:
		var cfg struct {
			AppID         string `json:"appId"`
			AppSecret     string `json:"appSecret"`
			EntryType     string `json:"entryType"`
			EntryToken    string `json:"entryToken"`
			DocumentID    string `json:"documentId"`
			DocumentURL   string `json:"documentUrl"`
			DocumentInput string `json:"documentInput"`
		}
		if err := json.Unmarshal(req.Config, &cfg); err != nil {
			writeInvalidRequestBody(w)
			return
		}
		if connection.Name == "" {
			connection.Name = "Feishu Document"
		}
		rawEntryHint := strings.TrimSpace(cfg.EntryToken)
		if rawEntryHint == "" {
			rawEntryHint = strings.TrimSpace(cfg.DocumentInput)
		}
		if rawEntryHint == "" {
			rawEntryHint = strings.TrimSpace(cfg.DocumentURL)
		}
		if rawEntryHint == "" {
			rawEntryHint = strings.TrimSpace(cfg.DocumentID)
		}
		connection.Feishu = &domain.KnowledgeConnectionFeishuConfig{
			AppID:      strings.TrimSpace(cfg.AppID),
			AppSecret:  strings.TrimSpace(cfg.AppSecret),
			EntryType:  inferFeishuEntryType(cfg.EntryType, rawEntryHint),
			EntryToken: strings.TrimSpace(cfg.EntryToken),
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		if connection.Feishu.EntryType == "" {
			connection.Feishu.EntryType = "docx"
		}
		if connection.Feishu.EntryToken == "" {
			switch connection.Feishu.EntryType {
			case "wiki_node":
				connection.Feishu.EntryToken = normalizeFeishuWikiToken(cfg.EntryToken, cfg.DocumentInput, cfg.DocumentURL, cfg.DocumentID)
			case "wiki_space":
				connection.Feishu.EntryToken = normalizeFeishuWikiToken(cfg.EntryToken, cfg.DocumentInput, cfg.DocumentURL, cfg.DocumentID)
			default:
				connection.Feishu.EntryType = "docx"
				connection.Feishu.EntryToken = normalizeFeishuDocumentID(cfg.DocumentID, cfg.DocumentURL, cfg.DocumentInput, cfg.EntryToken)
			}
		}
		missingFields := make([]string, 0, 3)
		if connection.Feishu.AppID == "" {
			missingFields = append(missingFields, "config.appId")
		}
		if connection.Feishu.AppSecret == "" {
			missingFields = append(missingFields, "config.appSecret")
		}
		if connection.Feishu.EntryToken == "" {
			missingFields = append(missingFields, "config.entryToken")
		}
		if len(missingFields) > 0 {
			writeValidationError(w, "feishu appId, appSecret and entry token are required", missingFields)
			return
		}
	default:
		writeValidationError(w, "unsupported knowledge provider", []string{"provider"})
		return
	}
	if err := h.store.CreateKnowledgeConnection(r.Context(), connection); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, scrubConnection(connection))
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
	body := map[string]any{
		"provider":    string(domain.ProviderYuque),
		"name":        req.Name,
		"syncEnabled": req.SyncEnabled,
		"config": map[string]any{
			"token":      req.Token,
			"groupLogin": req.GroupLogin,
			"namespace":  req.Namespace,
		},
	}
	raw, _ := json.Marshal(body)
	r.Body = io.NopCloser(strings.NewReader(string(raw)))
	_ = user
	h.createKnowledgeConnection(w, r)
}

func (h *Handler) listKnowledgeConnections(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	connections, err := h.store.ListKnowledgeConnections(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	items := make([]domain.KnowledgeConnection, 0, len(connections))
	for _, connection := range connections {
		items = append(items, scrubConnection(connection))
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": items})
}

func (h *Handler) listYuqueConnections(w http.ResponseWriter, r *http.Request) {
	h.listKnowledgeConnections(w, r)
}

func (h *Handler) deleteKnowledgeConnection(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	connectionID := chi.URLParam(r, "connectionID")
	connection, err := h.store.GetKnowledgeConnection(r.Context(), user.ID, connectionID)
	if err != nil {
		writeNotFound(w, "connection not found", "knowledge_connection")
		return
	}

	jobs, err := h.store.ListKnowledgeSyncJobs(r.Context(), user.ID, connectionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for _, job := range jobs {
		if job.Status == domain.SyncJobQueued || job.Status == domain.SyncJobRunning {
			writeConflict(w, "cannot delete a connection while sync is running", "knowledge_connection")
			return
		}
	}

	if err := h.store.DeleteKnowledgeConnection(r.Context(), user.ID, connectionID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeNotFound(w, "connection not found", "knowledge_connection")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.mirrorService.DeleteKnowledge(r.Context(), connection.UserID, connection.ID); err != nil {
		writeMirrorError(w, "connection was deleted locally but python mirror failed", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) deleteYuqueConnection(w http.ResponseWriter, r *http.Request) {
	h.deleteKnowledgeConnection(w, r)
}

func (h *Handler) listConnectionDocuments(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	connectionID := chi.URLParam(r, "connectionID")
	connection, err := h.store.GetKnowledgeConnection(r.Context(), user.ID, connectionID)
	if err != nil {
		writeNotFound(w, "connection not found", "knowledge_connection")
		return
	}

	docs, err := h.store.ListKnowledgeMetadata(r.Context(), connection.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(docs) == 0 {
		legacyDocs, chunks, err := h.store.GetKnowledgeCorpus(r.Context(), connection.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		sort.Slice(legacyDocs, func(i, j int) bool {
			if legacyDocs[i].Repo != legacyDocs[j].Repo {
				return legacyDocs[i].Repo < legacyDocs[j].Repo
			}
			if !legacyDocs[i].UpdatedAt.Equal(legacyDocs[j].UpdatedAt) {
				return legacyDocs[i].UpdatedAt.After(legacyDocs[j].UpdatedAt)
			}
			return legacyDocs[i].Title < legacyDocs[j].Title
		})

		type repoSummary struct {
			Name          string `json:"name"`
			DocumentCount int    `json:"documentCount"`
		}

		repoCounts := map[string]int{}
		for _, doc := range legacyDocs {
			repoCounts[doc.Repo]++
		}

		repos := make([]repoSummary, 0, len(repoCounts))
		for name, count := range repoCounts {
			repos = append(repos, repoSummary{Name: name, DocumentCount: count})
		}
		sort.Slice(repos, func(i, j int) bool {
			return repos[i].Name < repos[j].Name
		})

		writeJSON(w, http.StatusOK, map[string]any{
			"connection":    scrubConnection(connection),
			"repoCount":     len(repos),
			"documentCount": len(legacyDocs),
			"chunkCount":    len(chunks),
			"repos":         repos,
			"documents":     legacyDocs,
		})
		return
	}

	sort.Slice(docs, func(i, j int) bool {
		if docs[i].Repo != docs[j].Repo {
			return docs[i].Repo < docs[j].Repo
		}
		if !docs[i].UpdatedAt.Equal(docs[j].UpdatedAt) {
			return docs[i].UpdatedAt.After(docs[j].UpdatedAt)
		}
		return docs[i].Title < docs[j].Title
	})

	type repoSummary struct {
		Name          string `json:"name"`
		DocumentCount int    `json:"documentCount"`
	}

	repoCounts := map[string]int{}
	for _, doc := range docs {
		repoCounts[doc.Repo]++
	}

	repos := make([]repoSummary, 0, len(repoCounts))
	for name, count := range repoCounts {
		repos = append(repos, repoSummary{Name: name, DocumentCount: count})
	}
	sort.Slice(repos, func(i, j int) bool {
		return repos[i].Name < repos[j].Name
	})

	chunkCount := 0
	for _, doc := range docs {
		chunkCount += doc.ChunkCount
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"connection":    scrubConnection(connection),
		"repoCount":     len(repos),
		"documentCount": len(docs),
		"chunkCount":    chunkCount,
		"repos":         repos,
		"documents":     docs,
	})
}

func (h *Handler) triggerSync(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	connectionID := chi.URLParam(r, "connectionID")
	connection, err := h.store.GetKnowledgeConnection(r.Context(), user.ID, connectionID)
	if err != nil {
		writeNotFound(w, "connection not found", "knowledge_connection")
		return
	}
	jobs, err := h.store.ListKnowledgeSyncJobs(r.Context(), user.ID, connection.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for _, job := range jobs {
		if job.Status == domain.SyncJobQueued || job.Status == domain.SyncJobRunning {
			writeJSON(w, http.StatusAccepted, job)
			return
		}
	}

	job := domain.KnowledgeSyncJob{
		ID:           uuid.NewString(),
		UserID:       user.ID,
		ConnectionID: connection.ID,
		Status:       domain.SyncJobQueued,
		Summary:      "Queued for sync.",
		CreatedAt:    time.Now().UTC(),
	}
	if err := h.store.CreateKnowledgeSyncJob(r.Context(), job); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.dispatcher.EnqueueKnowledgeSync(r.Context(), connection.ID, job.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func (h *Handler) listSyncJobs(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	connectionID := chi.URLParam(r, "connectionID")
	jobs, err := h.store.ListKnowledgeSyncJobs(r.Context(), user.ID, connectionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": jobs})
}

func scrubConnection(connection domain.KnowledgeConnection) domain.KnowledgeConnection {
	if connection.Yuque != nil {
		connection.Yuque.Token = ""
	}
	if connection.Feishu != nil {
		connection.Feishu.AppSecret = ""
	}
	return connection
}
