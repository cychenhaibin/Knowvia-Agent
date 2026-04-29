package store

import (
	"database/sql"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skillruntime"
)

func scanSkill(scan scanner) (domain.Skill, error) {
	var skill domain.Skill
	var manifestJSON string
	var source string
	var kind string
	if err := scan.Scan(
		&skill.ID,
		&skill.UserID,
		&skill.DefinitionID,
		&skill.RevisionID,
		&skill.Version,
		&skill.Slug,
		&kind,
		&skill.Title,
		&skill.Description,
		&skill.Prompt,
		&skill.Mode,
		&manifestJSON,
		&source,
		&skill.Enabled,
		&skill.RepoURL,
		&skill.CreatedAt,
		&skill.UpdatedAt,
	); err != nil {
		return domain.Skill{}, normalizeError(err)
	}
	skill.Source = domain.SkillSource(source)
	skill.Kind = domain.SkillKind(kind)
	if err := skillruntime.ApplyRevisionManifest(&skill, manifestJSON); err != nil {
		return domain.Skill{}, err
	}
	return skill, nil
}

func scanSkillDefinition(scan scanner) (domain.SkillDefinition, error) {
	var definition domain.SkillDefinition
	var kind string
	var source string
	if err := scan.Scan(
		&definition.ID,
		&definition.UserID,
		&definition.Slug,
		&kind,
		&source,
		&definition.RepoURL,
		&definition.CreatedAt,
		&definition.UpdatedAt,
	); err != nil {
		return domain.SkillDefinition{}, normalizeError(err)
	}
	definition.Kind = domain.SkillKind(kind)
	definition.Source = domain.SkillSource(source)
	return definition, nil
}

func scanSkillRevision(scan scanner) (domain.SkillRevision, error) {
	var revision domain.SkillRevision
	if err := scan.Scan(
		&revision.ID,
		&revision.DefinitionID,
		&revision.Version,
		&revision.Title,
		&revision.Description,
		&revision.Prompt,
		&revision.Mode,
		&revision.ManifestJSON,
		&revision.CreatedAt,
	); err != nil {
		return domain.SkillRevision{}, normalizeError(err)
	}
	if err := skillruntime.ApplyRevisionManifestToRevision(&revision, revision.ManifestJSON); err != nil {
		return domain.SkillRevision{}, err
	}
	return revision, nil
}

func scanSkillInstallation(scan scanner) (domain.SkillInstallation, error) {
	var installation domain.SkillInstallation
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
	); err != nil {
		return domain.SkillInstallation{}, normalizeError(err)
	}
	return installation, nil
}

func scanSkillArtifact(scan scanner) (domain.SkillArtifact, error) {
	var artifact domain.SkillArtifact
	var source string
	var archiveBytes []byte
	if err := scan.Scan(
		&artifact.ID,
		&artifact.UserID,
		&artifact.DefinitionID,
		&artifact.RevisionID,
		&source,
		&artifact.FileName,
		&artifact.MediaType,
		&artifact.SourceURL,
		&artifact.SHA256,
		&artifact.SizeBytes,
		&artifact.EntryPath,
		&artifact.ManifestPath,
		&artifact.InstructionsPath,
		&archiveBytes,
		&artifact.CreatedAt,
	); err != nil {
		return domain.SkillArtifact{}, normalizeError(err)
	}
	artifact.Source = domain.SkillSource(source)
	artifact.ArchiveBytes = append([]byte(nil), archiveBytes...)
	return artifact, nil
}

func scanSkillArtifactFile(scan scanner) (domain.SkillArtifactFile, error) {
	var file domain.SkillArtifactFile
	if err := scan.Scan(
		&file.ID,
		&file.ArtifactID,
		&file.UserID,
		&file.Path,
		&file.MediaType,
		&file.SizeBytes,
		&file.SHA256,
		&file.IsManifest,
		&file.IsInstructions,
		&file.CreatedAt,
	); err != nil {
		return domain.SkillArtifactFile{}, normalizeError(err)
	}
	return file, nil
}

func scanSkillImportJob(scan scanner) (domain.SkillImportJob, error) {
	var job domain.SkillImportJob
	var source string
	var status string
	var completedAt sql.NullTime
	if err := scan.Scan(
		&job.ID,
		&job.UserID,
		&source,
		&status,
		&job.ArtifactID,
		&job.DefinitionID,
		&job.RevisionID,
		&job.InstallationID,
		&job.ErrorMessage,
		&job.RequestJSON,
		&job.CreatedAt,
		&job.UpdatedAt,
		&completedAt,
	); err != nil {
		return domain.SkillImportJob{}, normalizeError(err)
	}
	job.Source = domain.SkillSource(source)
	job.Status = domain.SkillImportStatus(status)
	job.CompletedAt = nullTimePtr(completedAt)
	return job, nil
}
