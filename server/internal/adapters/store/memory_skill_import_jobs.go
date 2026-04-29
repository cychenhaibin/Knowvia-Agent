package store

import (
	"context"
	"sort"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func (s *MemoryStore) CreateSkillImportJob(_ context.Context, job domain.SkillImportJob) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.skillImportJobs[job.ID] = job
	return nil
}

func (s *MemoryStore) UpdateSkillImportJob(_ context.Context, job domain.SkillImportJob) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.skillImportJobs[job.ID]; !ok {
		return ErrNotFound
	}
	s.skillImportJobs[job.ID] = job
	return nil
}

func (s *MemoryStore) GetSkillImportJob(_ context.Context, userID, jobID string) (domain.SkillImportJob, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.skillImportJobs[jobID]
	if !ok || job.UserID != userID {
		return domain.SkillImportJob{}, ErrNotFound
	}
	return job, nil
}

func (s *MemoryStore) ListSkillImportJobs(_ context.Context, userID string) ([]domain.SkillImportJob, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]domain.SkillImportJob, 0, len(s.skillImportJobs))
	for _, job := range s.skillImportJobs {
		if job.UserID == userID {
			items = append(items, job)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	return items, nil
}
