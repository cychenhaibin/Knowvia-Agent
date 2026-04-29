package pythonproxy

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/provider"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

type mirroredSkill struct {
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

func (c *Client) UpsertSkill(ctx context.Context, skill domain.Skill) error {
	if c == nil {
		return errors.New("python forward client is not configured")
	}
	payload := mirroredSkill{
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

func (c *Client) DeleteSkill(ctx context.Context, userID, skillID string) error {
	if c == nil {
		return errors.New("python forward client is not configured")
	}
	return c.doJSONWithRetry(ctx, http.MethodDelete, fmt.Sprintf("/internal/v1/skills/%s/%s", userID, skillID), nil, nil)
}

func (c *Client) UpsertKnowledge(
	ctx context.Context,
	req provider.MirroredKnowledgeUpsertRequest,
) (provider.MirroredKnowledgeUpsertResult, error) {
	if c == nil {
		return provider.MirroredKnowledgeUpsertResult{}, errors.New("python forward client is not configured")
	}
	var result provider.MirroredKnowledgeUpsertResult
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
		return provider.MirroredKnowledgeUpsertResult{}, err
	}
	return result, nil
}

func (c *Client) DeleteKnowledge(ctx context.Context, userID, connectionID string) error {
	if c == nil {
		return errors.New("python forward client is not configured")
	}
	return c.doJSONWithRetry(ctx, http.MethodDelete, fmt.Sprintf("/internal/v1/index/scopes/%s/%s", userID, connectionID), nil, nil)
}

func (c *Client) SyncKnowledgeSource(
	ctx context.Context,
	req provider.SyncedKnowledgeSourceRequest,
) (provider.SyncedKnowledgeSourceResult, error) {
	if c == nil {
		return provider.SyncedKnowledgeSourceResult{}, errors.New("python forward client is not configured")
	}
	var result provider.SyncedKnowledgeSourceResult
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
		return provider.SyncedKnowledgeSourceResult{}, err
	}
	return result, nil
}
