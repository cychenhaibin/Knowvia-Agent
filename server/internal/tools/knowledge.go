package tools

import (
	"context"
	"strings"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/provider"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/store"
)

// KnowledgeSearchTool is the Go-side adapter for internal knowledge retrieval.
// When the Python llm is available we treat it as the source of truth for
// scope-level retrieval, and only fall back to the local Go store when the
// forward client is unavailable or returns an error.
type KnowledgeSearchTool struct {
	store   store.Store
	forward provider.KnowledgeRetrieveForwardClient
}

func NewKnowledgeSearchTool(
	st store.Store,
	forward provider.KnowledgeRetrieveForwardClient,
) *KnowledgeSearchTool {
	return &KnowledgeSearchTool{store: st, forward: forward}
}

func (t *KnowledgeSearchTool) Search(
	ctx context.Context,
	userID string,
	connectionIDs []string,
	query string,
	limit int,
	trace *TraceContext,
) ([]domain.Evidence, error) {
	if t.forward != nil {
		result, err := t.forward.RetrieveKnowledge(ctx, provider.ForwardedRetrieveRequest{
			UserID:   userID,
			ScopeIDs: append([]string(nil), connectionIDs...),
			Query:    query,
			TopK:     limit,
			Trace:    forwardedTraceContext(trace),
		})
		if err == nil {
			return forwardedSourcesToEvidence(result.Sources), nil
		}
	}

	hits, err := t.store.SearchKnowledge(ctx, userID, connectionIDs, query, limit)
	if err != nil {
		return nil, err
	}
	evidence := make([]domain.Evidence, 0, len(hits))
	for _, hit := range hits {
		providerName := domain.ProviderYuque
		if hit.Provider == string(domain.ProviderFeishu) {
			providerName = domain.ProviderFeishu
		}
		evidence = append(evidence, domain.Evidence{
			Provider:     providerName,
			ConnectionID: hit.ConnectionID,
			DocumentID:   hit.DocumentID,
			ChunkID:      hit.ChunkID,
			Title:        hit.Title,
			Repo:         hit.Repo,
			URL:          hit.URL,
			Snippet:      hit.Snippet,
			Score:        hit.Score,
		})
	}
	return evidence, nil
}

func forwardedSourcesToEvidence(items []provider.ForwardedSource) []domain.Evidence {
	sources := make([]domain.Evidence, 0, len(items))
	for _, source := range items {
		providerName := domain.ProviderYuque
		if source.Provider == string(domain.ProviderFeishu) {
			providerName = domain.ProviderFeishu
		}
		if source.Type != "knowledge_base" && source.Provider == "" {
			providerName = domain.ProviderWeb
		}
		sources = append(sources, domain.Evidence{
			Provider:     providerName,
			ConnectionID: firstNonEmpty(source.ConnectionID, source.ScopeID),
			DocumentID:   source.DocumentID,
			ChunkID:      source.ChunkID,
			Title:        source.Title,
			Repo:         source.Repo,
			URL:          source.URL,
			Snippet:      source.Snippet,
			Score:        source.Score,
		})
	}
	return sources
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func forwardedTraceContext(trace *TraceContext) *provider.ForwardedTraceContext {
	if trace == nil {
		return nil
	}
	return &provider.ForwardedTraceContext{
		TraceID:   strings.TrimSpace(trace.TraceID),
		RunID:     strings.TrimSpace(trace.RunID),
		MessageID: strings.TrimSpace(trace.MessageID),
		SkillID:   strings.TrimSpace(trace.SkillID),
	}
}
