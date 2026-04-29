package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/provider"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/provider/feishu"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func toKnowledgeMetadata(connection domain.KnowledgeConnection, documents []provider.SyncedKnowledgeDocument) []domain.KnowledgeDocumentMeta {
	items := make([]domain.KnowledgeDocumentMeta, 0, len(documents))
	now := time.Now().UTC()
	for _, doc := range documents {
		updatedAt, _ := time.Parse(time.RFC3339Nano, strings.TrimSpace(doc.UpdatedAt))
		if updatedAt.IsZero() {
			updatedAt, _ = time.Parse(time.RFC3339, strings.TrimSpace(doc.UpdatedAt))
		}
		if updatedAt.IsZero() {
			updatedAt = now
		}
		items = append(items, domain.KnowledgeDocumentMeta{
			ID:           strings.TrimSpace(doc.DocID),
			UserID:       connection.UserID,
			ConnectionID: connection.ID,
			Repo:         strings.TrimSpace(doc.Repo),
			Title:        strings.TrimSpace(doc.Title),
			DocRef:       strings.TrimSpace(doc.DocRef),
			SourceURL:    strings.TrimSpace(doc.SourceURL),
			ChunkCount:   doc.ChunkCount,
			UpdatedAt:    updatedAt,
			CreatedAt:    now,
		})
	}
	return items
}

func (s *Service) failJob(ctx context.Context, job domain.KnowledgeSyncJob, err error) error {
	finishedAt := time.Now().UTC()
	job.Status = domain.SyncJobFailed
	job.FinishedAt = &finishedAt
	job.Summary = err.Error()

	var feishuErr *feishu.APIError
	if errors.As(err, &feishuErr) {
		job.Summary = marshalSyncErrorSummary(syncErrorSummary{
			Kind:     "sync_error",
			Provider: string(domain.ProviderFeishu),
			Code:     feishuErr.Code,
			Message:  feishuErr.Message,
			LogID:    feishuErr.LogID,
		})
	}
	_ = s.syncJobStore.UpdateKnowledgeSyncJob(ctx, job)
	return err
}

func limitSummaryTitles(titles []string) []string {
	if len(titles) == 0 {
		return nil
	}
	filtered := make([]string, 0, len(titles))
	for _, title := range titles {
		title = strings.TrimSpace(title)
		if title == "" {
			continue
		}
		filtered = append(filtered, title)
		if len(filtered) == 5 {
			break
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

func marshalSyncErrorSummary(summary syncErrorSummary) string {
	type syncErrorSummaryJSON struct {
		Kind     string `json:"kind"`
		Provider string `json:"provider"`
		Code     string `json:"code"`
		Message  string `json:"message"`
		LogID    string `json:"logId,omitempty"`
	}
	encoded, err := json.Marshal(syncErrorSummaryJSON{
		Kind:     summary.Kind,
		Provider: summary.Provider,
		Code:     summary.Code,
		Message:  summary.Message,
		LogID:    summary.LogID,
	})
	if err != nil {
		return summary.Message
	}
	return string(encoded)
}

func marshalIncrementalSyncSummary(summary incrementalSyncSummary) string {
	type incrementalSyncSummaryJSON struct {
		Kind           string   `json:"kind"`
		Documents      int      `json:"documents,omitempty"`
		Chunks         int      `json:"chunks,omitempty"`
		Added          int      `json:"added,omitempty"`
		Updated        int      `json:"updated,omitempty"`
		Deleted        int      `json:"deleted,omitempty"`
		Unchanged      int      `json:"unchanged,omitempty"`
		Deferred       int      `json:"deferred,omitempty"`
		RetryAfterSec  int      `json:"retryAfterSec,omitempty"`
		AddedTitles    []string `json:"addedTitles,omitempty"`
		UpdatedTitles  []string `json:"updatedTitles,omitempty"`
		DeletedTitles  []string `json:"deletedTitles,omitempty"`
		DeferredTitles []string `json:"deferredTitles,omitempty"`
		SyncedTitles   []string `json:"syncedTitles,omitempty"`
	}
	payload, err := json.Marshal(incrementalSyncSummaryJSON{
		Kind:           summary.Kind,
		Documents:      summary.Documents,
		Chunks:         summary.Chunks,
		Added:          summary.Added,
		Updated:        summary.Updated,
		Deleted:        summary.Deleted,
		Unchanged:      summary.Unchanged,
		Deferred:       summary.Deferred,
		RetryAfterSec:  summary.RetryAfterSec,
		AddedTitles:    summary.AddedTitles,
		UpdatedTitles:  summary.UpdatedTitles,
		DeletedTitles:  summary.DeletedTitles,
		DeferredTitles: summary.DeferredTitles,
		SyncedTitles:   summary.SyncedTitles,
	})
	if err != nil {
		return "Knowledge sync completed."
	}
	return string(payload)
}

func limitSyncedKnowledgeTitles(documents []provider.SyncedKnowledgeDocument) []string {
	if len(documents) == 0 {
		return nil
	}
	titles := make([]string, 0, len(documents))
	for _, doc := range documents {
		title := strings.TrimSpace(doc.Title)
		if title == "" {
			continue
		}
		titles = append(titles, title)
		if len(titles) == 5 {
			break
		}
	}
	if len(titles) == 0 {
		return nil
	}
	return titles
}
