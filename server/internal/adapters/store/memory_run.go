package store

import (
	"context"
	"sort"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func (s *MemoryStore) CreateRun(_ context.Context, run domain.Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if run.Kind == "" {
		run.Kind = domain.RunKindResearch
	}
	s.runs[run.ID] = run
	return nil
}

func (s *MemoryStore) GetRun(_ context.Context, userID, runID string) (domain.Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	run, ok := s.runs[runID]
	if !ok || run.UserID != userID {
		return domain.Run{}, ErrNotFound
	}
	if run.Kind == "" {
		run.Kind = domain.RunKindResearch
	}
	return run, nil
}

func (s *MemoryStore) GetRunByID(_ context.Context, runID string) (domain.Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	run, ok := s.runs[runID]
	if !ok {
		return domain.Run{}, ErrNotFound
	}
	if run.Kind == "" {
		run.Kind = domain.RunKindResearch
	}
	return run, nil
}

func (s *MemoryStore) UpdateRun(_ context.Context, run domain.Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if run.Kind == "" {
		run.Kind = domain.RunKindResearch
	}
	s.runs[run.ID] = run
	return nil
}

func (s *MemoryStore) ListRuns(_ context.Context, userID string) ([]domain.Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	runs := make([]domain.Run, 0, len(s.runs))
	for _, run := range s.runs {
		if run.UserID == userID {
			if run.Kind == "" {
				run.Kind = domain.RunKindResearch
			}
			runs = append(runs, run)
		}
	}
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].UpdatedAt.After(runs[j].UpdatedAt)
	})
	return runs, nil
}

func (s *MemoryStore) UpsertRunStep(_ context.Context, step domain.RunStep) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	steps := s.runSteps[step.RunID]
	for i := range steps {
		if steps[i].ID == step.ID {
			steps[i] = step
			s.runSteps[step.RunID] = steps
			return nil
		}
	}
	s.runSteps[step.RunID] = append(steps, step)
	return nil
}

func (s *MemoryStore) ListRunSteps(_ context.Context, runID string) ([]domain.RunStep, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	steps := append([]domain.RunStep(nil), s.runSteps[runID]...)
	sort.Slice(steps, func(i, j int) bool {
		return steps[i].CreatedAt.Before(steps[j].CreatedAt)
	})
	return steps, nil
}

func (s *MemoryStore) SaveArtifact(_ context.Context, artifact domain.RunArtifact) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.artifacts[artifact.RunID] = append(s.artifacts[artifact.RunID], artifact)
	return nil
}

func (s *MemoryStore) ListArtifacts(_ context.Context, runID string) ([]domain.RunArtifact, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	artifacts := append([]domain.RunArtifact(nil), s.artifacts[runID]...)
	sort.Slice(artifacts, func(i, j int) bool {
		return artifacts[i].CreatedAt.Before(artifacts[j].CreatedAt)
	})
	return artifacts, nil
}

func (s *MemoryStore) SaveSources(_ context.Context, runID string, sources []domain.RunSource) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runSources[runID] = append([]domain.RunSource(nil), sources...)
	return nil
}

func (s *MemoryStore) ListSources(_ context.Context, runID string) ([]domain.RunSource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sources := append([]domain.RunSource(nil), s.runSources[runID]...)
	sort.Slice(sources, func(i, j int) bool {
		return sources[i].CreatedAt.Before(sources[j].CreatedAt)
	})
	return sources, nil
}
