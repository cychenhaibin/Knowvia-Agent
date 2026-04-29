package store

import (
	"context"

	sqldb "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/db"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func (s *PostgresStore) UpsertRunStep(ctx context.Context, step domain.RunStep) error {
	_, err := s.queries.UpsertRunStep(ctx, sqldb.UpsertRunStepParams{
		ID:         step.ID,
		RunID:      step.RunID,
		Kind:       string(step.Kind),
		Label:      step.Label,
		Status:     string(step.Status),
		Summary:    step.Summary,
		StartedAt:  pgNullableTimestamptz(step.StartedAt),
		FinishedAt: pgNullableTimestamptz(step.FinishedAt),
		CreatedAt:  pgTimestamptz(step.CreatedAt),
	})
	return err
}

func (s *PostgresStore) ListRunSteps(ctx context.Context, runID string) ([]domain.RunStep, error) {
	rows, err := s.queries.ListRunSteps(ctx, runID)
	if err != nil {
		return nil, err
	}
	steps := make([]domain.RunStep, 0, len(rows))
	for _, row := range rows {
		steps = append(steps, domain.RunStep{
			ID:         row.ID,
			RunID:      row.RunID,
			Kind:       domain.StepKind(row.Kind),
			Label:      row.Label,
			Status:     domain.StepStatus(row.Status),
			Summary:    row.Summary,
			StartedAt:  pgNullableTimestamptzPtr(row.StartedAt),
			FinishedAt: pgNullableTimestamptzPtr(row.FinishedAt),
			CreatedAt:  pgTimestamptzValue(row.CreatedAt),
		})
	}
	return steps, nil
}
