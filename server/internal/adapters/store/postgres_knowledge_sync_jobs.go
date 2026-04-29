package store

import (
	"context"

	sqldb "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/db"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func (s *PostgresStore) CreateKnowledgeSyncJob(ctx context.Context, job domain.KnowledgeSyncJob) error {
	_, err := s.queries.CreateSyncJob(ctx, sqldb.CreateSyncJobParams{
		ID:           job.ID,
		UserID:       job.UserID,
		ConnectionID: job.ConnectionID,
		Status:       string(job.Status),
		Summary:      job.Summary,
		CreatedAt:    pgTimestamptz(job.CreatedAt),
		StartedAt:    pgNullableTimestamptz(job.StartedAt),
		FinishedAt:   pgNullableTimestamptz(job.FinishedAt),
	})
	return err
}

func (s *PostgresStore) UpdateKnowledgeSyncJob(ctx context.Context, job domain.KnowledgeSyncJob) error {
	rowsAffected, err := s.queries.UpdateSyncJob(ctx, sqldb.UpdateSyncJobParams{
		ID:           job.ID,
		UserID:       job.UserID,
		ConnectionID: job.ConnectionID,
		Status:       string(job.Status),
		Summary:      job.Summary,
		CreatedAt:    pgTimestamptz(job.CreatedAt),
		StartedAt:    pgNullableTimestamptz(job.StartedAt),
		FinishedAt:   pgNullableTimestamptz(job.FinishedAt),
	})
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) GetKnowledgeSyncJob(ctx context.Context, jobID string) (domain.KnowledgeSyncJob, error) {
	job, err := s.queries.GetSyncJob(ctx, jobID)
	if err != nil {
		return domain.KnowledgeSyncJob{}, normalizeError(err)
	}
	return mapDBKnowledgeSyncJob(job), nil
}

func (s *PostgresStore) ListKnowledgeSyncJobs(ctx context.Context, userID, connectionID string) ([]domain.KnowledgeSyncJob, error) {
	rows, err := s.queries.ListSyncJobs(ctx, sqldb.ListSyncJobsParams{
		UserID:       userID,
		ConnectionID: connectionID,
	})
	if err != nil {
		return nil, err
	}
	jobs := make([]domain.KnowledgeSyncJob, 0, len(rows))
	for _, row := range rows {
		jobs = append(jobs, mapDBKnowledgeSyncJob(row))
	}
	return jobs, nil
}

func mapDBKnowledgeSyncJob(job sqldb.KnowledgeSyncJob) domain.KnowledgeSyncJob {
	return domain.KnowledgeSyncJob{
		ID:           job.ID,
		UserID:       job.UserID,
		ConnectionID: job.ConnectionID,
		Status:       domain.SyncJobStatus(job.Status),
		Summary:      job.Summary,
		CreatedAt:    pgTimestamptzValue(job.CreatedAt),
		StartedAt:    pgNullableTimestamptzPtr(job.StartedAt),
		FinishedAt:   pgNullableTimestamptzPtr(job.FinishedAt),
	}
}
