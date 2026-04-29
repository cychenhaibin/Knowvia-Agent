package store

import (
	"context"
	"sort"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func (s *MemoryStore) CreateMirrorTask(_ context.Context, task domain.MirrorTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mirrorTasks[task.ID] = task
	return nil
}

func (s *MemoryStore) UpdateMirrorTask(_ context.Context, task domain.MirrorTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.mirrorTasks[task.ID]; !ok {
		return ErrNotFound
	}
	s.mirrorTasks[task.ID] = task
	return nil
}

func (s *MemoryStore) GetMirrorTask(_ context.Context, taskID string) (domain.MirrorTask, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	task, ok := s.mirrorTasks[taskID]
	if !ok {
		return domain.MirrorTask{}, ErrNotFound
	}
	return task, nil
}

func (s *MemoryStore) ListDueMirrorTasks(_ context.Context, dueBefore time.Time, limit int) ([]domain.MirrorTask, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tasks := []domain.MirrorTask{}
	for _, task := range s.mirrorTasks {
		if task.Status != domain.MirrorTaskPending {
			continue
		}
		if task.NextRetryAt.After(dueBefore) {
			continue
		}
		tasks = append(tasks, task)
	}
	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].NextRetryAt.Equal(tasks[j].NextRetryAt) {
			return tasks[i].CreatedAt.Before(tasks[j].CreatedAt)
		}
		return tasks[i].NextRetryAt.Before(tasks[j].NextRetryAt)
	})
	if limit > 0 && len(tasks) > limit {
		tasks = tasks[:limit]
	}
	return tasks, nil
}
