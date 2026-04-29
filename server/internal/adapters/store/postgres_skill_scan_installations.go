package store

import (
	sqldb "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/db"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skillruntime"
)

func scanSkillInstallationRecord(scan scanner) (domain.SkillInstallationRecord, error) {
	var installation domain.SkillInstallation
	var definition domain.SkillDefinition
	var revision domain.SkillRevision
	var definitionKind string
	var definitionSource string
	var manifestJSON string
	if err := scan.Scan(
		&installation.ID,
		&installation.UserID,
		&installation.DefinitionID,
		&installation.CurrentRevisionID,
		&installation.Name,
		&installation.IsDefault,
		&installation.Enabled,
		&installation.CreatedAt,
		&installation.UpdatedAt,
		&definition.ID,
		&definition.UserID,
		&definition.Slug,
		&definitionKind,
		&definitionSource,
		&definition.RepoURL,
		&definition.CreatedAt,
		&definition.UpdatedAt,
		&revision.ID,
		&revision.DefinitionID,
		&revision.Version,
		&revision.Title,
		&revision.Description,
		&revision.Prompt,
		&revision.Mode,
		&manifestJSON,
		&revision.CreatedAt,
	); err != nil {
		return domain.SkillInstallationRecord{}, normalizeError(err)
	}
	definition.Kind = domain.SkillKind(definitionKind)
	definition.Source = domain.SkillSource(definitionSource)
	revision.ManifestJSON = manifestJSON
	if err := skillruntime.ApplyRevisionManifestToRevision(&revision, manifestJSON); err != nil {
		return domain.SkillInstallationRecord{}, err
	}
	return domain.SkillInstallationRecord{
		Installation:    installation,
		Definition:      definition,
		CurrentRevision: revision,
	}, nil
}

func mapDBSkillInstallationRecord(row sqldb.GetSkillInstallationRecordRow) (domain.SkillInstallationRecord, error) {
	revision := domain.SkillRevision{
		ID:           row.ID_3,
		DefinitionID: row.DefinitionID_2,
		Version:      int(row.Version),
		Title:        row.Title,
		Description:  row.Description,
		Prompt:       row.Prompt,
		Mode:         row.Mode,
		ManifestJSON: row.ManifestJson,
		CreatedAt:    pgTimestamptzValue(row.CreatedAt_3),
	}
	if err := skillruntime.ApplyRevisionManifestToRevision(&revision, row.ManifestJson); err != nil {
		return domain.SkillInstallationRecord{}, err
	}
	return domain.SkillInstallationRecord{
		Installation: domain.SkillInstallation{
			ID:                row.ID,
			UserID:            row.UserID,
			DefinitionID:      row.DefinitionID,
			CurrentRevisionID: row.CurrentRevisionID,
			Name:              row.Name,
			IsDefault:         row.IsDefault,
			Enabled:           row.Enabled,
			CreatedAt:         pgTimestamptzValue(row.CreatedAt),
			UpdatedAt:         pgTimestamptzValue(row.UpdatedAt),
		},
		Definition: domain.SkillDefinition{
			ID:        row.ID_2,
			UserID:    row.UserID_2,
			Slug:      row.Slug,
			Kind:      domain.SkillKind(row.Kind),
			Source:    domain.SkillSource(row.Source),
			RepoURL:   row.RepoUrl,
			CreatedAt: pgTimestamptzValue(row.CreatedAt_2),
			UpdatedAt: pgTimestamptzValue(row.UpdatedAt_2),
		},
		CurrentRevision: revision,
	}, nil
}

func mapDBDefaultSkillInstallationRecord(row sqldb.GetDefaultSkillInstallationRecordRow) (domain.SkillInstallationRecord, error) {
	return mapDBSkillInstallationRecord(sqldb.GetSkillInstallationRecordRow(row))
}

func mapDBListedSkillInstallationRecord(row sqldb.ListSkillInstallationsRow) (domain.SkillInstallationRecord, error) {
	return mapDBSkillInstallationRecord(sqldb.GetSkillInstallationRecordRow(row))
}
