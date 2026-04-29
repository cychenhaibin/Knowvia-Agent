package store

import (
	"context"
	"strings"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skillruntime"
)

func (s *MemoryStore) UpdateSkill(_ context.Context, skill domain.Skill) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	installation, ok := s.skillInstallations[skill.ID]
	if !ok {
		return ErrNotFound
	}
	definition, ok := s.skillDefinitions[installation.DefinitionID]
	if !ok {
		return ErrNotFound
	}
	definition.Slug = skill.Slug
	definition.Kind = skill.Kind
	definition.Source = skill.Source
	definition.RepoURL = skill.RepoURL
	definition.UpdatedAt = skill.UpdatedAt
	s.skillDefinitions[definition.ID] = definition
	manifestJSON, err := skillruntime.EncodeRevisionManifest(skill)
	if err != nil {
		return err
	}
	s.skillRevisions[skill.RevisionID] = domain.SkillRevision{
		ID:            skill.RevisionID,
		DefinitionID:  definition.ID,
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
	installation.CurrentRevisionID = skill.RevisionID
	installation.Enabled = skill.Enabled
	installation.UpdatedAt = skill.UpdatedAt
	s.skillInstallations[installation.ID] = installation
	return nil
}

func (s *MemoryStore) UpdateSkillInstallation(_ context.Context, installation domain.SkillInstallation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.skillInstallations[installation.ID]
	if !ok || current.UserID != installation.UserID {
		return ErrNotFound
	}
	revision, ok := s.skillRevisions[installation.CurrentRevisionID]
	if !ok || revision.DefinitionID != current.DefinitionID {
		return ErrNotFound
	}
	if strings.TrimSpace(installation.Name) == "" {
		installation.Name = revision.Title
	}
	if installation.IsDefault {
		s.clearDefaultSkillInstallations(installation.UserID, installation.DefinitionID, installation.ID)
	}
	if current.IsDefault && !installation.IsDefault {
		if s.hasSiblingSkillInstallation(current.UserID, current.DefinitionID, current.ID) {
			s.promoteDefaultSkillInstallation(current.UserID, current.DefinitionID, current.ID)
		} else {
			installation.IsDefault = true
		}
	}
	current.CurrentRevisionID = installation.CurrentRevisionID
	current.Name = installation.Name
	current.IsDefault = installation.IsDefault
	current.Enabled = installation.Enabled
	current.UpdatedAt = installation.UpdatedAt
	s.skillInstallations[current.ID] = current
	return nil
}
