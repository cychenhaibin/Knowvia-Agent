package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	neturl "net/url"
	"strings"
	"sync"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/config"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

type ForwardedSource struct {
	Type         string   `json:"type"`
	Provider     string   `json:"provider,omitempty"`
	ScopeID      string   `json:"scope_id,omitempty"`
	ConnectionID string   `json:"connection_id,omitempty"`
	DocumentID   string   `json:"document_id,omitempty"`
	ChunkID      string   `json:"chunk_id,omitempty"`
	Title        string   `json:"title"`
	URL          string   `json:"url,omitempty"`
	Repo         string   `json:"repo,omitempty"`
	Snippet      string   `json:"snippet,omitempty"`
	MatchedLines []string `json:"matched_answer_lines,omitempty"`
	Score        float64  `json:"score,omitempty"`
}

type ForwardedModelProfile struct {
	ProfileID   string  `json:"profile_id,omitempty"`
	Purpose     string  `json:"purpose,omitempty"`
	Provider    string  `json:"provider,omitempty"`
	Name        string  `json:"name,omitempty"`
	BaseURL     string  `json:"base_url,omitempty"`
	APIKey      string  `json:"api_key,omitempty"`
	ModelName   string  `json:"model_name,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
}

type ForwardedTraceContext struct {
	TraceID   string `json:"trace_id,omitempty"`
	RunID     string `json:"run_id,omitempty"`
	MessageID string `json:"message_id,omitempty"`
	SkillID   string `json:"skill_id,omitempty"`
}

type ForwardedMetrics struct {
	RetrieveMS int `json:"retrieve_ms,omitempty"`
	GenerateMS int `json:"generate_ms,omitempty"`
	TotalMS    int `json:"total_ms,omitempty"`
}

type ForwardedChatRequest struct {
	UserID        string                  `json:"user_id"`
	Message       string                  `json:"message"`
	ConnectionIDs []string                `json:"scope_ids,omitempty"`
	ChatModel     string                  `json:"chat_model,omitempty"`
	ChatAPIBase   string                  `json:"chat_api_base,omitempty"`
	ChatAPIKey    string                  `json:"chat_api_key,omitempty"`
	SkillID       string                  `json:"skill_id,omitempty"`
	SkillPrompt   string                  `json:"skill_prompt,omitempty"`
	Mode          string                  `json:"mode"`
	SkillSnapshot *ForwardedSkillSnapshot `json:"skill_snapshot,omitempty"`
	ModelProfile  *ForwardedModelProfile  `json:"model_profile,omitempty"`
	Trace         *ForwardedTraceContext  `json:"trace,omitempty"`
}

type ForwardedSkillSnapshot struct {
	SnapshotID     string         `json:"snapshot_id"`
	InstallationID string         `json:"installation_id,omitempty"`
	DefinitionID   string         `json:"definition_id,omitempty"`
	RevisionID     string         `json:"revision_id,omitempty"`
	Kind           string         `json:"kind,omitempty"`
	Title          string         `json:"title,omitempty"`
	Description    string         `json:"description,omitempty"`
	Mode           string         `json:"mode,omitempty"`
	Prompt         string         `json:"prompt,omitempty"`
	RuntimeSpec    map[string]any `json:"runtime_spec,omitempty"`
}

type ForwardedChatResult struct {
	TraceID string            `json:"trace_id,omitempty"`
	Answer  string            `json:"answer"`
	Sources []ForwardedSource `json:"sources,omitempty"`
	Metrics ForwardedMetrics  `json:"metrics,omitempty"`
}

type ForwardedRetrieveRequest struct {
	UserID   string                 `json:"user_id"`
	ScopeIDs []string               `json:"scope_ids,omitempty"`
	Query    string                 `json:"query"`
	TopK     int                    `json:"top_k,omitempty"`
	Trace    *ForwardedTraceContext `json:"trace,omitempty"`
}

type ForwardedRetrieveResult struct {
	TraceID string            `json:"trace_id,omitempty"`
	Query   string            `json:"query"`
	Sources []ForwardedSource `json:"sources,omitempty"`
	Metrics ForwardedMetrics  `json:"metrics,omitempty"`
}

type ForwardedMergedEvidence struct {
	Provider     string  `json:"provider,omitempty"`
	ConnectionID string  `json:"connection_id,omitempty"`
	DocumentID   string  `json:"document_id,omitempty"`
	ChunkID      string  `json:"chunk_id,omitempty"`
	Title        string  `json:"title"`
	Repo         string  `json:"repo,omitempty"`
	URL          string  `json:"url,omitempty"`
	Snippet      string  `json:"snippet,omitempty"`
	Body         string  `json:"body,omitempty"`
	Score        float64 `json:"score,omitempty"`
}

type ForwardedEvidenceMergeRequest struct {
	UserID        string                  `json:"user_id"`
	Goal          string                  `json:"goal"`
	Mode          string                  `json:"mode"`
	SkillSnapshot *ForwardedSkillSnapshot `json:"skill_snapshot,omitempty"`
	Evidences     []ForwardedEvidence     `json:"evidences,omitempty"`
	TopK          int                     `json:"top_k,omitempty"`
}

type ForwardedEvidenceMergeResult struct {
	Goal           string                    `json:"goal"`
	Mode           string                    `json:"mode"`
	Evidences      []ForwardedMergedEvidence `json:"evidences,omitempty"`
	DuplicateCount int                       `json:"duplicate_count,omitempty"`
	GroupCount     int                       `json:"group_count,omitempty"`
	ConflictCount  int                       `json:"conflict_count,omitempty"`
	GroupLabels    []string                  `json:"group_labels,omitempty"`
	Warnings       []string                  `json:"warnings,omitempty"`
}

type ForwardedEvidenceDiagnostics struct {
	DuplicateCount int      `json:"duplicate_count,omitempty"`
	GroupCount     int      `json:"group_count,omitempty"`
	ConflictCount  int      `json:"conflict_count,omitempty"`
	GroupLabels    []string `json:"group_labels,omitempty"`
	Warnings       []string `json:"warnings,omitempty"`
}

type ForwardedReportRequest struct {
	UserID              string                        `json:"user_id"`
	Goal                string                        `json:"goal"`
	ScopeIDs            []string                      `json:"scope_ids,omitempty"`
	Mode                string                        `json:"mode"`
	ChatModel           string                        `json:"chat_model,omitempty"`
	ChatAPIBase         string                        `json:"chat_api_base,omitempty"`
	ChatAPIKey          string                        `json:"chat_api_key,omitempty"`
	SkillSnapshot       *ForwardedSkillSnapshot       `json:"skill_snapshot,omitempty"`
	Evidences           []ForwardedEvidence           `json:"evidences,omitempty"`
	EvidenceDiagnostics *ForwardedEvidenceDiagnostics `json:"evidence_diagnostics,omitempty"`
	ModelProfile        *ForwardedModelProfile        `json:"model_profile,omitempty"`
	Trace               *ForwardedTraceContext        `json:"trace,omitempty"`
}

type ForwardedEvidence struct {
	Provider     string  `json:"provider,omitempty"`
	ConnectionID string  `json:"connection_id,omitempty"`
	DocumentID   string  `json:"document_id,omitempty"`
	ChunkID      string  `json:"chunk_id,omitempty"`
	Title        string  `json:"title"`
	Repo         string  `json:"repo,omitempty"`
	URL          string  `json:"url,omitempty"`
	Snippet      string  `json:"snippet,omitempty"`
	Body         string  `json:"body,omitempty"`
	Score        float64 `json:"score,omitempty"`
}

type ForwardedReportResult struct {
	TraceID           string            `json:"trace_id,omitempty"`
	Summary           string            `json:"summary"`
	OutlineMarkdown   string            `json:"outline_markdown,omitempty"`
	DraftMarkdown     string            `json:"draft_markdown,omitempty"`
	RetrievalMarkdown string            `json:"retrieval_markdown,omitempty"`
	ReportMarkdown    string            `json:"report_markdown"`
	Sources           []ForwardedSource `json:"sources,omitempty"`
	Metrics           ForwardedMetrics  `json:"metrics,omitempty"`
}

func InferForwardedModelProfile(runtime domain.ChatRuntimeConfig, purpose string) *ForwardedModelProfile {
	modelName := strings.TrimSpace(runtime.ModelName)
	baseURL := strings.TrimSpace(runtime.BaseURL)
	apiKey := strings.TrimSpace(runtime.APIKey)
	if modelName == "" && baseURL == "" && apiKey == "" && strings.TrimSpace(purpose) == "" {
		return nil
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
		Temperature: runtime.Temperature,
	}
}

type MirroredSkill struct {
	ID          string `json:"id"`
	UserID      string `json:"user_id"`
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Prompt      string `json:"prompt"`
	Mode        string `json:"mode"`
	Source      string `json:"source"`
	Enabled     bool   `json:"enabled"`
	RepoURL     string `json:"repo_url,omitempty"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type MirroredConnection struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	GroupLogin string `json:"group_login"`
	Namespace  string `json:"namespace,omitempty"`
}

type MirroredKnowledgeDocument struct {
	DocID     string `json:"doc_id"`
	Title     string `json:"title"`
	Repo      string `json:"repo"`
	DocRef    string `json:"doc_ref"`
	SourceURL string `json:"source_url,omitempty"`
	UpdatedAt string `json:"updated_at"`
	RawBody   string `json:"raw_body"`
}

type MirroredKnowledgeUpsertRequest struct {
	UserID         string                      `json:"user_id"`
	ConnectionID   string                      `json:"connection_id"`
	ConnectionMeta MirroredConnection          `json:"connection_meta"`
	Documents      []MirroredKnowledgeDocument `json:"documents"`
}

type MirroredKnowledgeUpsertResult struct {
	DocumentCount    int                       `json:"document_count"`
	ChunkCount       int                       `json:"chunk_count"`
	IndexVersion     string                    `json:"index_version"`
	ChangedDocuments map[string]int            `json:"changed_documents,omitempty"`
	Documents        []SyncedKnowledgeDocument `json:"documents,omitempty"`
}

type SyncedKnowledgeSourceRequest struct {
	UserID       string `json:"user_id"`
	ConnectionID string `json:"connection_id"`
	Name         string `json:"name"`
	Provider     string `json:"provider"`
	RawPayload   any    `json:"raw_payload,omitempty"`
}

type SyncedKnowledgeDocument struct {
	DocID      string `json:"doc_id"`
	Title      string `json:"title"`
	Repo       string `json:"repo"`
	DocRef     string `json:"doc_ref"`
	SourceURL  string `json:"source_url,omitempty"`
	UpdatedAt  string `json:"updated_at"`
	ChunkCount int    `json:"chunk_count,omitempty"`
}

type SyncedKnowledgeSourceResult struct {
	DocumentCount    int                       `json:"document_count"`
	ChunkCount       int                       `json:"chunk_count"`
	IndexVersion     string                    `json:"index_version"`
	ChangedDocuments map[string]int            `json:"changed_documents,omitempty"`
	Documents        []SyncedKnowledgeDocument `json:"documents"`
}

type indexDocumentInput struct {
	ExternalID string `json:"external_id"`
	Title      string `json:"title"`
	Repo       string `json:"repo"`
	DocRef     string `json:"doc_ref"`
	SourceURL  string `json:"source_url,omitempty"`
	UpdatedAt  string `json:"updated_at"`
	RawBody    string `json:"raw_body"`
	Provider   string `json:"provider,omitempty"`
}

type indexUpsertBatchRequest struct {
	UserID    string               `json:"user_id"`
	ScopeID   string               `json:"scope_id"`
	Name      string               `json:"name"`
	ScopeType string               `json:"scope_type"`
	Provider  string               `json:"provider"`
	SyncMode  string               `json:"sync_mode,omitempty"`
	Documents []indexDocumentInput `json:"documents"`
}

type indexSyncSourceRequest struct {
	UserID     string `json:"user_id"`
	ScopeID    string `json:"scope_id"`
	Name       string `json:"name"`
	Provider   string `json:"provider"`
	RawPayload any    `json:"raw_payload,omitempty"`
}

type ForwardChatClient interface {
	StreamKnowledgeChat(
		ctx context.Context,
		req ForwardedChatRequest,
		onRetrieval func([]ForwardedSource) error,
		onDelta func(string) error,
	) (ForwardedChatResult, error)
}

type KnowledgeRetrieveForwardClient interface {
	RetrieveKnowledge(ctx context.Context, req ForwardedRetrieveRequest) (ForwardedRetrieveResult, error)
}

type EvidenceMergeForwardClient interface {
	MergeEvidence(ctx context.Context, req ForwardedEvidenceMergeRequest) (ForwardedEvidenceMergeResult, error)
}

type ReportForwardClient interface {
	GenerateReport(ctx context.Context, req ForwardedReportRequest) (ForwardedReportResult, error)
}

type MirrorClient interface {
	UpsertSkill(ctx context.Context, skill domain.Skill) error
	DeleteSkill(ctx context.Context, userID, skillID string) error
	UpsertKnowledge(ctx context.Context, req MirroredKnowledgeUpsertRequest) (MirroredKnowledgeUpsertResult, error)
	DeleteKnowledge(ctx context.Context, userID, connectionID string) error
}

type KnowledgeSyncClient interface {
	SyncKnowledgeSource(ctx context.Context, req SyncedKnowledgeSourceRequest) (SyncedKnowledgeSourceResult, error)
}

type PythonForwardClient struct {
	baseURL     string
	staticToken string
	username    string
	password    string
	deviceInfo  string
	httpClient  *http.Client
	syncClient  *http.Client

	mu          sync.Mutex
	cachedToken string
}

func NewPythonForwardClient(cfg config.Config) *PythonForwardClient {
	if strings.TrimSpace(cfg.PythonProxyBaseURL) == "" {
		return nil
	}
	return &PythonForwardClient{
		baseURL:     strings.TrimRight(cfg.PythonProxyBaseURL, "/"),
		staticToken: strings.TrimSpace(cfg.PythonProxyToken),
		username:    strings.TrimSpace(cfg.PythonProxyUsername),
		password:    cfg.PythonProxyPassword,
		deviceInfo:  strings.TrimSpace(cfg.PythonProxyDeviceInfo),
		httpClient: &http.Client{
			Timeout: 90 * time.Second,
		},
		syncClient: &http.Client{
			Timeout: 10 * time.Minute,
		},
	}
}

func (c *PythonForwardClient) StreamKnowledgeChat(
	ctx context.Context,
	req ForwardedChatRequest,
	onRetrieval func([]ForwardedSource) error,
	onDelta func(string) error,
) (ForwardedChatResult, error) {
	if c == nil {
		return ForwardedChatResult{}, errors.New("python forward client is not configured")
	}
	return c.streamKnowledgeChatWithRetry(ctx, req, onRetrieval, onDelta)
}

func (c *PythonForwardClient) GenerateReport(
	ctx context.Context,
	req ForwardedReportRequest,
) (ForwardedReportResult, error) {
	if c == nil {
		return ForwardedReportResult{}, errors.New("python forward client is not configured")
	}
	var result ForwardedReportResult
	if err := c.doJSONWithRetryWithClient(
		ctx,
		c.syncClient,
		http.MethodPost,
		"/internal/v1/report/generate",
		req,
		&result,
	); err != nil {
		return ForwardedReportResult{}, err
	}
	return result, nil
}

func (c *PythonForwardClient) RetrieveKnowledge(
	ctx context.Context,
	req ForwardedRetrieveRequest,
) (ForwardedRetrieveResult, error) {
	if c == nil {
		return ForwardedRetrieveResult{}, errors.New("python forward client is not configured")
	}
	var result ForwardedRetrieveResult
	if err := c.doJSONWithRetry(
		ctx,
		http.MethodPost,
		"/internal/v1/retrieve",
		req,
		&result,
	); err != nil {
		return ForwardedRetrieveResult{}, err
	}
	return result, nil
}

func (c *PythonForwardClient) MergeEvidence(
	ctx context.Context,
	req ForwardedEvidenceMergeRequest,
) (ForwardedEvidenceMergeResult, error) {
	if c == nil {
		return ForwardedEvidenceMergeResult{}, errors.New("python forward client is not configured")
	}
	var result ForwardedEvidenceMergeResult
	if err := c.doJSONWithRetry(
		ctx,
		http.MethodPost,
		"/internal/v1/evidence/merge",
		req,
		&result,
	); err != nil {
		return ForwardedEvidenceMergeResult{}, err
	}
	return result, nil
}

func (c *PythonForwardClient) UpsertSkill(ctx context.Context, skill domain.Skill) error {
	if c == nil {
		return errors.New("python forward client is not configured")
	}
	payload := MirroredSkill{
		ID:          skill.ID,
		UserID:      skill.UserID,
		Slug:        skill.Slug,
		Title:       skill.Title,
		Description: skill.Description,
		Prompt:      skill.Prompt,
		Mode:        skill.Mode,
		Source:      string(skill.Source),
		Enabled:     skill.Enabled,
		RepoURL:     skill.RepoURL,
		CreatedAt:   skill.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:   skill.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	return c.doJSONWithRetry(ctx, http.MethodPost, "/internal/v1/skills/upsert", payload, nil)
}

func (c *PythonForwardClient) DeleteSkill(ctx context.Context, userID, skillID string) error {
	if c == nil {
		return errors.New("python forward client is not configured")
	}
	return c.doJSONWithRetry(ctx, http.MethodDelete, fmt.Sprintf("/internal/v1/skills/%s/%s", userID, skillID), nil, nil)
}

func (c *PythonForwardClient) UpsertKnowledge(
	ctx context.Context,
	req MirroredKnowledgeUpsertRequest,
) (MirroredKnowledgeUpsertResult, error) {
	if c == nil {
		return MirroredKnowledgeUpsertResult{}, errors.New("python forward client is not configured")
	}
	var result MirroredKnowledgeUpsertResult
	payload := indexUpsertBatchRequest{
		UserID:    req.UserID,
		ScopeID:   req.ConnectionID,
		Name:      strings.TrimSpace(req.ConnectionMeta.Name),
		ScopeType: "knowledge_connection",
		Provider:  "knowledge",
		Documents: make([]indexDocumentInput, 0, len(req.Documents)),
	}
	for _, doc := range req.Documents {
		payload.Documents = append(payload.Documents, indexDocumentInput{
			ExternalID: strings.TrimSpace(doc.DocID),
			Title:      doc.Title,
			Repo:       doc.Repo,
			DocRef:     doc.DocRef,
			SourceURL:  doc.SourceURL,
			UpdatedAt:  doc.UpdatedAt,
			RawBody:    doc.RawBody,
		})
	}
	if err := c.doJSONWithRetryWithClient(ctx, c.syncClient, http.MethodPost, "/internal/v1/index/upsert-batch", payload, &result); err != nil {
		return MirroredKnowledgeUpsertResult{}, err
	}
	return result, nil
}

func (c *PythonForwardClient) DeleteKnowledge(ctx context.Context, userID, connectionID string) error {
	if c == nil {
		return errors.New("python forward client is not configured")
	}
	return c.doJSONWithRetry(ctx, http.MethodDelete, fmt.Sprintf("/internal/v1/index/scopes/%s/%s", userID, connectionID), nil, nil)
}

func (c *PythonForwardClient) SyncKnowledgeSource(
	ctx context.Context,
	req SyncedKnowledgeSourceRequest,
) (SyncedKnowledgeSourceResult, error) {
	if c == nil {
		return SyncedKnowledgeSourceResult{}, errors.New("python forward client is not configured")
	}
	var result SyncedKnowledgeSourceResult
	payload := indexSyncSourceRequest{
		UserID:     req.UserID,
		ScopeID:    req.ConnectionID,
		Name:       req.Name,
		Provider:   req.Provider,
		RawPayload: req.RawPayload,
	}
	if err := c.doJSONWithRetryWithClient(
		ctx,
		c.syncClient,
		http.MethodPost,
		"/internal/v1/index/sync-source",
		payload,
		&result,
	); err != nil {
		return SyncedKnowledgeSourceResult{}, err
	}
	return result, nil
}

func (c *PythonForwardClient) streamKnowledgeChatWithRetry(
	ctx context.Context,
	req ForwardedChatRequest,
	onRetrieval func([]ForwardedSource) error,
	onDelta func(string) error,
) (ForwardedChatResult, error) {
	return c.streamKnowledgeChat(ctx, req, onRetrieval, onDelta)
}

func (c *PythonForwardClient) streamKnowledgeChat(
	ctx context.Context,
	req ForwardedChatRequest,
	onRetrieval func([]ForwardedSource) error,
	onDelta func(string) error,
) (ForwardedChatResult, error) {
	token, err := c.getToken(ctx)
	if err != nil {
		return ForwardedChatResult{}, err
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return ForwardedChatResult{}, err
	}
	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/internal/v1/chat/stream",
		bytes.NewReader(payload),
	)
	if err != nil {
		return ForwardedChatResult{}, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return ForwardedChatResult{}, c.normalizeTransportError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return ForwardedChatResult{}, unauthorizedError(resp.Status)
	}
	if resp.StatusCode >= 300 {
		return ForwardedChatResult{}, httpError("python knowledge stream request failed", resp)
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 1024), 1024*1024)

	var answer strings.Builder
	sources := []ForwardedSource{}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") || !strings.HasPrefix(line, "data:") {
			continue
		}

		var event struct {
			Type    string            `json:"type"`
			Content string            `json:"content"`
			Error   any               `json:"error"`
			Sources []ForwardedSource `json:"sources"`
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return ForwardedChatResult{}, err
		}
		if message := forwardedEventErrorMessage(event.Error); message != "" {
			return ForwardedChatResult{}, errors.New(message)
		}

		switch event.Type {
		case "retrieval":
			sources = append([]ForwardedSource(nil), event.Sources...)
			if onRetrieval != nil {
				if err := onRetrieval(sources); err != nil {
					return ForwardedChatResult{Answer: answer.String(), Sources: sources}, err
				}
			}
		case "chunk":
			if event.Content != "" {
				answer.WriteString(event.Content)
				if onDelta != nil {
					if err := onDelta(event.Content); err != nil {
						return ForwardedChatResult{Answer: answer.String(), Sources: sources}, err
					}
				}
			}
		case "done":
			finalAnswer := strings.TrimSpace(answer.String())
			if strings.TrimSpace(event.Content) != "" && finalAnswer == "" {
				finalAnswer = strings.TrimSpace(event.Content)
			}
			if len(event.Sources) > 0 {
				sources = append([]ForwardedSource(nil), event.Sources...)
			}
			return ForwardedChatResult{
				Answer:  finalAnswer,
				Sources: sources,
			}, nil
		case "error":
			return ForwardedChatResult{}, errors.New(forwardedEventErrorMessage(event.Error))
		}
	}
	if err := scanner.Err(); err != nil {
		return ForwardedChatResult{}, err
	}
	return ForwardedChatResult{
		Answer:  strings.TrimSpace(answer.String()),
		Sources: sources,
	}, nil
}

func forwardedEventErrorMessage(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case map[string]any:
		if message, ok := typed["message"].(string); ok {
			return strings.TrimSpace(message)
		}
	case map[string]string:
		return strings.TrimSpace(typed["message"])
	}
	return ""
}

func (c *PythonForwardClient) doJSONWithRetry(
	ctx context.Context,
	method string,
	path string,
	body any,
	target any,
) error {
	return c.doJSONWithRetryWithClient(ctx, c.httpClient, method, path, body, target)
}

func (c *PythonForwardClient) doJSONWithRetryWithClient(
	ctx context.Context,
	client *http.Client,
	method string,
	path string,
	body any,
	target any,
) error {
	return c.doJSONWithClient(ctx, client, method, path, body, target)
}

func (c *PythonForwardClient) doJSON(
	ctx context.Context,
	method string,
	path string,
	body any,
	target any,
) error {
	return c.doJSONWithClient(ctx, c.httpClient, method, path, body, target)
}

func (c *PythonForwardClient) doJSONWithClient(
	ctx context.Context,
	client *http.Client,
	method string,
	path string,
	body any,
	target any,
) error {
	token, err := c.getToken(ctx)
	if err != nil {
		return err
	}

	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return c.normalizeTransportError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return unauthorizedError(resp.Status)
	}
	if resp.StatusCode >= 300 {
		return httpError("python proxy request failed", resp)
	}
	if target == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func (c *PythonForwardClient) getToken(ctx context.Context) (string, error) {
	_ = ctx
	if strings.TrimSpace(c.staticToken) == "" {
		return "", errors.New("python internal proxy token is not configured")
	}
	return strings.TrimSpace(c.staticToken), nil
}

func (c *PythonForwardClient) clearCachedToken() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cachedToken = ""
}

func (c *PythonForwardClient) normalizeTransportError(err error) error {
	if errors.Is(err, context.Canceled) {
		return err
	}
	if !isTransportError(err) {
		return err
	}
	return fmt.Errorf(
		"python backend unavailable at %s; start quickque-agent/llm/run_server.sh or update QQA_PYTHON_PROXY_BASE_URL",
		c.baseURL,
	)
}

func unauthorizedError(status string) error {
	return fmt.Errorf("python chat proxy unauthorized: %s", status)
}

func isUnauthorized(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "unauthorized")
}

func isTransportError(err error) bool {
	if err == nil {
		return false
	}

	var urlErr *neturl.Error
	if errors.As(err, &urlErr) {
		return true
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}

	var netErr net.Error
	return errors.As(err, &netErr)
}

func httpError(prefix string, resp *http.Response) error {
	defer resp.Body.Close()
	var payload struct {
		Error   string `json:"error"`
		Detail  string `json:"detail"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err == nil {
		message := strings.TrimSpace(payload.Error)
		if message == "" {
			message = strings.TrimSpace(payload.Detail)
		}
		if message == "" {
			message = strings.TrimSpace(payload.Message)
		}
		if message != "" {
			return fmt.Errorf("%s: %s", prefix, message)
		}
	}
	return fmt.Errorf("%s: %s", prefix, resp.Status)
}
