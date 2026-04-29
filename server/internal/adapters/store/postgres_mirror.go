package store

import (
	"context"
	"time"

	sqldb "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/db"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func (s *PostgresStore) CreateMirrorTask(ctx context.Context, task domain.MirrorTask) error {
	_, err := s.queries.CreateMirrorTask(ctx, sqldb.CreateMirrorTaskParams{
		ID:          task.ID,
		Kind:        string(task.Kind),
		UserID:      task.UserID,
		ResourceID:  task.ResourceID,
		Status:      string(task.Status),
		Payload:     task.Payload,
		Attempts:    int32(task.Attempts),
		LastError:   task.LastError,
		NextRetryAt: pgTimestamptz(task.NextRetryAt),
		CreatedAt:   pgTimestamptz(task.CreatedAt),
		UpdatedAt:   pgTimestamptz(task.UpdatedAt),
		CompletedAt: pgNullableTimestamptz(task.CompletedAt),
	})
	return err
}

func (s *PostgresStore) UpdateMirrorTask(ctx context.Context, task domain.MirrorTask) error {
	rowsAffected, err := s.queries.UpdateMirrorTask(ctx, sqldb.UpdateMirrorTaskParams{
		ID:          task.ID,
		Kind:        string(task.Kind),
		UserID:      task.UserID,
		ResourceID:  task.ResourceID,
		Status:      string(task.Status),
		Payload:     task.Payload,
		Attempts:    int32(task.Attempts),
		LastError:   task.LastError,
		NextRetryAt: pgTimestamptz(task.NextRetryAt),
		CreatedAt:   pgTimestamptz(task.CreatedAt),
		UpdatedAt:   pgTimestamptz(task.UpdatedAt),
		CompletedAt: pgNullableTimestamptz(task.CompletedAt),
	})
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) GetMirrorTask(ctx context.Context, taskID string) (domain.MirrorTask, error) {
	task, err := s.queries.GetMirrorTask(ctx, taskID)
	if err != nil {
		return domain.MirrorTask{}, normalizeError(err)
	}
	return mapDBMirrorTask(task), nil
}

func (s *PostgresStore) ListDueMirrorTasks(ctx context.Context, dueBefore time.Time, limit int) ([]domain.MirrorTask, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.queries.ListDueMirrorTasks(ctx, sqldb.ListDueMirrorTasksParams{
		Status:      string(domain.MirrorTaskPending),
		NextRetryAt: pgTimestamptz(dueBefore),
		Limit:       int32(limit),
	})
	if err != nil {
		return nil, err
	}

	tasks := make([]domain.MirrorTask, 0, len(rows))
	for _, task := range rows {
		tasks = append(tasks, mapDBMirrorTask(task))
	}
	return tasks, nil
}

func mapDBMirrorTask(task sqldb.MirrorTask) domain.MirrorTask {
	return domain.MirrorTask{
		ID:          task.ID,
		Kind:        domain.MirrorTaskKind(task.Kind),
		UserID:      task.UserID,
		ResourceID:  task.ResourceID,
		Status:      domain.MirrorTaskStatus(task.Status),
		Payload:     task.Payload,
		Attempts:    int(task.Attempts),
		LastError:   task.LastError,
		NextRetryAt: pgTimestamptzValue(task.NextRetryAt),
		CreatedAt:   pgTimestamptzValue(task.CreatedAt),
		UpdatedAt:   pgTimestamptzValue(task.UpdatedAt),
		CompletedAt: pgNullableTimestamptzPtr(task.CompletedAt),
	}
}
