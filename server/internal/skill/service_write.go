package skill

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/persistence"
)

func (s *Service) CreateStandaloneSkill(ctx context.Context, userID string, input CreateInput) (domain.Skill, error) {
	if isExistingDefinitionInstallationRequest(input) {
		return domain.Skill{}, newValidationError(
			ValidationInstallationOnlyFields,
			"definitionId and currentRevisionId are only supported for skill installations",
			[]string{"definitionId", "currentRevisionId"},
			nil,
		)
	}
	if strings.TrimSpace(input.Name) != "" || input.IsDefault != nil {
		return domain.Skill{}, newValidationError(
			ValidationInstallationOnlyFields,
			"name and isDefault are only supported for skill installations",
			[]string{"name", "isDefault"},
			nil,
		)
	}
	return s.CreateSkill(ctx, userID, input)
}

func (s *Service) CreateInstallation(ctx context.Context, userID string, input CreateInput) (domain.SkillInstallationRecord, error) {
	if isExistingDefinitionInstallationRequest(input) {
		if hasNewSkillCreateFields(input) {
			return domain.SkillInstallationRecord{}, newValidationError(
				ValidationInvalidFieldCombination,
				"definitionId/currentRevisionId cannot be combined with new skill fields",
				presentCreateInputFields(input),
				nil,
			)
		}
		return s.CreateInstallationFromDefinition(ctx, userID, input)
	}
	return s.CreateInstalledSkill(ctx, userID, input)
}

func (s *Service) UpdateStandaloneSkill(ctx context.Context, userID, installationID string, input UpdateInput) (domain.Skill, error) {
	if input.CurrentRevisionID != nil {
		return domain.Skill{}, newValidationError(
			ValidationInstallationOnlyFields,
			"currentRevisionId is only supported for skill installations",
			[]string{"currentRevisionId"},
			nil,
		)
	}
	if hasSkillRevisionPatch(input) {
		return s.UpdateSkillRevision(ctx, userID, installationID, input)
	}
	if !hasSkillInstallationPatch(input) {
		return domain.Skill{}, newValidationError(
			ValidationNoUpdatableFields,
			ErrNoUpdatableFields.Error(),
			nil,
			supportedSkillUpdateFields(),
		)
	}
	return s.UpdateSkillAfterInstallationPatch(ctx, userID, installationID, input)
}

func (s *Service) UpdateInstallationFromInput(ctx context.Context, userID, installationID string, input UpdateInput) (domain.SkillInstallationRecord, error) {
	if input.CurrentRevisionID != nil {
		if hasSkillRevisionPatch(input) {
			return domain.SkillInstallationRecord{}, newValidationError(
				ValidationInvalidFieldCombination,
				"currentRevisionId cannot be combined with skill definition or revision fields",
				presentUpdateInputFields(input),
				nil,
			)
		}
		return s.UpdateInstallation(ctx, userID, installationID, input)
	}
	if hasSkillRevisionPatch(input) {
		return s.UpdateInstallationRevision(ctx, userID, installationID, input)
	}
	if !hasSkillInstallationPatch(input) {
		return domain.SkillInstallationRecord{}, newValidationError(
			ValidationNoUpdatableFields,
			ErrNoUpdatableFields.Error(),
			nil,
			supportedSkillInstallationUpdateFields(),
		)
	}
	return s.UpdateInstallation(ctx, userID, installationID, input)
}

func (s *Service) CreateSkill(ctx context.Context, userID string, input CreateInput) (domain.Skill, error) {
	skill, err := buildSkill(userID, input)
	if err != nil {
		return domain.Skill{}, err
	}
	if err := s.mutations.CreateSkill(ctx, skill); err != nil {
		if errors.Is(err, persistence.ErrConflict) {
			return domain.Skill{}, ErrSkillConflict
		}
		return domain.Skill{}, err
	}
	if err := s.syncStoredSkill(ctx, userID, skill.ID); err != nil {
		return domain.Skill{}, err
	}
	return skill, nil
}

func (s *Service) CreateInstallationFromDefinition(ctx context.Context, userID string, input CreateInput) (domain.SkillInstallationRecord, error) {
	installation, err := s.buildInstallation(userID, ctx, input)
	if err != nil {
		return domain.SkillInstallationRecord{}, err
	}
	if err := s.installations.CreateSkillInstallation(ctx, installation); err != nil {
		if errors.Is(err, persistence.ErrConflict) {
			return domain.SkillInstallationRecord{}, ErrInstallationConflict
		}
		return domain.SkillInstallationRecord{}, err
	}
	record, err := s.installations.GetSkillInstallationRecord(ctx, installation.UserID, installation.ID)
	if err != nil {
		return domain.SkillInstallationRecord{}, err
	}
	if err := s.syncStoredSkill(ctx, installation.UserID, installation.ID); err != nil {
		return domain.SkillInstallationRecord{}, err
	}
	return record, nil
}

func (s *Service) CreateInstalledSkill(ctx context.Context, userID string, input CreateInput) (domain.SkillInstallationRecord, error) {
	skill, err := buildSkill(userID, input)
	if err != nil {
		return domain.SkillInstallationRecord{}, err
	}
	if err := s.mutations.CreateSkill(ctx, skill); err != nil {
		if errors.Is(err, persistence.ErrConflict) {
			return domain.SkillInstallationRecord{}, ErrSkillConflict
		}
		return domain.SkillInstallationRecord{}, err
	}
	record, err := s.installations.GetSkillInstallationRecord(ctx, skill.UserID, skill.ID)
	if err != nil {
		return domain.SkillInstallationRecord{}, err
	}
	if input.Name != "" || input.IsDefault != nil {
		record.Installation = applyCreateInstallationMetadata(record.Installation, input)
		record.Installation.UpdatedAt = time.Now().UTC()
		if err := s.installations.UpdateSkillInstallation(ctx, record.Installation); err != nil {
			return domain.SkillInstallationRecord{}, err
		}
		record, err = s.installations.GetSkillInstallationRecord(ctx, skill.UserID, skill.ID)
		if err != nil {
			return domain.SkillInstallationRecord{}, err
		}
	}
	if err := s.syncStoredSkill(ctx, skill.UserID, skill.ID); err != nil {
		return domain.SkillInstallationRecord{}, err
	}
	return record, nil
}

func (s *Service) UpdateSkillRevision(ctx context.Context, userID, installationID string, input UpdateInput) (domain.Skill, error) {
	skill, err := s.records.GetSkill(ctx, userID, installationID)
	if err != nil {
		if errors.Is(err, persistence.ErrNotFound) {
			return domain.Skill{}, ErrInstallationNotFound
		}
		return domain.Skill{}, err
	}
	skill, err = applySkillPatch(skill, input)
	if err != nil {
		return domain.Skill{}, err
	}
	if err := s.mutations.UpdateSkill(ctx, skill); err != nil {
		if errors.Is(err, persistence.ErrNotFound) {
			return domain.Skill{}, ErrInstallationNotFound
		}
		return domain.Skill{}, err
	}
	if err := s.syncSkill(ctx, skill); err != nil {
		return domain.Skill{}, err
	}
	return skill, nil
}

func (s *Service) UpdateInstallation(ctx context.Context, userID, installationID string, input UpdateInput) (domain.SkillInstallationRecord, error) {
	record, err := s.installations.GetSkillInstallationRecord(ctx, userID, installationID)
	if err != nil {
		if errors.Is(err, persistence.ErrNotFound) {
			return domain.SkillInstallationRecord{}, ErrInstallationNotFound
		}
		return domain.SkillInstallationRecord{}, err
	}
	if err := applyInstallationPatch(&record.Installation, input); err != nil {
		return domain.SkillInstallationRecord{}, err
	}
	if err := s.installations.UpdateSkillInstallation(ctx, record.Installation); err != nil {
		if errors.Is(err, persistence.ErrNotFound) && input.CurrentRevisionID != nil {
			return domain.SkillInstallationRecord{}, ErrRevisionNotFound
		}
		if errors.Is(err, persistence.ErrNotFound) {
			return domain.SkillInstallationRecord{}, ErrInstallationNotFound
		}
		return domain.SkillInstallationRecord{}, err
	}
	record, err = s.installations.GetSkillInstallationRecord(ctx, userID, installationID)
	if err != nil {
		if errors.Is(err, persistence.ErrNotFound) {
			return domain.SkillInstallationRecord{}, ErrInstallationNotFound
		}
		return domain.SkillInstallationRecord{}, err
	}
	if err := s.syncStoredSkill(ctx, userID, installationID); err != nil {
		return domain.SkillInstallationRecord{}, err
	}
	return record, nil
}

func (s *Service) UpdateInstallationRevision(ctx context.Context, userID, installationID string, input UpdateInput) (domain.SkillInstallationRecord, error) {
	skill, err := s.UpdateSkillRevision(ctx, userID, installationID, input)
	if err != nil {
		return domain.SkillInstallationRecord{}, err
	}
	return s.GetInstallation(ctx, userID, skill.ID)
}

func (s *Service) UpdateSkillAfterInstallationPatch(ctx context.Context, userID, installationID string, input UpdateInput) (domain.Skill, error) {
	if _, err := s.UpdateInstallation(ctx, userID, installationID, input); err != nil {
		return domain.Skill{}, err
	}
	skill, err := s.records.GetSkill(ctx, userID, installationID)
	if errors.Is(err, persistence.ErrNotFound) {
		return domain.Skill{}, ErrInstallationNotFound
	}
	return skill, err
}

func (s *Service) DeleteSkill(ctx context.Context, userID, installationID string) error {
	if err := s.mutations.DeleteSkill(ctx, userID, installationID); err != nil {
		if errors.Is(err, persistence.ErrNotFound) {
			return ErrInstallationNotFound
		}
		return err
	}
	if s.mirror == nil {
		return nil
	}
	return s.mirror.DeleteSkill(ctx, userID, installationID)
}
