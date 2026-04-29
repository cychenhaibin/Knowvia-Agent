package mirror

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/provider"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

type mirrorStoreStub struct {
	tasks map[string]domain.MirrorTask
}

func newMirrorStoreStub(tasks ...domain.MirrorTask) *mirrorStoreStub {
	items := make(map[string]domain.MirrorTask, len(tasks))
	for _, task := range tasks {
		items[task.ID] = task
	}
	return &mirrorStoreStub{tasks: items}
}

func (s *mirrorStoreStub) ListDueMirrorTasks(_ context.Context, cutoff time.Time, limit int) ([]domain.MirrorTask, error) {
	items := make([]domain.MirrorTask, 0, len(s.tasks))
	for _, task := range s.tasks {
		if !task.NextRetryAt.After(cutoff) {
			items = append(items, task)
		}
	}
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (s *mirrorStoreStub) GetMirrorTask(_ context.Context, id string) (domain.MirrorTask, error) {
	task, ok := s.tasks[id]
	if !ok {
		return domain.MirrorTask{}, errors.New("missing task")
	}
	return task, nil
}

func (s *mirrorStoreStub) UpdateMirrorTask(_ context.Context, task domain.MirrorTask) error {
	s.tasks[task.ID] = task
	return nil
}

func (s *mirrorStoreStub) CreateMirrorTask(_ context.Context, task domain.MirrorTask) error {
	s.tasks[task.ID] = task
	return nil
}

type mirrorClientStub struct {
	upsertSkillErr error
	upsertCalls    int
}

func (c *mirrorClientStub) UpsertSkill(_ context.Context, _ domain.Skill) error {
	c.upsertCalls++
	return c.upsertSkillErr
}

func (c *mirrorClientStub) DeleteSkill(context.Context, string, string) error { return nil }

func (c *mirrorClientStub) UpsertKnowledge(context.Context, provider.MirroredKnowledgeUpsertRequest) (provider.MirroredKnowledgeUpsertResult, error) {
	return provider.MirroredKnowledgeUpsertResult{}, nil
}

func (c *mirrorClientStub) DeleteKnowledge(context.Context, string, string) error { return nil }

func TestProcessTaskMarksCompletedOnSuccess(t *testing.T) {
	now := time.Now().UTC()
	store := newMirrorStoreStub(domain.MirrorTask{
		ID:         "task-1",
		Kind:       domain.MirrorTaskSkillUpsert,
		UserID:     "user-1",
		ResourceID: "skill-1",
		Status:     domain.MirrorTaskPending,
		Payload:    `{"id":"skill-1","user_id":"user-1","slug":"slug","title":"Skill","description":"desc","prompt":"prompt","mode":"chat","source":"manual","enabled":true,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`,
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	client := &mirrorClientStub{}
	service := NewService(store, client)

	if err := service.processTask(context.Background(), "task-1"); err != nil {
		t.Fatalf("process task: %v", err)
	}

	task := store.tasks["task-1"]
	if task.Status != domain.MirrorTaskCompleted {
		t.Fatalf("expected completed task, got %s", task.Status)
	}
	if task.CompletedAt == nil {
		t.Fatalf("expected completed timestamp to be set")
	}
	if task.LastError != "" {
		t.Fatalf("expected cleared last error, got %q", task.LastError)
	}
	if client.upsertCalls != 1 {
		t.Fatalf("expected one upsert call, got %d", client.upsertCalls)
	}
}

func TestProcessTaskSchedulesRetryOnFailure(t *testing.T) {
	now := time.Now().UTC()
	store := newMirrorStoreStub(domain.MirrorTask{
		ID:         "task-1",
		Kind:       domain.MirrorTaskSkillUpsert,
		UserID:     "user-1",
		ResourceID: "skill-1",
		Status:     domain.MirrorTaskPending,
		Payload:    `{"id":"skill-1","user_id":"user-1","slug":"slug","title":"Skill","description":"desc","prompt":"prompt","mode":"chat","source":"manual","enabled":true,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`,
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	client := &mirrorClientStub{upsertSkillErr: errors.New("proxy unavailable")}
	service := NewService(store, client)

	if err := service.processTask(context.Background(), "task-1"); err != nil {
		t.Fatalf("process task: %v", err)
	}

	task := store.tasks["task-1"]
	if task.Status != domain.MirrorTaskPending {
		t.Fatalf("expected pending retry task, got %s", task.Status)
	}
	if task.Attempts != 1 {
		t.Fatalf("expected attempts to increment to 1, got %d", task.Attempts)
	}
	if task.LastError != "proxy unavailable" {
		t.Fatalf("expected last error to be stored, got %q", task.LastError)
	}
	if !task.NextRetryAt.After(now) {
		t.Fatalf("expected next retry to move into future, got %s", task.NextRetryAt)
	}
}
