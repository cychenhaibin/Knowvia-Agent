package store

import (
	"context"

	sqldb "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/db"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func (s *PostgresStore) CreateSkillImportJob(ctx context.Context, job domain.SkillImportJob) error {
	_, err := s.queries.CreateSkillImportJob(ctx, sqldb.CreateSkillImportJobParams{
		ID:             job.ID,
		UserID:         job.UserID,
		Source:         string(job.Source),
		Status:         string(job.Status),
		ArtifactID:     job.ArtifactID,
		DefinitionID:   job.DefinitionID,
		RevisionID:     job.RevisionID,
		InstallationID: job.InstallationID,
		ErrorMessage:   job.ErrorMessage,
		RequestJson:    job.RequestJSON,
		CreatedAt:      pgTimestamptz(job.CreatedAt),
		UpdatedAt:      pgTimestamptz(job.UpdatedAt),
		CompletedAt:    pgNullableTimestamptz(job.CompletedAt),
	})
	return normalizeError(err)
}

func (s *PostgresStore) UpdateSkillImportJob(ctx context.Context, job domain.SkillImportJob) error {
	rowsAffected, err := s.queries.UpdateSkillImportJob(ctx, sqldb.UpdateSkillImportJobParams{
		ID:             job.ID,
		Source:         string(job.Source),
		Status:         string(job.Status),
		ArtifactID:     job.ArtifactID,
		DefinitionID:   job.DefinitionID,
		RevisionID:     job.RevisionID,
		InstallationID: job.InstallationID,
		ErrorMessage:   job.ErrorMessage,
		RequestJson:    job.RequestJSON,
		UpdatedAt:      pgTimestamptz(job.UpdatedAt),
		CompletedAt:    pgNullableTimestamptz(job.CompletedAt),
		UserID:         job.UserID,
	})
	if err != nil {
		return normalizeError(err)
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) GetSkillImportJob(ctx context.Context, userID, jobID string) (domain.SkillImportJob, error) {
	row, err := s.queries.GetSkillImportJob(ctx, sqldb.GetSkillImportJobParams{ID: jobID, UserID: userID})
	if err != nil {
		return domain.SkillImportJob{}, normalizeError(err)
	}
	return mapDBSkillImportJob(row), nil
}

func (s *PostgresStore) ListSkillImportJobs(ctx context.Context, userID string) ([]domain.SkillImportJob, error) {
	rows, err := s.queries.ListSkillImportJobs(ctx, userID)
	if err != nil {
		return nil, err
	}

	items := make([]domain.SkillImportJob, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapDBSkillImportJob(row))
	}
	return items, nil
}
