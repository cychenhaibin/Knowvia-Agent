package store

import (
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skillruntime"
)

func (s *MemoryStore) materializeSkill(installationID string) (domain.Skill, error) {
	installation, ok := s.skillInstallations[installationID]
	if !ok {
		return domain.Skill{}, ErrNotFound
	}
	definition, ok := s.skillDefinitions[installation.DefinitionID]
	if !ok {
		return domain.Skill{}, ErrNotFound
	}
	revision, ok := s.skillRevisions[installation.CurrentRevisionID]
	if !ok {
		return domain.Skill{}, ErrNotFound
	}
	skill := domain.Skill{
		ID:           installation.ID,
		UserID:       installation.UserID,
		DefinitionID: definition.ID,
		RevisionID:   revision.ID,
		Version:      revision.Version,
		Slug:         definition.Slug,
		Kind:         definition.Kind,
		Title:        revision.Title,
		Description:  revision.Description,
		Prompt:       revision.Prompt,
		Mode:         revision.Mode,
		Source:       definition.Source,
		Enabled:      installation.Enabled,
		RepoURL:      definition.RepoURL,
		CreatedAt:    installation.CreatedAt,
		UpdatedAt:    installation.UpdatedAt,
	}
	if err := skillruntime.ApplyRevisionManifest(&skill, revision.ManifestJSON); err != nil {
		return domain.Skill{}, err
	}
	return skill, nil
}

func (s *MemoryStore) materializeSkillInstallationRecord(installationID string) (domain.SkillInstallationRecord, error) {
	installation, ok := s.skillInstallations[installationID]
	if !ok {
		return domain.SkillInstallationRecord{}, ErrNotFound
	}
	definition, ok := s.skillDefinitions[installation.DefinitionID]
	if !ok {
		return domain.SkillInstallationRecord{}, ErrNotFound
	}
	revision, ok := s.skillRevisions[installation.CurrentRevisionID]
	if !ok {
		return domain.SkillInstallationRecord{}, ErrNotFound
	}
	currentRevision := revision
	if err := skillruntime.ApplyRevisionManifestToRevision(&currentRevision, revision.ManifestJSON); err != nil {
		return domain.SkillInstallationRecord{}, err
	}
	return domain.SkillInstallationRecord{
		Installation:    installation,
		Definition:      definition,
		CurrentRevision: currentRevision,
	}, nil
}

func (s *MemoryStore) definitionHasInstallationsForUser(userID, definitionID string) bool {
	for _, installation := range s.skillInstallations {
		if installation.UserID == userID && installation.DefinitionID == definitionID {
			return true
		}
	}
	return false
}

func (s *MemoryStore) clearDefaultSkillInstallations(userID, definitionID, exceptID string) {
	for id, installation := range s.skillInstallations {
		if installation.UserID != userID || installation.DefinitionID != definitionID || id == exceptID {
			continue
		}
		installation.IsDefault = false
		s.skillInstallations[id] = installation
	}
}

func (s *MemoryStore) promoteDefaultSkillInstallation(userID, definitionID, excludedID string) {
	var selectedID string
	var selected domain.SkillInstallation
	for id, installation := range s.skillInstallations {
		if installation.UserID != userID || installation.DefinitionID != definitionID || id == excludedID {
			continue
		}
		if selectedID == "" || installation.UpdatedAt.After(selected.UpdatedAt) {
			selectedID = id
			selected = installation
		}
	}
	if selectedID == "" {
		return
	}
	selected.IsDefault = true
	s.skillInstallations[selectedID] = selected
}

func (s *MemoryStore) hasSiblingSkillInstallation(userID, definitionID, excludedID string) bool {
	for id, installation := range s.skillInstallations {
		if id == excludedID {
			continue
		}
		if installation.UserID == userID && installation.DefinitionID == definitionID {
			return true
		}
	}
	return false
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}
