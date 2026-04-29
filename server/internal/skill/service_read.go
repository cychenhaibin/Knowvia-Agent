package skill

import (
	"context"
	"errors"
	"strings"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/persistence"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skillartifact"
)

func (s *Service) ListSkills(ctx context.Context, userID string) ([]domain.Skill, error) {
	return s.records.ListSkills(ctx, userID)
}

func (s *Service) ListInstallations(ctx context.Context, userID string) ([]domain.SkillInstallationRecord, error) {
	return s.installations.ListSkillInstallations(ctx, userID)
}

func (s *Service) GetInstallation(ctx context.Context, userID, installationID string) (domain.SkillInstallationRecord, error) {
	record, err := s.installations.GetSkillInstallationRecord(ctx, userID, installationID)
	if err != nil {
		if errors.Is(err, persistence.ErrNotFound) {
			return domain.SkillInstallationRecord{}, ErrInstallationNotFound
		}
		return domain.SkillInstallationRecord{}, err
	}
	return record, nil
}

func (s *Service) ListDefinitions(ctx context.Context, userID string) ([]domain.SkillDefinition, error) {
	return s.definitions.ListSkillDefinitions(ctx, userID)
}

func (s *Service) GetDefinition(ctx context.Context, userID, definitionID string) (domain.SkillDefinitionDetails, error) {
	details, err := s.definitions.GetSkillDefinitionDetails(ctx, userID, definitionID)
	if err != nil {
		if errors.Is(err, persistence.ErrNotFound) {
			return domain.SkillDefinitionDetails{}, ErrDefinitionNotFound
		}
		return domain.SkillDefinitionDetails{}, err
	}
	return details, nil
}

func (s *Service) ListRevisions(ctx context.Context, userID, definitionID string) ([]domain.SkillRevision, error) {
	revisions, err := s.definitions.ListSkillRevisions(ctx, userID, definitionID)
	if err != nil {
		if errors.Is(err, persistence.ErrNotFound) {
			return nil, ErrDefinitionNotFound
		}
		return nil, err
	}
	return revisions, nil
}

func (s *Service) ListImportJobs(ctx context.Context, userID string) ([]domain.SkillImportJob, error) {
	return s.importJobs.ListSkillImportJobs(ctx, userID)
}

func (s *Service) GetImportJobDetails(ctx context.Context, userID, jobID string) (domain.SkillImportJob, *domain.SkillArtifact, error) {
	job, err := s.importJobs.GetSkillImportJob(ctx, userID, jobID)
	if err != nil {
		if errors.Is(err, persistence.ErrNotFound) {
			return domain.SkillImportJob{}, nil, ErrImportJobNotFound
		}
		return domain.SkillImportJob{}, nil, err
	}

	var artifact *domain.SkillArtifact
	if strings.TrimSpace(job.ArtifactID) != "" {
		item, err := s.artifacts.GetSkillArtifact(ctx, userID, job.ArtifactID)
		if err == nil {
			artifact = &item
		} else if !errors.Is(err, persistence.ErrNotFound) {
			return domain.SkillImportJob{}, nil, err
		}
	}
	return job, artifact, nil
}

func (s *Service) ListArtifactFiles(ctx context.Context, userID, artifactID string) ([]domain.SkillArtifactFile, error) {
	files, err := s.artifacts.ListSkillArtifactFiles(ctx, userID, artifactID)
	if err != nil {
		if errors.Is(err, persistence.ErrNotFound) {
			return nil, ErrArtifactNotFound
		}
		return nil, err
	}
	return files, nil
}

func (s *Service) LoadArtifactFile(ctx context.Context, userID, artifactID, filePath string) (domain.SkillArtifactFile, []byte, error) {
	item, content, err := skillartifact.LoadFile(ctx, s.artifacts, userID, artifactID, filePath)
	if err != nil {
		if errors.Is(err, persistence.ErrNotFound) {
			return domain.SkillArtifactFile{}, nil, ErrArtifactNotFound
		}
		return domain.SkillArtifactFile{}, nil, err
	}
	return item, content, nil
}
