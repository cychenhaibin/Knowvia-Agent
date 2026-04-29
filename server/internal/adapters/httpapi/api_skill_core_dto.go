package httpapi

import (
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

type skillListDTO struct {
	Items []skillDTO `json:"items"`
}

type skillDefinitionListDTO struct {
	Items []skillDefinitionDTO `json:"items"`
}

type skillRevisionListDTO struct {
	Items []skillRevisionDTO `json:"items"`
}

type skillDTO struct {
	ID            string             `json:"id"`
	UserID        string             `json:"userId"`
	DefinitionID  string             `json:"definitionId,omitempty"`
	RevisionID    string             `json:"revisionId,omitempty"`
	Version       int                `json:"version,omitempty"`
	Slug          string             `json:"slug"`
	Kind          domain.SkillKind   `json:"kind,omitempty"`
	Title         string             `json:"title"`
	Description   string             `json:"description"`
	Prompt        string             `json:"prompt"`
	Mode          string             `json:"mode"`
	PlannerPolicy map[string]string  `json:"plannerPolicy,omitempty"`
	ToolAllowlist []string           `json:"toolAllowlist,omitempty"`
	Source        domain.SkillSource `json:"source"`
	Enabled       bool               `json:"enabled"`
	RepoURL       string             `json:"repoUrl,omitempty"`
	CreatedAt     time.Time          `json:"createdAt"`
	UpdatedAt     time.Time          `json:"updatedAt"`
}

type skillDefinitionDTO struct {
	ID        string             `json:"id"`
	UserID    string             `json:"userId"`
	Slug      string             `json:"slug"`
	Kind      domain.SkillKind   `json:"kind"`
	Source    domain.SkillSource `json:"source"`
	RepoURL   string             `json:"repoUrl,omitempty"`
	CreatedAt time.Time          `json:"createdAt"`
	UpdatedAt time.Time          `json:"updatedAt"`
}

type skillRevisionDTO struct {
	ID            string            `json:"id"`
	DefinitionID  string            `json:"definitionId"`
	Version       int               `json:"version"`
	Title         string            `json:"title"`
	Description   string            `json:"description"`
	Prompt        string            `json:"prompt"`
	Mode          string            `json:"mode"`
	PlannerPolicy map[string]string `json:"plannerPolicy,omitempty"`
	ToolAllowlist []string          `json:"toolAllowlist,omitempty"`
	CreatedAt     time.Time         `json:"createdAt"`
}

func mapSkill(skill domain.Skill) skillDTO {
	return skillDTO{
		ID:            skill.ID,
		UserID:        skill.UserID,
		DefinitionID:  skill.DefinitionID,
		RevisionID:    skill.RevisionID,
		Version:       skill.Version,
		Slug:          skill.Slug,
		Kind:          skill.Kind,
		Title:         skill.Title,
		Description:   skill.Description,
		Prompt:        skill.Prompt,
		Mode:          skill.Mode,
		PlannerPolicy: cloneStringMap(skill.PlannerPolicy),
		ToolAllowlist: append([]string(nil), skill.ToolAllowlist...),
		Source:        skill.Source,
		Enabled:       skill.Enabled,
		RepoURL:       skill.RepoURL,
		CreatedAt:     skill.CreatedAt,
		UpdatedAt:     skill.UpdatedAt,
	}
}

func mapSkills(skills []domain.Skill) []skillDTO {
	items := make([]skillDTO, 0, len(skills))
	for _, skill := range skills {
		items = append(items, mapSkill(skill))
	}
	return items
}

func mapSkillDefinition(definition domain.SkillDefinition) skillDefinitionDTO {
	return skillDefinitionDTO{
		ID:        definition.ID,
		UserID:    definition.UserID,
		Slug:      definition.Slug,
		Kind:      definition.Kind,
		Source:    definition.Source,
		RepoURL:   definition.RepoURL,
		CreatedAt: definition.CreatedAt,
		UpdatedAt: definition.UpdatedAt,
	}
}

func mapSkillDefinitions(definitions []domain.SkillDefinition) []skillDefinitionDTO {
	items := make([]skillDefinitionDTO, 0, len(definitions))
	for _, definition := range definitions {
		items = append(items, mapSkillDefinition(definition))
	}
	return items
}

func mapSkillRevision(revision domain.SkillRevision) skillRevisionDTO {
	return skillRevisionDTO{
		ID:            revision.ID,
		DefinitionID:  revision.DefinitionID,
		Version:       revision.Version,
		Title:         revision.Title,
		Description:   revision.Description,
		Prompt:        revision.Prompt,
		Mode:          revision.Mode,
		PlannerPolicy: cloneStringMap(revision.PlannerPolicy),
		ToolAllowlist: append([]string(nil), revision.ToolAllowlist...),
		CreatedAt:     revision.CreatedAt,
	}
}

func mapSkillRevisions(revisions []domain.SkillRevision) []skillRevisionDTO {
	items := make([]skillRevisionDTO, 0, len(revisions))
	for _, revision := range revisions {
		items = append(items, mapSkillRevision(revision))
	}
	return items
}
