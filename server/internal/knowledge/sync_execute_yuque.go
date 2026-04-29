package knowledge

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/provider"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/provider/yuque"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func (s *Service) executeYuqueSync(
	ctx context.Context,
	connection domain.KnowledgeConnection,
	job domain.KnowledgeSyncJob,
) error {
	if connection.Yuque == nil {
		return s.failJob(ctx, job, fmt.Errorf("missing yuque config"))
	}
	if s.mirrorClient == nil {
		return s.failJob(
			ctx,
			job,
			fmt.Errorf("python knowledge upsert unavailable; set QQA_PYTHON_PROXY_BASE_URL and start quickque-agent/llm/run_server.sh"),
		)
	}

	client := yuque.NewClient()
	namespace := normalizeNamespace(connection.Yuque.GroupLogin, connection.Yuque.Namespace)

	storedDocs, err := s.metadataStore.ListKnowledgeSourceDocuments(ctx, connection.ID)
	if err != nil {
		return s.failJob(ctx, job, err)
	}
	storedByExternalID := make(map[string]domain.KnowledgeSourceDocument, len(storedDocs))
	for _, doc := range storedDocs {
		storedByExternalID[doc.ExternalID] = doc
	}

	now := time.Now().UTC()
	if len(connection.Yuque.PendingDocs) > 0 {
		return s.executeYuquePendingSync(ctx, connection, job, namespace, storedDocs, storedByExternalID, now, client)
	}

	currentDocs, err := client.ListDocs(ctx, connection.Yuque.Token, namespace)
	if err != nil {
		return s.failJob(ctx, job, err)
	}

	prepared, err := prepareYuqueSyncDocuments(
		currentDocs,
		storedByExternalID,
		connection,
		namespace,
		now,
		func(slug string) (string, error) {
			return client.GetDocBody(ctx, connection.Yuque.Token, namespace, slug)
		},
	)
	if err != nil {
		return s.failJob(ctx, job, err)
	}

	sort.Slice(prepared.upsertDocs, func(i, j int) bool {
		if prepared.upsertDocs[i].Repo == prepared.upsertDocs[j].Repo {
			return prepared.upsertDocs[i].DocRef < prepared.upsertDocs[j].DocRef
		}
		return prepared.upsertDocs[i].Repo < prepared.upsertDocs[j].Repo
	})
	deletedCount := 0
	deletedTitles := make([]string, 0)
	for _, doc := range storedDocs {
		if _, exists := prepared.currentIDs[doc.ExternalID]; !exists {
			deletedCount++
			deletedTitles = append(deletedTitles, strings.TrimSpace(doc.Title))
		}
	}

	result, err := s.mirrorClient.UpsertKnowledge(ctx, provider.MirroredKnowledgeUpsertRequest{
		UserID:       connection.UserID,
		ConnectionID: connection.ID,
		ConnectionMeta: provider.MirroredConnection{
			ID:         connection.ID,
			Name:       connection.Name,
			GroupLogin: connection.Yuque.GroupLogin,
			Namespace:  namespace,
		},
		Documents: prepared.upsertDocs,
	})
	if err != nil {
		return s.failJob(ctx, job, err)
	}

	if err := s.metadataStore.ReplaceKnowledgeSourceDocuments(ctx, connection.ID, prepared.mergedDocs); err != nil {
		return s.failJob(ctx, job, err)
	}
	metadata := toKnowledgeMetadata(connection, result.Documents)
	if err := s.metadataStore.ReplaceKnowledgeMetadata(ctx, connection.ID, metadata); err != nil {
		return s.failJob(ctx, job, err)
	}

	finishedAt := time.Now().UTC()
	connection.LastSyncedAt = &finishedAt
	connection.UpdatedAt = finishedAt
	job.Status = domain.SyncJobCompleted
	job.FinishedAt = &finishedAt
	connection.Yuque.PendingDocs = nil
	job.Summary = marshalIncrementalSyncSummary(incrementalSyncSummary{
		Kind:          "incremental_sync",
		Documents:     result.DocumentCount,
		Chunks:        result.ChunkCount,
		Added:         prepared.addedCount,
		Updated:       prepared.updatedCount,
		Deleted:       deletedCount,
		Unchanged:     prepared.unchangedCount,
		AddedTitles:   limitSummaryTitles(prepared.addedTitles),
		UpdatedTitles: limitSummaryTitles(prepared.updatedTitles),
		DeletedTitles: limitSummaryTitles(deletedTitles),
	})
	if prepared.rateLimitErr != nil {
		connection.Yuque.PendingDocs = prepared.deferredDocs
		job.Summary = marshalIncrementalSyncSummary(incrementalSyncSummary{
			Kind:           "incremental_sync_rate_limited",
			Documents:      result.DocumentCount,
			Chunks:         result.ChunkCount,
			Added:          prepared.addedCount,
			Updated:        prepared.updatedCount,
			Deleted:        deletedCount,
			Unchanged:      prepared.unchangedCount,
			Deferred:       prepared.deferredCount,
			RetryAfterSec:  retryAfterSeconds(prepared.rateLimitErr.RetryAfter),
			AddedTitles:    limitSummaryTitles(prepared.addedTitles),
			UpdatedTitles:  limitSummaryTitles(prepared.updatedTitles),
			DeletedTitles:  limitSummaryTitles(deletedTitles),
			DeferredTitles: limitSummaryTitles(prepared.deferredTitles),
		})
	} else if result.DocumentCount == 0 {
		job.Summary = marshalIncrementalSyncSummary(incrementalSyncSummary{
			Kind:          "incremental_sync_empty",
			Deleted:       deletedCount,
			DeletedTitles: limitSummaryTitles(deletedTitles),
		})
	}

	if err := s.connectionStore.UpdateKnowledgeConnection(ctx, connection); err != nil {
		return err
	}
	return s.syncJobStore.UpdateKnowledgeSyncJob(ctx, job)
}

func (s *Service) executeYuquePendingSync(
	ctx context.Context,
	connection domain.KnowledgeConnection,
	job domain.KnowledgeSyncJob,
	namespace string,
	storedDocs []domain.KnowledgeSourceDocument,
	storedByExternalID map[string]domain.KnowledgeSourceDocument,
	now time.Time,
	client *yuque.Client,
) error {
	prepared, err := prepareYuquePendingSyncDocuments(
		connection.Yuque.PendingDocs,
		storedDocs,
		storedByExternalID,
		connection,
		namespace,
		now,
		func(slug string) (string, error) {
			return client.GetDocBody(ctx, connection.Yuque.Token, namespace, slug)
		},
	)
	if err != nil {
		return s.failJob(ctx, job, err)
	}

	sort.Slice(prepared.upsertDocs, func(i, j int) bool {
		if prepared.upsertDocs[i].Repo == prepared.upsertDocs[j].Repo {
			return prepared.upsertDocs[i].DocRef < prepared.upsertDocs[j].DocRef
		}
		return prepared.upsertDocs[i].Repo < prepared.upsertDocs[j].Repo
	})

	result, err := s.mirrorClient.UpsertKnowledge(ctx, provider.MirroredKnowledgeUpsertRequest{
		UserID:       connection.UserID,
		ConnectionID: connection.ID,
		ConnectionMeta: provider.MirroredConnection{
			ID:         connection.ID,
			Name:       connection.Name,
			GroupLogin: connection.Yuque.GroupLogin,
			Namespace:  namespace,
		},
		Documents: prepared.upsertDocs,
	})
	if err != nil {
		return s.failJob(ctx, job, err)
	}

	if err := s.metadataStore.ReplaceKnowledgeSourceDocuments(ctx, connection.ID, prepared.mergedDocs); err != nil {
		return s.failJob(ctx, job, err)
	}
	metadata := toKnowledgeMetadata(connection, result.Documents)
	if err := s.metadataStore.ReplaceKnowledgeMetadata(ctx, connection.ID, metadata); err != nil {
		return s.failJob(ctx, job, err)
	}

	finishedAt := time.Now().UTC()
	connection.LastSyncedAt = &finishedAt
	connection.UpdatedAt = finishedAt
	job.Status = domain.SyncJobCompleted
	job.FinishedAt = &finishedAt
	connection.Yuque.PendingDocs = nil
	job.Summary = marshalIncrementalSyncSummary(incrementalSyncSummary{
		Kind:          "incremental_sync",
		Documents:     result.DocumentCount,
		Chunks:        result.ChunkCount,
		Added:         prepared.addedCount,
		Updated:       prepared.updatedCount,
		Deleted:       0,
		AddedTitles:   limitSummaryTitles(prepared.addedTitles),
		UpdatedTitles: limitSummaryTitles(prepared.updatedTitles),
	})
	if prepared.rateLimitErr != nil {
		connection.Yuque.PendingDocs = prepared.deferredDocs
		job.Summary = marshalIncrementalSyncSummary(incrementalSyncSummary{
			Kind:           "incremental_sync_rate_limited",
			Documents:      result.DocumentCount,
			Chunks:         result.ChunkCount,
			Added:          prepared.addedCount,
			Updated:        prepared.updatedCount,
			Deleted:        0,
			Deferred:       prepared.deferredCount,
			RetryAfterSec:  retryAfterSeconds(prepared.rateLimitErr.RetryAfter),
			AddedTitles:    limitSummaryTitles(prepared.addedTitles),
			UpdatedTitles:  limitSummaryTitles(prepared.updatedTitles),
			DeferredTitles: limitSummaryTitles(prepared.deferredTitles),
		})
	}

	if err := s.connectionStore.UpdateKnowledgeConnection(ctx, connection); err != nil {
		return err
	}
	return s.syncJobStore.UpdateKnowledgeSyncJob(ctx, job)
}
