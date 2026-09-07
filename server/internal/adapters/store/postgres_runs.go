package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func (s *PostgresStore) CreateRun(ctx context.Context, run domain.Run) error {
	if run.Kind == "" {
		run.Kind = domain.RunKindResearch
	}
	_, err := s.pool.Exec(ctx, `
INSERT INTO runs (
	id, user_id, kind, title, goal, source_url, task_session_id, task_prompt,
	requested_mode, knowledge_connection_ids, skill_installation_id, skill_snapshot_id,
	effective_mode, status, latest_artifact_id, error_message, created_at, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
`, run.ID, run.UserID, string(run.Kind), run.Title, run.Goal, run.SourceURL, run.TaskSessionID, run.TaskPrompt,
		string(run.RequestedMode), run.KnowledgeConnectionIDs, run.SkillInstallationID, run.SkillSnapshotID,
		string(run.EffectiveMode), string(run.Status), run.LatestArtifactID, run.ErrorMessage,
		pgTimestamptz(run.CreatedAt), pgTimestamptz(run.UpdatedAt))
	return err
}

func (s *PostgresStore) GetRun(ctx context.Context, userID, runID string) (domain.Run, error) {
	row := s.pool.QueryRow(ctx, `
SELECT id, user_id, kind, title, goal, source_url, task_session_id, task_prompt,
       requested_mode, knowledge_connection_ids, skill_installation_id, skill_snapshot_id,
       effective_mode, status, latest_artifact_id, error_message, created_at, updated_at
FROM runs
WHERE id = $1 AND user_id = $2
`, runID, userID)
	run, err := scanRun(row)
	if err != nil {
		return domain.Run{}, normalizeError(err)
	}
	return run, nil
}

func (s *PostgresStore) GetRunByID(ctx context.Context, runID string) (domain.Run, error) {
	row := s.pool.QueryRow(ctx, `
SELECT id, user_id, kind, title, goal, source_url, task_session_id, task_prompt,
       requested_mode, knowledge_connection_ids, skill_installation_id, skill_snapshot_id,
       effective_mode, status, latest_artifact_id, error_message, created_at, updated_at
FROM runs
WHERE id = $1
`, runID)
	run, err := scanRun(row)
	if err != nil {
		return domain.Run{}, normalizeError(err)
	}
	return run, nil
}

func (s *PostgresStore) UpdateRun(ctx context.Context, run domain.Run) error {
	if run.Kind == "" {
		run.Kind = domain.RunKindResearch
	}
	tag, err := s.pool.Exec(ctx, `
UPDATE runs
SET user_id = $2,
    kind = $3,
    title = $4,
    goal = $5,
    source_url = $6,
    task_session_id = $7,
    task_prompt = $8,
    requested_mode = $9,
    knowledge_connection_ids = $10,
    skill_installation_id = $11,
    skill_snapshot_id = $12,
    effective_mode = $13,
    status = $14,
    latest_artifact_id = $15,
    error_message = $16,
    created_at = $17,
    updated_at = $18
WHERE id = $1
`, run.ID, run.UserID, string(run.Kind), run.Title, run.Goal, run.SourceURL, run.TaskSessionID, run.TaskPrompt,
		string(run.RequestedMode), run.KnowledgeConnectionIDs, run.SkillInstallationID, run.SkillSnapshotID,
		string(run.EffectiveMode), string(run.Status), run.LatestArtifactID, run.ErrorMessage,
		pgTimestamptz(run.CreatedAt), pgTimestamptz(run.UpdatedAt))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) ListRuns(ctx context.Context, userID string) ([]domain.Run, error) {
	rows, err := s.pool.Query(ctx, `
SELECT id, user_id, kind, title, goal, source_url, task_session_id, task_prompt,
       requested_mode, knowledge_connection_ids, skill_installation_id, skill_snapshot_id,
       effective_mode, status, latest_artifact_id, error_message, created_at, updated_at
FROM runs
WHERE user_id = $1
ORDER BY updated_at DESC
`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	runs := make([]domain.Run, 0)
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

type runScanner interface {
	Scan(dest ...any) error
}

func scanRun(scanner runScanner) (domain.Run, error) {
	var run domain.Run
	var kind string
	var requestedMode string
	var effectiveMode string
	var status string
	var createdAt pgtype.Timestamptz
	var updatedAt pgtype.Timestamptz
	if err := scanner.Scan(
		&run.ID,
		&run.UserID,
		&kind,
		&run.Title,
		&run.Goal,
		&run.SourceURL,
		&run.TaskSessionID,
		&run.TaskPrompt,
		&requestedMode,
		&run.KnowledgeConnectionIDs,
		&run.SkillInstallationID,
		&run.SkillSnapshotID,
		&effectiveMode,
		&status,
		&run.LatestArtifactID,
		&run.ErrorMessage,
		&createdAt,
		&updatedAt,
	); err != nil {
		return domain.Run{}, err
	}
	run.Kind = domain.RunKind(kind)
	if run.Kind == "" {
		run.Kind = domain.RunKindResearch
	}
	run.RequestedMode = domain.RunMode(requestedMode)
	run.EffectiveMode = domain.RunMode(effectiveMode)
	run.Status = domain.RunStatus(status)
	run.KnowledgeConnectionIDs = append([]string(nil), run.KnowledgeConnectionIDs...)
	run.CreatedAt = pgTimestamptzValue(createdAt)
	run.UpdatedAt = pgTimestamptzValue(updatedAt)
	return run, nil
}
