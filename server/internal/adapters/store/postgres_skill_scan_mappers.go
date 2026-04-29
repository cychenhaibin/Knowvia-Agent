package store

import (
	sqldb "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/db"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skillruntime"
)

func mapDBSkill(row sqldb.GetSkillRow) (domain.Skill, error) {
	skill := domain.Skill{
		ID:           row.ID,
		UserID:       row.UserID,
		DefinitionID: row.ID_2,
		RevisionID:   row.ID_3,
		Version:      int(row.Version),
		Slug:         row.Slug,
		Kind:         domain.SkillKind(row.Kind),
		Title:        row.Title,
		Description:  row.Description,
		Prompt:       row.Prompt,
		Mode:         row.Mode,
		Source:       domain.SkillSource(row.Source),
		Enabled:      row.Enabled,
		RepoURL:      row.RepoUrl,
		CreatedAt:    pgTimestamptzValue(row.CreatedAt),
		UpdatedAt:    pgTimestamptzValue(row.UpdatedAt),
	}
	if err := skillruntime.ApplyRevisionManifest(&skill, row.ManifestJson); err != nil {
		return domain.Skill{}, err
	}
	return skill, nil
}

func mapDBSkillListRow(row sqldb.ListSkillsRow) (domain.Skill, error) {
	return mapDBSkill(sqldb.GetSkillRow(row))
}

func mapDBSkillDefinition(row sqldb.SkillDefinition) domain.SkillDefinition {
	return domain.SkillDefinition{
		ID:        row.ID,
		UserID:    row.UserID,
		Slug:      row.Slug,
		Kind:      domain.SkillKind(row.Kind),
		Source:    domain.SkillSource(row.Source),
		RepoURL:   row.RepoUrl,
		CreatedAt: pgTimestamptzValue(row.CreatedAt),
		UpdatedAt: pgTimestamptzValue(row.UpdatedAt),
	}
}

func mapDBSkillRevision(row sqldb.SkillRevision) (domain.SkillRevision, error) {
	revision := domain.SkillRevision{
		ID:           row.ID,
		DefinitionID: row.DefinitionID,
		Version:      int(row.Version),
		Title:        row.Title,
		Description:  row.Description,
		Prompt:       row.Prompt,
		Mode:         row.Mode,
		ManifestJSON: row.ManifestJson,
		CreatedAt:    pgTimestamptzValue(row.CreatedAt),
	}
	if err := skillruntime.ApplyRevisionManifestToRevision(&revision, row.ManifestJson); err != nil {
		return domain.SkillRevision{}, err
	}
	return revision, nil
}

func mapDBSkillInstallation(row sqldb.ListDefinitionInstallationsRow) domain.SkillInstallation {
	return domain.SkillInstallation{
		ID:                row.ID,
		UserID:            row.UserID,
		DefinitionID:      row.DefinitionID,
		CurrentRevisionID: row.CurrentRevisionID,
		Name:              row.Name,
		IsDefault:         row.IsDefault,
		Enabled:           row.Enabled,
		CreatedAt:         pgTimestamptzValue(row.CreatedAt),
		UpdatedAt:         pgTimestamptzValue(row.UpdatedAt),
	}
}

func mapDBSkillArtifact(row sqldb.SkillArtifact) domain.SkillArtifact {
	return domain.SkillArtifact{
		ID:               row.ID,
		UserID:           row.UserID,
		DefinitionID:     row.DefinitionID,
		RevisionID:       row.RevisionID,
		Source:           domain.SkillSource(row.Source),
		FileName:         row.FileName,
		MediaType:        row.MediaType,
		SourceURL:        row.SourceUrl,
		SHA256:           row.Sha256,
		SizeBytes:        row.SizeBytes,
		EntryPath:        row.EntryPath,
		ManifestPath:     row.ManifestPath,
		InstructionsPath: row.InstructionsPath,
		ArchiveBytes:     append([]byte(nil), row.ArchiveBytes...),
		CreatedAt:        pgTimestamptzValue(row.CreatedAt),
	}
}

func mapDBSkillArtifactFile(row sqldb.SkillArtifactFile) domain.SkillArtifactFile {
	return domain.SkillArtifactFile{
		ID:             row.ID,
		ArtifactID:     row.ArtifactID,
		UserID:         row.UserID,
		Path:           row.Path,
		MediaType:      row.MediaType,
		SizeBytes:      row.SizeBytes,
		SHA256:         row.Sha256,
		IsManifest:     row.IsManifest,
		IsInstructions: row.IsInstructions,
		CreatedAt:      pgTimestamptzValue(row.CreatedAt),
	}
}

func mapDBSkillImportJob(row sqldb.SkillImportJob) domain.SkillImportJob {
	return domain.SkillImportJob{
		ID:             row.ID,
		UserID:         row.UserID,
		Source:         domain.SkillSource(row.Source),
		Status:         domain.SkillImportStatus(row.Status),
		ArtifactID:     row.ArtifactID,
		DefinitionID:   row.DefinitionID,
		RevisionID:     row.RevisionID,
		InstallationID: row.InstallationID,
		ErrorMessage:   row.ErrorMessage,
		RequestJSON:    row.RequestJson,
		CreatedAt:      pgTimestamptzValue(row.CreatedAt),
		UpdatedAt:      pgTimestamptzValue(row.UpdatedAt),
		CompletedAt:    pgNullableTimestamptzPtr(row.CompletedAt),
	}
}
