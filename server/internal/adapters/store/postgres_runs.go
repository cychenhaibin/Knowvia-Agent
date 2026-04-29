package store

import (
	"context"

	sqldb "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/db"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func (s *PostgresStore) CreateRun(ctx context.Context, run domain.Run) error {
	_, err := s.queries.CreateRun(ctx, sqldb.CreateRunParams{
		ID:                     run.ID,
		UserID:                 run.UserID,
		Title:                  run.Title,
		Goal:                   run.Goal,
		RequestedMode:          string(run.RequestedMode),
		KnowledgeConnectionIds: run.KnowledgeConnectionIDs,
		SkillInstallationID:    run.SkillInstallationID,
		SkillSnapshotID:        run.SkillSnapshotID,
		EffectiveMode:          string(run.EffectiveMode),
		Status:                 string(run.Status),
		LatestArtifactID:       run.LatestArtifactID,
		ErrorMessage:           run.ErrorMessage,
		CreatedAt:              pgTimestamptz(run.CreatedAt),
		UpdatedAt:              pgTimestamptz(run.UpdatedAt),
	})
	return err
}

func (s *PostgresStore) GetRun(ctx context.Context, userID, runID string) (domain.Run, error) {
	row, err := s.queries.GetRun(ctx, sqldb.GetRunParams{ID: runID, UserID: userID})
	if err != nil {
		return domain.Run{}, normalizeError(err)
	}
	return mapDBRun(
		row.ID, row.UserID, row.Title, row.Goal, row.RequestedMode, row.KnowledgeConnectionIds,
		row.SkillInstallationID, row.SkillSnapshotID, row.EffectiveMode, row.Status,
		row.LatestArtifactID, row.ErrorMessage, row.CreatedAt, row.UpdatedAt,
	), nil
}

func (s *PostgresStore) GetRunByID(ctx context.Context, runID string) (domain.Run, error) {
	row, err := s.queries.GetRunById(ctx, runID)
	if err != nil {
		return domain.Run{}, normalizeError(err)
	}
	return mapDBRun(
		row.ID, row.UserID, row.Title, row.Goal, row.RequestedMode, row.KnowledgeConnectionIds,
		row.SkillInstallationID, row.SkillSnapshotID, row.EffectiveMode, row.Status,
		row.LatestArtifactID, row.ErrorMessage, row.CreatedAt, row.UpdatedAt,
	), nil
}

func (s *PostgresStore) UpdateRun(ctx context.Context, run domain.Run) error {
	rowsAffected, err := s.queries.UpdateRun(ctx, sqldb.UpdateRunParams{
		ID:                     run.ID,
		UserID:                 run.UserID,
		Title:                  run.Title,
		Goal:                   run.Goal,
		RequestedMode:          string(run.RequestedMode),
		KnowledgeConnectionIds: run.KnowledgeConnectionIDs,
		SkillInstallationID:    run.SkillInstallationID,
		SkillSnapshotID:        run.SkillSnapshotID,
		EffectiveMode:          string(run.EffectiveMode),
		Status:                 string(run.Status),
		LatestArtifactID:       run.LatestArtifactID,
		ErrorMessage:           run.ErrorMessage,
		CreatedAt:              pgTimestamptz(run.CreatedAt),
		UpdatedAt:              pgTimestamptz(run.UpdatedAt),
	})
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) ListRuns(ctx context.Context, userID string) ([]domain.Run, error) {
	rows, err := s.queries.ListRuns(ctx, userID)
	if err != nil {
		return nil, err
	}
	runs := make([]domain.Run, 0, len(rows))
	for _, row := range rows {
		runs = append(runs, mapDBRun(
			row.ID, row.UserID, row.Title, row.Goal, row.RequestedMode, row.KnowledgeConnectionIds,
			row.SkillInstallationID, row.SkillSnapshotID, row.EffectiveMode, row.Status,
			row.LatestArtifactID, row.ErrorMessage, row.CreatedAt, row.UpdatedAt,
		))
	}
	return runs, nil
}

func mapDBRun(id, userID, title, goal, requestedMode string, knowledgeConnectionIDs []string, skillInstallationID, skillSnapshotID, effectiveMode, status, latestArtifactID, errorMessage string, createdAt, updatedAt pgtype.Timestamptz) domain.Run {
	return domain.Run{
		ID:                     id,
		UserID:                 userID,
		Title:                  title,
		Goal:                   goal,
		RequestedMode:          domain.RunMode(requestedMode),
		KnowledgeConnectionIDs: append([]string(nil), knowledgeConnectionIDs...),
		SkillInstallationID:    skillInstallationID,
		SkillSnapshotID:        skillSnapshotID,
		EffectiveMode:          domain.RunMode(effectiveMode),
		Status:                 domain.RunStatus(status),
		LatestArtifactID:       latestArtifactID,
		ErrorMessage:           errorMessage,
		CreatedAt:              pgTimestamptzValue(createdAt),
		UpdatedAt:              pgTimestamptzValue(updatedAt),
	}
}
