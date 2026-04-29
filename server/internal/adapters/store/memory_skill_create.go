package store

import (
	"context"
	"strings"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skillruntime"
)

func (s *MemoryStore) CreateSkill(_ context.Context, skill domain.Skill) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	manifestJSON, err := skillruntime.EncodeRevisionManifest(skill)
	if err != nil {
		return err
	}
	s.skillDefinitions[skill.DefinitionID] = domain.SkillDefinition{
		ID:        skill.DefinitionID,
		UserID:    skill.UserID,
		Slug:      skill.Slug,
		Kind:      skill.Kind,
		Source:    skill.Source,
		RepoURL:   skill.RepoURL,
		CreatedAt: skill.CreatedAt,
		UpdatedAt: skill.UpdatedAt,
	}
	s.skillRevisions[skill.RevisionID] = domain.SkillRevision{
		ID:            skill.RevisionID,
		DefinitionID:  skill.DefinitionID,
		Version:       skill.Version,
		Title:         skill.Title,
		Description:   skill.Description,
		Prompt:        skill.Prompt,
		Mode:          skill.Mode,
		PlannerPolicy: cloneStringMap(skill.PlannerPolicy),
		ToolAllowlist: append([]string(nil), skill.ToolAllowlist...),
		ManifestJSON:  manifestJSON,
		CreatedAt:     skill.UpdatedAt,
	}
	s.skillInstallations[skill.ID] = domain.SkillInstallation{
		ID:                skill.ID,
		UserID:            skill.UserID,
		DefinitionID:      skill.DefinitionID,
		CurrentRevisionID: skill.RevisionID,
		Name:              skill.Title,
		IsDefault:         true,
		Enabled:           skill.Enabled,
		CreatedAt:         skill.CreatedAt,
		UpdatedAt:         skill.UpdatedAt,
	}
	return nil
}

func (s *MemoryStore) CreateSkillDefinitionRevision(_ context.Context, definition domain.SkillDefinition, revision domain.SkillRevision) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.skillDefinitions {
		if existing.UserID == definition.UserID && existing.Slug == definition.Slug {
			return ErrConflict
		}
	}
	if strings.TrimSpace(revision.ManifestJSON) == "" {
		revision.ManifestJSON = "{}"
	}
	s.skillDefinitions[definition.ID] = definition
	s.skillRevisions[revision.ID] = domain.SkillRevision{
		ID:            revision.ID,
		DefinitionID:  revision.DefinitionID,
		Version:       revision.Version,
		Title:         revision.Title,
		Description:   revision.Description,
		Prompt:        revision.Prompt,
		Mode:          revision.Mode,
		PlannerPolicy: cloneStringMap(revision.PlannerPolicy),
		ToolAllowlist: append([]string(nil), revision.ToolAllowlist...),
		ManifestJSON:  revision.ManifestJSON,
		CreatedAt:     revision.CreatedAt,
	}
	return nil
}

func (s *MemoryStore) CreateSkillInstallation(_ context.Context, installation domain.SkillInstallation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	definition, ok := s.skillDefinitions[installation.DefinitionID]
	if !ok || definition.UserID != installation.UserID {
		return ErrNotFound
	}
	revision, ok := s.skillRevisions[installation.CurrentRevisionID]
	if !ok || revision.DefinitionID != installation.DefinitionID {
		return ErrNotFound
	}
	if strings.TrimSpace(installation.Name) == "" {
		installation.Name = revision.Title
	}
	if !installation.IsDefault && !s.definitionHasInstallationsForUser(installation.UserID, installation.DefinitionID) {
		installation.IsDefault = true
	}
	if installation.IsDefault {
		s.clearDefaultSkillInstallations(installation.UserID, installation.DefinitionID, installation.ID)
	}
	s.skillInstallations[installation.ID] = installation
	return nil
}
