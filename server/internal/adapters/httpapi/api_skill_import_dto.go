package httpapi

import (
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	skillsvc "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skill"
)

type skillImportJobListDTO struct {
	Items []skillImportJobDTO `json:"items"`
}

type skillArtifactFileListDTO struct {
	Items []skillArtifactFileDTO `json:"items"`
}

type skillImportJobDetailsDTO struct {
	Job      skillImportJobDTO `json:"job"`
	Artifact *skillArtifactDTO `json:"artifact,omitempty"`
}

type skillArtifactDTO struct {
	ID               string             `json:"id"`
	UserID           string             `json:"userId"`
	DefinitionID     string             `json:"definitionId,omitempty"`
	RevisionID       string             `json:"revisionId,omitempty"`
	Source           domain.SkillSource `json:"source"`
	FileName         string             `json:"fileName"`
	MediaType        string             `json:"mediaType,omitempty"`
	SourceURL        string             `json:"sourceUrl,omitempty"`
	SHA256           string             `json:"sha256"`
	SizeBytes        int64              `json:"sizeBytes"`
	EntryPath        string             `json:"entryPath,omitempty"`
	ManifestPath     string             `json:"manifestPath,omitempty"`
	InstructionsPath string             `json:"instructionsPath,omitempty"`
	CreatedAt        time.Time          `json:"createdAt"`
}

type skillArtifactFileDTO struct {
	ID             string    `json:"id"`
	ArtifactID     string    `json:"artifactId"`
	UserID         string    `json:"userId"`
	Path           string    `json:"path"`
	MediaType      string    `json:"mediaType,omitempty"`
	SizeBytes      int64     `json:"sizeBytes"`
	SHA256         string    `json:"sha256"`
	IsManifest     bool      `json:"isManifest"`
	IsInstructions bool      `json:"isInstructions"`
	CreatedAt      time.Time `json:"createdAt"`
}

type skillImportJobDTO struct {
	ID             string                   `json:"id"`
	UserID         string                   `json:"userId"`
	Source         domain.SkillSource       `json:"source"`
	Status         domain.SkillImportStatus `json:"status"`
	ArtifactID     string                   `json:"artifactId,omitempty"`
	DefinitionID   string                   `json:"definitionId,omitempty"`
	RevisionID     string                   `json:"revisionId,omitempty"`
	InstallationID string                   `json:"installationId,omitempty"`
	ErrorMessage   string                   `json:"errorMessage,omitempty"`
	CreatedAt      time.Time                `json:"createdAt"`
	UpdatedAt      time.Time                `json:"updatedAt"`
	CompletedAt    *time.Time               `json:"completedAt,omitempty"`
}

func mapSkillArtifact(artifact domain.SkillArtifact) skillArtifactDTO {
	return skillArtifactDTO{
		ID:               artifact.ID,
		UserID:           artifact.UserID,
		DefinitionID:     artifact.DefinitionID,
		RevisionID:       artifact.RevisionID,
		Source:           artifact.Source,
		FileName:         artifact.FileName,
		MediaType:        artifact.MediaType,
		SourceURL:        artifact.SourceURL,
		SHA256:           artifact.SHA256,
		SizeBytes:        artifact.SizeBytes,
		EntryPath:        artifact.EntryPath,
		ManifestPath:     artifact.ManifestPath,
		InstructionsPath: artifact.InstructionsPath,
		CreatedAt:        artifact.CreatedAt,
	}
}

func mapSkillArtifactFile(file domain.SkillArtifactFile) skillArtifactFileDTO {
	return skillArtifactFileDTO{
		ID:             file.ID,
		ArtifactID:     file.ArtifactID,
		UserID:         file.UserID,
		Path:           file.Path,
		MediaType:      file.MediaType,
		SizeBytes:      file.SizeBytes,
		SHA256:         file.SHA256,
		IsManifest:     file.IsManifest,
		IsInstructions: file.IsInstructions,
		CreatedAt:      file.CreatedAt,
	}
}

func mapSkillArtifactFiles(files []domain.SkillArtifactFile) []skillArtifactFileDTO {
	items := make([]skillArtifactFileDTO, 0, len(files))
	for _, file := range files {
		items = append(items, mapSkillArtifactFile(file))
	}
	return items
}

func mapSkillImportJob(job domain.SkillImportJob) skillImportJobDTO {
	return skillImportJobDTO{
		ID:             job.ID,
		UserID:         job.UserID,
		Source:         job.Source,
		Status:         job.Status,
		ArtifactID:     job.ArtifactID,
		DefinitionID:   job.DefinitionID,
		RevisionID:     job.RevisionID,
		InstallationID: job.InstallationID,
		ErrorMessage:   job.ErrorMessage,
		CreatedAt:      job.CreatedAt,
		UpdatedAt:      job.UpdatedAt,
		CompletedAt:    job.CompletedAt,
	}
}

func mapSkillImportJobs(jobs []domain.SkillImportJob) []skillImportJobDTO {
	items := make([]skillImportJobDTO, 0, len(jobs))
	for _, job := range jobs {
		items = append(items, mapSkillImportJob(job))
	}
	return items
}

func mapSkillImportResult(result skillsvc.ImportResult) skillImportJobDetailsDTO {
	dto := skillImportJobDetailsDTO{
		Job: mapSkillImportJob(result.Job),
	}
	if result.Artifact != nil {
		mapped := mapSkillArtifact(*result.Artifact)
		dto.Artifact = &mapped
	}
	return dto
}
