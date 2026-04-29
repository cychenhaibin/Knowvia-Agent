package store

import (
	"context"
	"sort"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skillruntime"
)

func (s *MemoryStore) ListSkillDefinitions(_ context.Context, userID string) ([]domain.SkillDefinition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]domain.SkillDefinition, 0, len(s.skillDefinitions))
	for _, definition := range s.skillDefinitions {
		if definition.UserID != userID {
			continue
		}
		items = append(items, definition)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	return items, nil
}

func (s *MemoryStore) GetSkillDefinitionDetails(_ context.Context, userID, definitionID string) (domain.SkillDefinitionDetails, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	definition, ok := s.skillDefinitions[definitionID]
	if !ok || definition.UserID != userID {
		return domain.SkillDefinitionDetails{}, ErrNotFound
	}

	revisions := make([]domain.SkillRevision, 0)
	for _, revision := range s.skillRevisions {
		if revision.DefinitionID != definitionID {
			continue
		}
		item := revision
		if err := skillruntime.ApplyRevisionManifestToRevision(&item, revision.ManifestJSON); err != nil {
			return domain.SkillDefinitionDetails{}, err
		}
		revisions = append(revisions, item)
	}
	sort.Slice(revisions, func(i, j int) bool {
		if revisions[i].Version == revisions[j].Version {
			return revisions[i].CreatedAt.After(revisions[j].CreatedAt)
		}
		return revisions[i].Version > revisions[j].Version
	})

	installations := make([]domain.SkillInstallation, 0)
	for _, installation := range s.skillInstallations {
		if installation.UserID != userID || installation.DefinitionID != definitionID {
			continue
		}
		installations = append(installations, installation)
	}
	sort.Slice(installations, func(i, j int) bool {
		return installations[i].UpdatedAt.After(installations[j].UpdatedAt)
	})

	return domain.SkillDefinitionDetails{
		Definition:    definition,
		Revisions:     revisions,
		Installations: installations,
	}, nil
}

func (s *MemoryStore) ListSkillRevisions(_ context.Context, userID, definitionID string) ([]domain.SkillRevision, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	definition, ok := s.skillDefinitions[definitionID]
	if !ok || definition.UserID != userID {
		return nil, ErrNotFound
	}
	items := make([]domain.SkillRevision, 0)
	for _, revision := range s.skillRevisions {
		if revision.DefinitionID != definitionID {
			continue
		}
		item := revision
		if err := skillruntime.ApplyRevisionManifestToRevision(&item, revision.ManifestJSON); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Version == items[j].Version {
			return items[i].CreatedAt.After(items[j].CreatedAt)
		}
		return items[i].Version > items[j].Version
	})
	return items, nil
}
