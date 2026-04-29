package skill

import (
	"context"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	parserpkg "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skillimport"
)

type ImportService struct {
	definitions   ImportDefinitionStore
	installations SkillInstallationStore
	artifacts     ImportArtifactStore
	jobs          ImportJobLifecycleStore
	records       SkillRecordStore
	parser        ImportParser
	mirror        MirrorSyncer
}

type ImportGitHubInput struct {
	RepoURL          string
	Ref              string
	Path             string
	Install          *bool
	InstallationName string
	IsDefault        *bool
	Enabled          *bool
}

type ImportUploadInput struct {
	FileName         string
	MediaType        string
	Path             string
	ArchiveBytes     []byte
	Install          bool
	InstallationName string
	IsDefault        *bool
	Enabled          *bool
}

var (
	ErrUnsupportedRepoURL    = parserpkg.ErrUnsupportedRepoURL
	ErrArchiveTooLarge       = parserpkg.ErrArchiveTooLarge
	ErrInvalidArchive        = parserpkg.ErrInvalidArchive
	ErrSkillPackageNotFound  = parserpkg.ErrSkillPackageNotFound
	ErrInvalidManifest       = parserpkg.ErrInvalidManifest
	ErrIncompleteSkillPrompt = parserpkg.ErrIncompleteSkillPrompt
)

type GitHubArchiveDownloadError struct {
	Err error
}

func (e *GitHubArchiveDownloadError) Error() string { return e.Err.Error() }

func (e *GitHubArchiveDownloadError) Unwrap() error { return e.Err }

func NewImportService(deps ImportDeps, parser ImportParser, mirror MirrorSyncer) *ImportService {
	return &ImportService{
		definitions:   deps.Definitions,
		installations: deps.Installations,
		artifacts:     deps.Artifacts,
		jobs:          deps.Jobs,
		records:       deps.Records,
		parser:        parser,
		mirror:        mirror,
	}
}

func (s *ImportService) ImportFromGitHub(ctx context.Context, userID string, input ImportGitHubInput) (ImportResult, error) {
	job := newSkillImportJob(userID, domain.SkillSourceGithub, input)
	if err := s.jobs.CreateSkillImportJob(ctx, job); err != nil {
		return ImportResult{}, mapImportError(err)
	}
	parsed, err := s.parser.ImportGitHubArchive(ctx, parserpkg.GitHubImportRequest{
		RepoURL: input.RepoURL,
		Ref:     input.Ref,
		Path:    input.Path,
	})
	if err != nil {
		s.failSkillImportJob(ctx, &job, err)
		return ImportResult{}, mapImportError(err)
	}
	return s.materializeImportedSkill(ctx, userID, job, parsed, input.Install, input.InstallationName, input.IsDefault, input.Enabled)
}

func (s *ImportService) ImportFromUpload(ctx context.Context, userID string, input ImportUploadInput) (ImportResult, error) {
	requestPayload := map[string]any{
		"fileName":         input.FileName,
		"path":             input.Path,
		"install":          input.Install,
		"installationName": input.InstallationName,
	}
	if input.IsDefault != nil {
		requestPayload["isDefault"] = *input.IsDefault
	}
	if input.Enabled != nil {
		requestPayload["enabled"] = *input.Enabled
	}

	job := newSkillImportJob(userID, domain.SkillSourceUpload, requestPayload)
	if err := s.jobs.CreateSkillImportJob(ctx, job); err != nil {
		return ImportResult{}, mapImportError(err)
	}
	parsed, err := s.parser.ImportUploadedArchive(input.FileName, input.MediaType, input.Path, input.ArchiveBytes)
	if err != nil {
		s.failSkillImportJob(ctx, &job, err)
		return ImportResult{}, mapImportError(err)
	}
	return s.materializeImportedSkill(ctx, userID, job, parsed, &input.Install, input.InstallationName, input.IsDefault, input.Enabled)
}
