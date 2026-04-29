package knowledge

import (
	"context"
	"fmt"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/provider"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/provider/feishu"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func (s *Service) ExecuteSync(ctx context.Context, connectionID, jobID string) error {
	job, err := s.syncJobStore.GetKnowledgeSyncJob(ctx, jobID)
	if err != nil {
		return err
	}
	connection, err := s.connectionStore.GetKnowledgeConnectionByID(ctx, connectionID)
	if err != nil {
		return err
	}
	if s.syncClient == nil {
		return s.failJob(
			ctx,
			job,
			fmt.Errorf(
				"python knowledge sync unavailable; set QQA_PYTHON_PROXY_BASE_URL and start quickque-agent/llm/run_server.sh",
			),
		)
	}

	now := time.Now().UTC()
	job.Status = domain.SyncJobRunning
	job.StartedAt = &now
	job.Summary = "Preparing knowledge sync."
	if err := s.syncJobStore.UpdateKnowledgeSyncJob(ctx, job); err != nil {
		return err
	}

	if connection.Provider == domain.ProviderYuque {
		return s.executeYuqueSync(ctx, connection, job)
	}

	var result provider.SyncedKnowledgeSourceResult
	switch connection.Provider {
	case domain.ProviderFeishu:
		if connection.Feishu == nil {
			return s.failJob(ctx, job, fmt.Errorf("missing feishu config"))
		}
		rawPayload, fetchErr := feishu.NewClient().FetchRawPayload(
			ctx,
			connection.Feishu.AppID,
			connection.Feishu.AppSecret,
			connection.Feishu.EntryType,
			connection.Feishu.EntryToken,
		)
		if fetchErr != nil {
			return s.failJob(ctx, job, fetchErr)
		}
		result, err = s.syncClient.SyncKnowledgeSource(ctx, provider.SyncedKnowledgeSourceRequest{
			UserID:       connection.UserID,
			ConnectionID: connection.ID,
			Name:         connection.Name,
			Provider:     string(connection.Provider),
			RawPayload:   rawPayload,
		})
	default:
		return s.failJob(ctx, job, fmt.Errorf("knowledge provider %q is not supported", connection.Provider))
	}
	if err != nil {
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
	job.Summary = marshalIncrementalSyncSummary(incrementalSyncSummary{
		Kind:         "full_sync",
		Documents:    result.DocumentCount,
		Chunks:       result.ChunkCount,
		SyncedTitles: limitSyncedKnowledgeTitles(result.Documents),
	})
	if result.DocumentCount == 0 {
		job.Summary = marshalIncrementalSyncSummary(incrementalSyncSummary{Kind: "full_sync_empty"})
	}

	if err := s.connectionStore.UpdateKnowledgeConnection(ctx, connection); err != nil {
		return err
	}
	return s.syncJobStore.UpdateKnowledgeSyncJob(ctx, job)
}
