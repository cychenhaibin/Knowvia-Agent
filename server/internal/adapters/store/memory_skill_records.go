package store

import (
	"context"
	"sort"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func (s *MemoryStore) GetSkill(_ context.Context, userID, skillID string) (domain.Skill, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	skill, err := s.materializeSkill(skillID)
	if err != nil || skill.UserID != userID {
		return domain.Skill{}, ErrNotFound
	}
	return skill, nil
}

func (s *MemoryStore) ListSkills(_ context.Context, userID string) ([]domain.Skill, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	skills := []domain.Skill{}
	for installationID, installation := range s.skillInstallations {
		if installation.UserID != userID {
			continue
		}
		skill, err := s.materializeSkill(installationID)
		if err == nil {
			skills = append(skills, skill)
		}
	}
	sort.Slice(skills, func(i, j int) bool {
		return skills[i].UpdatedAt.After(skills[j].UpdatedAt)
	})
	return skills, nil
}

func (s *MemoryStore) GetSkillInstallationRecord(_ context.Context, userID, installationID string) (domain.SkillInstallationRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, err := s.materializeSkillInstallationRecord(installationID)
	if err != nil || record.Installation.UserID != userID {
		return domain.SkillInstallationRecord{}, ErrNotFound
	}
	return record, nil
}

func (s *MemoryStore) GetDefaultSkillInstallationRecord(_ context.Context, userID, definitionID string) (domain.SkillInstallationRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var selectedID string
	var selected domain.SkillInstallation
	for installationID, installation := range s.skillInstallations {
		if installation.UserID != userID || installation.DefinitionID != definitionID {
			continue
		}
		if selectedID == "" || installation.IsDefault {
			selectedID = installationID
			selected = installation
			if installation.IsDefault {
				break
			}
		}
	}
	if selectedID == "" {
		return domain.SkillInstallationRecord{}, ErrNotFound
	}
	record, err := s.materializeSkillInstallationRecord(selected.ID)
	if err != nil {
		return domain.SkillInstallationRecord{}, err
	}
	return record, nil
}

func (s *MemoryStore) ListSkillInstallations(_ context.Context, userID string) ([]domain.SkillInstallationRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]domain.SkillInstallationRecord, 0, len(s.skillInstallations))
	for installationID, installation := range s.skillInstallations {
		if installation.UserID != userID {
			continue
		}
		record, err := s.materializeSkillInstallationRecord(installationID)
		if err == nil {
			items = append(items, record)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Installation.UpdatedAt.After(items[j].Installation.UpdatedAt)
	})
	return items, nil
}
