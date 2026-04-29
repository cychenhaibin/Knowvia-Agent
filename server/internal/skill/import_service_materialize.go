package skill

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	parserpkg "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skillimport"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skillruntime"
)

func (s *ImportService) materializeImportedSkill(
	ctx context.Context,
	userID string,
	job domain.SkillImportJob,
	parsed parserpkg.ParsedPackage,
	install *bool,
	installationName string,
	isDefault *bool,
	enabled *bool,
) (ImportResult, error) {
	now := time.Now().UTC()
	definitionID := uuid.NewString()
	revisionID := uuid.NewString()

	skillModel := domain.Skill{
		ID:            "",
		UserID:        userID,
		DefinitionID:  definitionID,
		RevisionID:    revisionID,
		Version:       1,
		Slug:          parsed.Slug,
		Kind:          parsed.Kind,
		Title:         parsed.Title,
		Description:   parsed.Description,
		Prompt:        parsed.Prompt,
		Mode:          parsed.Mode,
		PlannerPolicy: clonePlannerPolicy(parsed.PlannerPolicy),
		ToolAllowlist: append([]string(nil), parsed.ToolAllowlist...),
		Source:        parsed.Source,
		RepoURL:       parsed.RepoURL,
		Enabled:       enabled == nil || *enabled,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	manifestJSON, err := skillruntime.EncodeRevisionManifest(skillModel)
	if err != nil {
		s.failSkillImportJob(ctx, &job, err)
		return ImportResult{}, err
	}

	definition := domain.SkillDefinition{
		ID:        definitionID,
		UserID:    userID,
		Slug:      parsed.Slug,
		Kind:      parsed.Kind,
		Source:    parsed.Source,
		RepoURL:   parsed.RepoURL,
		CreatedAt: now,
		UpdatedAt: now,
	}
	revision := domain.SkillRevision{
		ID:            revisionID,
		DefinitionID:  definitionID,
		Version:       1,
		Title:         parsed.Title,
		Description:   parsed.Description,
		Prompt:        parsed.Prompt,
		Mode:          parsed.Mode,
		PlannerPolicy: clonePlannerPolicy(parsed.PlannerPolicy),
		ToolAllowlist: append([]string(nil), parsed.ToolAllowlist...),
		ManifestJSON:  manifestJSON,
		CreatedAt:     now,
	}
	if err := s.definitions.CreateSkillDefinitionRevision(ctx, definition, revision); err != nil {
		s.failSkillImportJob(ctx, &job, err)
		return ImportResult{}, mapImportError(err)
	}

	artifact := domain.SkillArtifact{
		ID:               uuid.NewString(),
		UserID:           userID,
		DefinitionID:     definitionID,
		RevisionID:       revisionID,
		Source:           parsed.Source,
		FileName:         parsed.FileName,
		MediaType:        parsed.MediaType,
		SourceURL:        parsed.SourceURL,
		SHA256:           parsed.SHA256,
		SizeBytes:        parsed.SizeBytes,
		EntryPath:        parsed.EntryPath,
		ManifestPath:     parsed.ManifestPath,
		InstructionsPath: parsed.InstructionsPath,
		ArchiveBytes:     append([]byte(nil), parsed.ArchiveBytes...),
		CreatedAt:        now,
	}
	if err := s.artifacts.CreateSkillArtifact(ctx, artifact); err != nil {
		s.failSkillImportJob(ctx, &job, err)
		return ImportResult{}, mapImportError(err)
	}
	files := make([]domain.SkillArtifactFile, 0, len(parsed.Files))
	for _, item := range parsed.Files {
		files = append(files, domain.SkillArtifactFile{
			ID:             uuid.NewString(),
			ArtifactID:     artifact.ID,
			UserID:         userID,
			Path:           item.Path,
			MediaType:      item.MediaType,
			SizeBytes:      item.SizeBytes,
			SHA256:         item.SHA256,
			IsManifest:     item.IsManifest,
			IsInstructions: item.IsInstructions,
			CreatedAt:      now,
		})
	}
	if err := s.artifacts.ReplaceSkillArtifactFiles(ctx, artifact.ID, files); err != nil {
		s.failSkillImportJob(ctx, &job, err)
		return ImportResult{}, mapImportError(err)
	}

	job.ArtifactID = artifact.ID
	job.DefinitionID = definitionID
	job.RevisionID = revisionID

	shouldInstall := true
	if install != nil {
		shouldInstall = *install
	}
	if shouldInstall {
		installation := domain.SkillInstallation{
			ID:                uuid.NewString(),
			UserID:            userID,
			DefinitionID:      definitionID,
			CurrentRevisionID: revisionID,
			Name:              strings.TrimSpace(installationName),
			IsDefault:         isDefault != nil && *isDefault,
			Enabled:           enabled == nil || *enabled,
			CreatedAt:         now,
			UpdatedAt:         now,
		}
		if err := s.installations.CreateSkillInstallation(ctx, installation); err != nil {
			s.failSkillImportJob(ctx, &job, err)
			return ImportResult{}, mapImportError(err)
		}
		job.InstallationID = installation.ID
		storedSkill, err := s.records.GetSkill(ctx, userID, installation.ID)
		if err != nil {
			s.failSkillImportJob(ctx, &job, err)
			return ImportResult{}, mapImportError(err)
		}
		if s.mirror != nil {
			if err := s.mirror.SyncSkill(ctx, storedSkill); err != nil {
				s.failSkillImportJob(ctx, &job, err)
				return ImportResult{}, mapImportError(err)
			}
		}
	}

	completedAt := time.Now().UTC()
	job.Status = domain.SkillImportCompleted
	job.ErrorMessage = ""
	job.UpdatedAt = completedAt
	job.CompletedAt = &completedAt
	if err := s.jobs.UpdateSkillImportJob(ctx, job); err != nil {
		return ImportResult{}, mapImportError(err)
	}
	return ImportResult{
		Job:      job,
		Artifact: &artifact,
	}, nil
}

func clonePlannerPolicy(source map[string]string) map[string]string {
	if len(source) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}
