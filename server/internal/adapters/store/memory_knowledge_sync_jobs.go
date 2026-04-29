package store

import (
	"context"
	"sort"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func (s *MemoryStore) CreateKnowledgeSyncJob(_ context.Context, job domain.KnowledgeSyncJob) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.syncJobs[job.ID] = job
	return nil
}

func (s *MemoryStore) UpdateKnowledgeSyncJob(_ context.Context, job domain.KnowledgeSyncJob) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.syncJobs[job.ID] = job
	return nil
}

func (s *MemoryStore) GetKnowledgeSyncJob(_ context.Context, jobID string) (domain.KnowledgeSyncJob, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.syncJobs[jobID]
	if !ok {
		return domain.KnowledgeSyncJob{}, ErrNotFound
	}
	return job, nil
}

func (s *MemoryStore) ListKnowledgeSyncJobs(_ context.Context, userID, connectionID string) ([]domain.KnowledgeSyncJob, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	jobs := []domain.KnowledgeSyncJob{}
	for _, job := range s.syncJobs {
		if job.UserID == userID && job.ConnectionID == connectionID {
			jobs = append(jobs, job)
		}
	}
	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].CreatedAt.After(jobs[j].CreatedAt)
	})
	return jobs, nil
}
