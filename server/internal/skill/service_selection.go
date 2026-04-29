package skill

import (
	"context"
	"errors"
	"strings"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/persistence"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skillresolver"
)

func (s *Service) ResolveChatSelection(ctx context.Context, userID, installationID, definitionID string) (string, domain.Skill, error) {
	record, err := skillresolver.ResolveInstallationRecord(ctx, s.selection, userID, installationID, definitionID)
	if err != nil {
		return "", domain.Skill{}, mapSkillSelectionError(installationID, definitionID, err)
	}
	storedSkill, err := s.selection.GetSkill(ctx, userID, record.Installation.ID)
	if err != nil {
		if errors.Is(err, persistence.ErrNotFound) {
			return "", domain.Skill{}, ErrInstallationNotFound
		}
		return "", domain.Skill{}, err
	}
	return record.Installation.ID, storedSkill, nil
}

func mapSkillSelectionError(installationID, definitionID string, err error) error {
	definitionOnly := strings.TrimSpace(definitionID) != "" && strings.TrimSpace(installationID) == ""
	switch {
	case errors.Is(err, persistence.ErrNotFound) && definitionOnly:
		return ErrDefinitionNotFound
	case errors.Is(err, persistence.ErrNotFound):
		return ErrInstallationNotFound
	case errors.Is(err, skillresolver.ErrDefinitionMismatch):
		return ErrDefinitionMismatch
	case errors.Is(err, skillresolver.ErrDisabled) && definitionOnly:
		return ErrNoEnabledInstallation
	case errors.Is(err, skillresolver.ErrDisabled):
		return ErrInstallationDisabled
	default:
		return err
	}
}

func (s *Service) syncStoredSkill(ctx context.Context, userID, skillID string) error {
	storedSkill, err := s.records.GetSkill(ctx, userID, skillID)
	if err != nil {
		if errors.Is(err, persistence.ErrNotFound) {
			return ErrInstallationNotFound
		}
		return err
	}
	return s.syncSkill(ctx, storedSkill)
}

func (s *Service) syncSkill(ctx context.Context, skill domain.Skill) error {
	if s.mirror == nil {
		return nil
	}
	if err := s.mirror.SyncSkill(ctx, skill); err != nil {
		return &MirrorSyncError{Err: err}
	}
	return nil
}
