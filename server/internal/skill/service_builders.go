package skill

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/persistence"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/runtimepolicy"
)

func buildSkill(userID string, input CreateInput) (domain.Skill, error) {
	if strings.TrimSpace(input.Title) == "" || strings.TrimSpace(input.Prompt) == "" {
		return domain.Skill{}, ErrTitlePromptRequired
	}
	now := time.Now().UTC()
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	return domain.Skill{
		ID:            uuid.NewString(),
		UserID:        userID,
		DefinitionID:  uuid.NewString(),
		RevisionID:    uuid.NewString(),
		Version:       1,
		Slug:          fallbackSlug(strings.TrimSpace(input.Slug), strings.TrimSpace(input.Title)),
		Kind:          normalizeSkillKind(input.Kind),
		Title:         strings.TrimSpace(input.Title),
		Description:   strings.TrimSpace(input.Description),
		Prompt:        strings.TrimSpace(input.Prompt),
		Mode:          runtimepolicy.NormalizeSkillMode(input.Mode),
		PlannerPolicy: normalizeStringMap(input.PlannerPolicy),
		ToolAllowlist: normalizeStringSlice(input.ToolAllowlist),
		Source:        normalizeSkillSource(input.Source),
		Enabled:       enabled,
		RepoURL:       strings.TrimSpace(input.RepoURL),
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

func (s *Service) buildInstallation(userID string, ctx context.Context, input CreateInput) (domain.SkillInstallation, error) {
	definitionID := strings.TrimSpace(input.DefinitionID)
	if definitionID == "" {
		return domain.SkillInstallation{}, ErrDefinitionRequired
	}

	details, err := s.definitions.GetSkillDefinitionDetails(ctx, userID, definitionID)
	if err != nil {
		if errors.Is(err, persistence.ErrNotFound) {
			return domain.SkillInstallation{}, ErrDefinitionNotFound
		}
		return domain.SkillInstallation{}, err
	}
	if len(details.Revisions) == 0 {
		return domain.SkillInstallation{}, ErrDefinitionNotFound
	}

	revisionID := strings.TrimSpace(input.CurrentRevisionID)
	selectedRevision := details.Revisions[0]
	if revisionID == "" {
		revisionID = selectedRevision.ID
	} else {
		matched := false
		for _, revision := range details.Revisions {
			if revision.ID == revisionID {
				selectedRevision = revision
				matched = true
				break
			}
		}
		if !matched {
			return domain.SkillInstallation{}, ErrRevisionNotFound
		}
	}

	now := time.Now().UTC()
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = selectedRevision.Title
	}
	isDefault := len(details.Installations) == 0
	if input.IsDefault != nil {
		isDefault = isDefault || *input.IsDefault
	}
	return domain.SkillInstallation{
		ID:                uuid.NewString(),
		UserID:            userID,
		DefinitionID:      definitionID,
		CurrentRevisionID: revisionID,
		Name:              name,
		IsDefault:         isDefault,
		Enabled:           enabled,
		CreatedAt:         now,
		UpdatedAt:         now,
	}, nil
}

func applyCreateInstallationMetadata(installation domain.SkillInstallation, input CreateInput) domain.SkillInstallation {
	if strings.TrimSpace(input.Name) != "" {
		installation.Name = strings.TrimSpace(input.Name)
	}
	if input.IsDefault != nil {
		installation.IsDefault = installation.IsDefault || *input.IsDefault
	}
	return installation
}

func applySkillPatch(skill domain.Skill, input UpdateInput) (domain.Skill, error) {
	if input.Slug != nil {
		skill.Slug = fallbackSlug(strings.TrimSpace(*input.Slug), skill.Title)
	}
	if input.Kind != nil {
		skill.Kind = normalizeSkillKind(*input.Kind)
	}
	if input.Title != nil {
		skill.Title = strings.TrimSpace(*input.Title)
	}
	if input.Description != nil {
		skill.Description = strings.TrimSpace(*input.Description)
	}
	if input.Prompt != nil {
		skill.Prompt = strings.TrimSpace(*input.Prompt)
	}
	if input.Mode != nil {
		skill.Mode = runtimepolicy.NormalizeSkillMode(*input.Mode)
	}
	if input.Source != nil {
		skill.Source = normalizeSkillSource(*input.Source)
	}
	if input.Enabled != nil {
		skill.Enabled = *input.Enabled
	}
	if input.RepoURL != nil {
		skill.RepoURL = strings.TrimSpace(*input.RepoURL)
	}
	if input.PlannerPolicy != nil {
		skill.PlannerPolicy = normalizeStringMap(*input.PlannerPolicy)
	}
	if input.ToolAllowlist != nil {
		skill.ToolAllowlist = normalizeStringSlice(*input.ToolAllowlist)
	}
	if !hasSkillRevisionPatch(input) && !hasSkillInstallationPatch(input) {
		return domain.Skill{}, ErrNoUpdatableFields
	}
	skill.Version++
	skill.RevisionID = uuid.NewString()
	skill.UpdatedAt = time.Now().UTC()

	if strings.TrimSpace(skill.Title) == "" || strings.TrimSpace(skill.Prompt) == "" {
		return domain.Skill{}, ErrTitlePromptRequired
	}
	skill.Slug = fallbackSlug(skill.Slug, skill.Title)
	return skill, nil
}

func applyInstallationPatch(installation *domain.SkillInstallation, input UpdateInput) error {
	if input.CurrentRevisionID == nil && !hasSkillInstallationPatch(input) {
		return ErrNoUpdatableFields
	}
	if input.CurrentRevisionID != nil {
		revisionID := strings.TrimSpace(*input.CurrentRevisionID)
		if revisionID == "" {
			return ErrRevisionRequired
		}
		installation.CurrentRevisionID = revisionID
	}
	if input.Enabled != nil {
		installation.Enabled = *input.Enabled
	}
	if input.Name != nil {
		installation.Name = strings.TrimSpace(*input.Name)
	}
	if input.IsDefault != nil {
		installation.IsDefault = *input.IsDefault
	}
	installation.UpdatedAt = time.Now().UTC()
	return nil
}
