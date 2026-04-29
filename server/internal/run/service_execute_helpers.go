package run

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func (s *Service) startStep(ctx context.Context, run *domain.Run, step *domain.RunStep) error {
	now := time.Now().UTC()
	run.Status = domain.RunStatusRunning
	run.UpdatedAt = now
	if err := s.runStore.UpdateRun(ctx, *run); err != nil {
		return err
	}
	step.Status = domain.StepStatusRunning
	step.StartedAt = &now
	if err := s.stepStore.UpsertRunStep(ctx, *step); err != nil {
		return err
	}
	s.broker.Publish(run.ID, "step.started", map[string]string{
		"stepId": step.ID,
		"label":  step.Label,
	})
	return nil
}

func (s *Service) completeStep(ctx context.Context, run *domain.Run, step *domain.RunStep) error {
	now := time.Now().UTC()
	step.Status = domain.StepStatusCompleted
	step.FinishedAt = &now
	if err := s.stepStore.UpsertRunStep(ctx, *step); err != nil {
		return err
	}
	run.UpdatedAt = now
	if err := s.runStore.UpdateRun(ctx, *run); err != nil {
		return err
	}
	s.broker.Publish(run.ID, "step.completed", map[string]string{
		"stepId":  step.ID,
		"label":   step.Label,
		"summary": step.Summary,
	})
	return nil
}

func (s *Service) failRun(ctx context.Context, run domain.Run, step domain.RunStep, err error) error {
	now := time.Now().UTC()
	step.Status = domain.StepStatusFailed
	step.FinishedAt = &now
	step.Summary = err.Error()
	_ = s.stepStore.UpsertRunStep(ctx, step)
	run.Status = domain.RunStatusFailed
	run.ErrorMessage = err.Error()
	run.UpdatedAt = now
	_ = s.runStore.UpdateRun(ctx, run)
	s.broker.Publish(run.ID, "run.failed", map[string]string{
		"stepId": step.ID,
		"error":  err.Error(),
	})
	return err
}

func skillSnapshotID(snapshot *domain.SkillRuntimeSnapshot) string {
	if snapshot == nil {
		return ""
	}
	return snapshot.ID
}

func evidenceToSources(runID string, evidences []domain.Evidence) []domain.RunSource {
	sources := make([]domain.RunSource, 0, len(evidences))
	for _, evidence := range evidences {
		sources = append(sources, domain.RunSource{
			ID:           uuid.NewString(),
			RunID:        runID,
			Provider:     evidence.Provider,
			ConnectionID: evidence.ConnectionID,
			DocumentID:   evidence.DocumentID,
			ChunkID:      evidence.ChunkID,
			Title:        evidence.Title,
			Repo:         evidence.Repo,
			URL:          evidence.URL,
			Snippet:      evidence.Snippet,
			Score:        evidence.Score,
			CreatedAt:    time.Now().UTC(),
		})
	}
	return sources
}
