package skill

import (
	"context"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skillartifact"
	parserpkg "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skillimport"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skillresolver"
)

type SkillMutationStore interface {
	CreateSkill(context.Context, domain.Skill) error
	UpdateSkill(context.Context, domain.Skill) error
	DeleteSkill(context.Context, string, string) error
}

type SkillInstallationStore interface {
	CreateSkillInstallation(context.Context, domain.SkillInstallation) error
	GetSkillInstallationRecord(context.Context, string, string) (domain.SkillInstallationRecord, error)
	ListSkillInstallations(context.Context, string) ([]domain.SkillInstallationRecord, error)
	UpdateSkillInstallation(context.Context, domain.SkillInstallation) error
}

type SkillRecordStore interface {
	GetSkill(context.Context, string, string) (domain.Skill, error)
	ListSkills(context.Context, string) ([]domain.Skill, error)
}

type SkillDefinitionStore interface {
	ListSkillDefinitions(context.Context, string) ([]domain.SkillDefinition, error)
	GetSkillDefinitionDetails(context.Context, string, string) (domain.SkillDefinitionDetails, error)
	ListSkillRevisions(context.Context, string, string) ([]domain.SkillRevision, error)
}

type SkillImportJobStore interface {
	GetSkillImportJob(context.Context, string, string) (domain.SkillImportJob, error)
	ListSkillImportJobs(context.Context, string) ([]domain.SkillImportJob, error)
}

type SkillSelectionStore interface {
	skillresolver.InstallationRecordStore
	GetSkill(ctx context.Context, userID, skillID string) (domain.Skill, error)
}

type MirrorSyncer interface {
	SyncSkill(context.Context, domain.Skill) error
	DeleteSkill(context.Context, string, string) error
}

type ServiceDeps struct {
	Mutations     SkillMutationStore
	Installations SkillInstallationStore
	Records       SkillRecordStore
	Definitions   SkillDefinitionStore
	ImportJobs    SkillImportJobStore
	Artifacts     skillartifact.ArtifactFileStore
	Selection     SkillSelectionStore
}

type ImportDefinitionStore interface {
	CreateSkillDefinitionRevision(ctx context.Context, definition domain.SkillDefinition, revision domain.SkillRevision) error
	ListSkillDefinitions(ctx context.Context, userID string) ([]domain.SkillDefinition, error)
	GetSkillDefinitionDetails(ctx context.Context, userID, definitionID string) (domain.SkillDefinitionDetails, error)
	ListSkillRevisions(ctx context.Context, userID, definitionID string) ([]domain.SkillRevision, error)
}

type ImportArtifactStore interface {
	CreateSkillArtifact(ctx context.Context, artifact domain.SkillArtifact) error
	GetSkillArtifact(ctx context.Context, userID, artifactID string) (domain.SkillArtifact, error)
	ReplaceSkillArtifactFiles(ctx context.Context, artifactID string, files []domain.SkillArtifactFile) error
	ListSkillArtifactFiles(ctx context.Context, userID, artifactID string) ([]domain.SkillArtifactFile, error)
}

type ImportJobLifecycleStore interface {
	CreateSkillImportJob(ctx context.Context, job domain.SkillImportJob) error
	UpdateSkillImportJob(ctx context.Context, job domain.SkillImportJob) error
	GetSkillImportJob(ctx context.Context, userID, jobID string) (domain.SkillImportJob, error)
	ListSkillImportJobs(ctx context.Context, userID string) ([]domain.SkillImportJob, error)
}

type ImportDeps struct {
	Definitions   ImportDefinitionStore
	Installations SkillInstallationStore
	Artifacts     ImportArtifactStore
	Jobs          ImportJobLifecycleStore
	Records       SkillRecordStore
}

type ImportParser interface {
	ImportGitHubArchive(context.Context, parserpkg.GitHubImportRequest) (parserpkg.ParsedPackage, error)
	ImportUploadedArchive(string, string, string, []byte) (parserpkg.ParsedPackage, error)
}
