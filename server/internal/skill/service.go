package skill

import (
	"errors"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skillartifact"
)

var (
	ErrDefinitionNotFound       = errors.New("skill definition not found")
	ErrInstallationNotFound     = errors.New("skill installation not found")
	ErrSkillConflict            = errors.New("skill already exists")
	ErrInstallationConflict     = errors.New("skill installation already exists")
	ErrRevisionNotFound         = errors.New("skill revision not found")
	ErrNoUpdatableFields        = errors.New("no updatable fields provided")
	ErrRevisionRequired         = errors.New("currentRevisionId is required")
	ErrDefinitionRequired       = errors.New("definitionId is required")
	ErrTitlePromptRequired      = errors.New("title and prompt are required")
	ErrImportJobNotFound        = errors.New("skill import job not found")
	ErrArtifactNotFound         = errors.New("skill artifact not found")
	ErrDefinitionMismatch       = errors.New("skill definition mismatch")
	ErrInstallationDisabled     = errors.New("skill installation is disabled")
	ErrNoEnabledInstallation    = errors.New("no enabled skill installation available for skill definition")
	ErrArtifactFilePathRequired = skillartifact.ErrArtifactFilePathRequired
	ErrArtifactFileNotFound     = skillartifact.ErrArtifactFileNotFound
)

type ValidationErrorCode string

const (
	ValidationInstallationOnlyFields  ValidationErrorCode = "installation_only_fields"
	ValidationInvalidFieldCombination ValidationErrorCode = "invalid_field_combination"
	ValidationNoUpdatableFields       ValidationErrorCode = "no_updatable_fields"
)

type ValidationError struct {
	Code          ValidationErrorCode
	Message       string
	Fields        []string
	AllowedFields []string
}

func (e *ValidationError) Error() string {
	return e.Message
}

type MirrorSyncError struct {
	Err error
}

func (e *MirrorSyncError) Error() string { return e.Err.Error() }

func (e *MirrorSyncError) Unwrap() error { return e.Err }

type Service struct {
	mutations     SkillMutationStore
	installations SkillInstallationStore
	records       SkillRecordStore
	definitions   SkillDefinitionStore
	importJobs    SkillImportJobStore
	artifacts     skillartifact.ArtifactFileStore
	selection     SkillSelectionStore
	mirror        MirrorSyncer
}

type CreateInput struct {
	DefinitionID      string
	CurrentRevisionID string
	Name              string
	IsDefault         *bool
	Slug              string
	Kind              string
	Title             string
	Description       string
	Prompt            string
	Mode              string
	Source            string
	Enabled           *bool
	RepoURL           string
	PlannerPolicy     map[string]string
	ToolAllowlist     []string
}

type UpdateInput struct {
	CurrentRevisionID *string
	Name              *string
	IsDefault         *bool
	Slug              *string
	Kind              *string
	Title             *string
	Description       *string
	Prompt            *string
	Mode              *string
	Source            *string
	Enabled           *bool
	RepoURL           *string
	PlannerPolicy     *map[string]string
	ToolAllowlist     *[]string
}

func NewService(deps ServiceDeps, mirror MirrorSyncer) *Service {
	return &Service{
		mutations:     deps.Mutations,
		installations: deps.Installations,
		records:       deps.Records,
		definitions:   deps.Definitions,
		importJobs:    deps.ImportJobs,
		artifacts:     deps.Artifacts,
		selection:     deps.Selection,
		mirror:        mirror,
	}
}
