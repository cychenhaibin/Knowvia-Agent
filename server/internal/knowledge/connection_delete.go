package knowledge

import (
	"context"
	"errors"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/persistence"
)

func (s *Service) DeleteConnection(ctx context.Context, userID, connectionID string) error {
	connection, err := s.connectionStore.GetKnowledgeConnection(ctx, userID, connectionID)
	if err != nil {
		if errors.Is(err, persistence.ErrNotFound) {
			return ErrConnectionNotFound
		}
		return err
	}

	jobs, err := s.syncJobStore.ListKnowledgeSyncJobs(ctx, userID, connectionID)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		if job.Status == domain.SyncJobQueued || job.Status == domain.SyncJobRunning {
			return ErrActiveSyncJob
		}
	}

	if err := s.connectionStore.DeleteKnowledgeConnection(ctx, userID, connectionID); err != nil {
		if errors.Is(err, persistence.ErrNotFound) {
			return ErrConnectionNotFound
		}
		return err
	}
	if s.mirrorClient != nil {
		if err := s.mirrorClient.DeleteKnowledge(ctx, connection.UserID, connection.ID); err != nil {
			return &MirrorSyncError{Err: err}
		}
	}
	return nil
}
