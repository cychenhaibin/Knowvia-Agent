package skillresolver

import (
	"context"
	"errors"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/persistence"
)

var ErrDisabled = errors.New("skill installation disabled")
var ErrDefinitionMismatch = errors.New("skill installation does not belong to skill definition")

type InstallationRecordStore interface {
	GetSkillInstallationRecord(ctx context.Context, userID, installationID string) (domain.SkillInstallationRecord, error)
	GetSkillDefinitionDetails(ctx context.Context, userID, definitionID string) (domain.SkillDefinitionDetails, error)
}

func ResolveInstallationRecord(
	ctx context.Context,
	st InstallationRecordStore,
	userID, installationID, definitionID string,
) (domain.SkillInstallationRecord, error) {
	if installationID != "" {
		record, err := st.GetSkillInstallationRecord(ctx, userID, installationID)
		if err != nil {
			return domain.SkillInstallationRecord{}, err
		}
		if definitionID != "" && record.Installation.DefinitionID != definitionID {
			return domain.SkillInstallationRecord{}, ErrDefinitionMismatch
		}
		if !record.Installation.Enabled {
			return domain.SkillInstallationRecord{}, ErrDisabled
		}
		return record, nil
	}
	if definitionID == "" {
		return domain.SkillInstallationRecord{}, persistence.ErrNotFound
	}

	details, err := st.GetSkillDefinitionDetails(ctx, userID, definitionID)
	if err != nil {
		return domain.SkillInstallationRecord{}, err
	}
	selectedID, err := selectEnabledInstallation(details.Installations)
	if err != nil {
		return domain.SkillInstallationRecord{}, err
	}
	return st.GetSkillInstallationRecord(ctx, userID, selectedID)
}

func selectEnabledInstallation(installations []domain.SkillInstallation) (string, error) {
	if len(installations) == 0 {
		return "", persistence.ErrNotFound
	}
	var candidate *domain.SkillInstallation
	for i := range installations {
		installation := installations[i]
		if !installation.Enabled {
			continue
		}
		if installation.IsDefault {
			return installation.ID, nil
		}
		if candidate == nil || installation.UpdatedAt.After(candidate.UpdatedAt) {
			candidate = &installation
		}
	}
	if candidate != nil {
		return candidate.ID, nil
	}
	return "", ErrDisabled
}
