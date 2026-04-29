package mirror

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/provider"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/persistence"
)

type Service struct {
	store  MirrorStore
	client provider.MirrorClient
}

type MirrorStore interface {
	ListDueMirrorTasks(context.Context, time.Time, int) ([]domain.MirrorTask, error)
	GetMirrorTask(context.Context, string) (domain.MirrorTask, error)
	UpdateMirrorTask(context.Context, domain.MirrorTask) error
	CreateMirrorTask(context.Context, domain.MirrorTask) error
}

func NewService(st MirrorStore, client provider.MirrorClient) *Service {
	return &Service{store: st, client: client}
}

func (s *Service) StartBackgroundRetry(ctx context.Context) {
	if s == nil || s.client == nil {
		return
	}
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = s.ProcessDueTasks(context.Background(), 20)
		}
	}
}

func (s *Service) SyncSkill(ctx context.Context, skill domain.Skill) error {
	if s == nil || s.client == nil {
		return nil
	}
	if err := s.client.UpsertSkill(ctx, skill); err != nil {
		return s.recordAndSchedule(ctx, domain.MirrorTaskSkillUpsert, skill.UserID, skill.ID, skill, err)
	}
	return nil
}

func (s *Service) DeleteSkill(ctx context.Context, userID, skillID string) error {
	if s == nil || s.client == nil {
		return nil
	}
	payload := map[string]string{
		"user_id":  userID,
		"skill_id": skillID,
	}
	if err := s.client.DeleteSkill(ctx, userID, skillID); err != nil {
		return s.recordAndSchedule(ctx, domain.MirrorTaskSkillDelete, userID, skillID, payload, err)
	}
	return nil
}

func (s *Service) SyncKnowledge(
	ctx context.Context,
	req provider.MirroredKnowledgeUpsertRequest,
) (provider.MirroredKnowledgeUpsertResult, error) {
	if s == nil || s.client == nil {
		return provider.MirroredKnowledgeUpsertResult{}, nil
	}
	result, err := s.client.UpsertKnowledge(ctx, req)
	if err != nil {
		return provider.MirroredKnowledgeUpsertResult{}, s.recordAndSchedule(
			ctx,
			domain.MirrorTaskKnowledgeUpsert,
			req.UserID,
			req.ConnectionID,
			req,
			err,
		)
	}
	return result, nil
}

func (s *Service) DeleteKnowledge(ctx context.Context, userID, connectionID string) error {
	if s == nil || s.client == nil {
		return nil
	}
	payload := map[string]string{
		"user_id":       userID,
		"connection_id": connectionID,
	}
	if err := s.client.DeleteKnowledge(ctx, userID, connectionID); err != nil {
		return s.recordAndSchedule(ctx, domain.MirrorTaskKnowledgeDelete, userID, connectionID, payload, err)
	}
	return nil
}

func (s *Service) ProcessDueTasks(ctx context.Context, limit int) error {
	if s == nil || s.client == nil {
		return nil
	}
	tasks, err := s.store.ListDueMirrorTasks(ctx, time.Now().UTC(), limit)
	if err != nil {
		return err
	}
	for _, task := range tasks {
		if err := s.processTask(ctx, task.ID); err != nil {
			continue
		}
	}
	return nil
}

func (s *Service) processTask(ctx context.Context, taskID string) error {
	task, err := s.store.GetMirrorTask(ctx, taskID)
	if err != nil {
		return err
	}
	if task.Status == domain.MirrorTaskCompleted {
		return nil
	}

	task.Status = domain.MirrorTaskRunning
	task.UpdatedAt = time.Now().UTC()
	if err := s.store.UpdateMirrorTask(ctx, task); err != nil {
		return err
	}

	callErr := s.executeTask(ctx, task)
	if callErr == nil {
		finishedAt := time.Now().UTC()
		task.Status = domain.MirrorTaskCompleted
		task.LastError = ""
		task.CompletedAt = &finishedAt
		task.UpdatedAt = finishedAt
		return s.store.UpdateMirrorTask(ctx, task)
	}

	task.Status = domain.MirrorTaskPending
	task.Attempts++
	task.LastError = callErr.Error()
	task.NextRetryAt = nextRetryAt(task.Attempts)
	task.UpdatedAt = time.Now().UTC()
	return s.store.UpdateMirrorTask(ctx, task)
}

func (s *Service) executeTask(ctx context.Context, task domain.MirrorTask) error {
	switch task.Kind {
	case domain.MirrorTaskSkillUpsert:
		var skill domain.Skill
		if err := json.Unmarshal([]byte(task.Payload), &skill); err != nil {
			return err
		}
		return s.client.UpsertSkill(ctx, skill)
	case domain.MirrorTaskSkillDelete:
		var payload struct {
			UserID  string `json:"user_id"`
			SkillID string `json:"skill_id"`
		}
		if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
			return err
		}
		return s.client.DeleteSkill(ctx, payload.UserID, payload.SkillID)
	case domain.MirrorTaskKnowledgeUpsert:
		var req provider.MirroredKnowledgeUpsertRequest
		if err := json.Unmarshal([]byte(task.Payload), &req); err != nil {
			return err
		}
		_, err := s.client.UpsertKnowledge(ctx, req)
		return err
	case domain.MirrorTaskKnowledgeDelete:
		var payload struct {
			UserID       string `json:"user_id"`
			ConnectionID string `json:"connection_id"`
		}
		if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
			return err
		}
		return s.client.DeleteKnowledge(ctx, payload.UserID, payload.ConnectionID)
	default:
		return fmt.Errorf("unsupported mirror task kind: %s", task.Kind)
	}
}

func (s *Service) recordAndSchedule(
	ctx context.Context,
	kind domain.MirrorTaskKind,
	userID string,
	resourceID string,
	payload any,
	cause error,
) error {
	if errors.Is(cause, persistence.ErrNotFound) {
		return cause
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("%v; failed to encode retry payload: %w", cause, err)
	}

	now := time.Now().UTC()
	task := domain.MirrorTask{
		ID:          uuid.NewString(),
		Kind:        kind,
		UserID:      userID,
		ResourceID:  resourceID,
		Status:      domain.MirrorTaskPending,
		Payload:     string(raw),
		Attempts:    1,
		LastError:   cause.Error(),
		NextRetryAt: nextRetryAt(1),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.store.CreateMirrorTask(ctx, task); err != nil {
		return fmt.Errorf("%v; failed to record retry task: %w", cause, err)
	}

	go func(id string) {
		retryCtx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		_ = s.processTask(retryCtx, id)
	}(task.ID)

	return fmt.Errorf("%w; mirror retry queued", cause)
}

func nextRetryAt(attempt int) time.Time {
	if attempt < 1 {
		attempt = 1
	}
	delay := time.Duration(1<<min(attempt, 6)) * time.Second
	return time.Now().UTC().Add(delay)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
