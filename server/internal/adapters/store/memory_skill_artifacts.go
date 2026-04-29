package store

import (
	"context"
	"sort"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func (s *MemoryStore) CreateSkillRuntimeSnapshot(_ context.Context, snapshot domain.SkillRuntimeSnapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.skillSnapshots[snapshot.ID] = snapshot
	return nil
}

func (s *MemoryStore) CreateSkillArtifact(_ context.Context, artifact domain.SkillArtifact) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.skillArtifacts[artifact.ID] = cloneSkillArtifact(artifact)
	return nil
}

func (s *MemoryStore) GetSkillArtifact(_ context.Context, userID, artifactID string) (domain.SkillArtifact, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	artifact, ok := s.skillArtifacts[artifactID]
	if !ok || artifact.UserID != userID {
		return domain.SkillArtifact{}, ErrNotFound
	}
	return cloneSkillArtifact(artifact), nil
}

func (s *MemoryStore) ReplaceSkillArtifactFiles(_ context.Context, artifactID string, files []domain.SkillArtifactFile) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	artifact, ok := s.skillArtifacts[artifactID]
	if !ok {
		return ErrNotFound
	}
	items := make([]domain.SkillArtifactFile, 0, len(files))
	for _, file := range files {
		file.ArtifactID = artifactID
		file.UserID = artifact.UserID
		items = append(items, file)
	}
	s.skillArtifactFiles[artifactID] = items
	return nil
}

func (s *MemoryStore) ListSkillArtifactFiles(_ context.Context, userID, artifactID string) ([]domain.SkillArtifactFile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	artifact, ok := s.skillArtifacts[artifactID]
	if !ok || artifact.UserID != userID {
		return nil, ErrNotFound
	}
	items := append([]domain.SkillArtifactFile(nil), s.skillArtifactFiles[artifactID]...)
	sort.Slice(items, func(i, j int) bool {
		return items[i].Path < items[j].Path
	})
	return items, nil
}

func cloneSkillArtifact(artifact domain.SkillArtifact) domain.SkillArtifact {
	artifact.ArchiveBytes = append([]byte(nil), artifact.ArchiveBytes...)
	return artifact
}
