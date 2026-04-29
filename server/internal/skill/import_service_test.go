package skill

import (
	"context"
	"errors"
	"testing"

	memorybackend "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/store/memory"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	parserpkg "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skillimport"
)

type fakeImportParser struct {
	parsed parserpkg.ParsedPackage
	err    error
}

func (f fakeImportParser) ImportGitHubArchive(context.Context, parserpkg.GitHubImportRequest) (parserpkg.ParsedPackage, error) {
	return f.parsed, f.err
}

func (f fakeImportParser) ImportUploadedArchive(string, string, string, []byte) (parserpkg.ParsedPackage, error) {
	return f.parsed, f.err
}

type recordingMirrorSyncer struct {
	synced []domain.Skill
	err    error
}

func (m *recordingMirrorSyncer) SyncSkill(_ context.Context, skill domain.Skill) error {
	m.synced = append(m.synced, skill)
	return m.err
}

func (m *recordingMirrorSyncer) DeleteSkill(context.Context, string, string) error {
	return nil
}

func TestImportFromUploadMarksJobFailedWhenParserFails(t *testing.T) {
	t.Parallel()

	store := memorybackend.New()
	parser := fakeImportParser{err: parserpkg.ErrInvalidArchive}
	service := NewImportService(ImportDeps{
		Definitions:   store,
		Installations: store,
		Artifacts:     store,
		Jobs:          store,
		Records:       store,
	}, parser, nil)

	_, err := service.ImportFromUpload(context.Background(), "import-user", ImportUploadInput{
		FileName:     "skill.zip",
		MediaType:    "application/zip",
		ArchiveBytes: []byte("archive"),
		Install:      true,
	})
	if !errors.Is(err, parserpkg.ErrInvalidArchive) {
		t.Fatalf("ImportFromUpload error = %v, want %v", err, parserpkg.ErrInvalidArchive)
	}

	jobs, err := store.ListSkillImportJobs(context.Background(), "import-user")
	if err != nil {
		t.Fatalf("ListSkillImportJobs failed: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 import job, got %d", len(jobs))
	}
	if jobs[0].Status != domain.SkillImportFailed {
		t.Fatalf("job status = %q, want failed", jobs[0].Status)
	}
	if jobs[0].ErrorMessage == "" {
		t.Fatalf("expected failed job to keep error message")
	}
}

func TestImportFromUploadCreatesCompletedInstallationAndArtifact(t *testing.T) {
	t.Parallel()

	store := memorybackend.New()
	mirror := &recordingMirrorSyncer{}
	service := NewImportService(ImportDeps{
		Definitions:   store,
		Installations: store,
		Artifacts:     store,
		Jobs:          store,
		Records:       store,
	}, fakeImportParser{
		parsed: parserpkg.ParsedPackage{
			Source:           domain.SkillSourceUpload,
			FileName:         "skill.zip",
			MediaType:        "application/zip",
			EntryPath:        "SKILL.md",
			InstructionsPath: "SKILL.md",
			ArchiveBytes:     []byte("zip"),
			SHA256:           "abc123",
			SizeBytes:        3,
			Slug:             "release-helper",
			Kind:             domain.SkillKindChatProfile,
			Title:            "Release Helper",
			Description:      "Coordinates release steps",
			Prompt:           "Follow the release playbook",
			Mode:             "answer",
			Files: []parserpkg.ParsedPackageFile{
				{
					Path:           "SKILL.md",
					MediaType:      "text/markdown",
					SizeBytes:      3,
					SHA256:         "abc123",
					IsInstructions: true,
				},
			},
		},
	}, mirror)

	result, err := service.ImportFromUpload(context.Background(), "import-success-user", ImportUploadInput{
		FileName:         "skill.zip",
		MediaType:        "application/zip",
		ArchiveBytes:     []byte("zip"),
		Install:          true,
		InstallationName: "Release Helper",
	})
	if err != nil {
		t.Fatalf("ImportFromUpload failed: %v", err)
	}
	if result.Job.Status != domain.SkillImportCompleted {
		t.Fatalf("job status = %q, want completed", result.Job.Status)
	}
	if result.Job.InstallationID == "" || result.Artifact == nil {
		t.Fatalf("expected completed import to create installation and artifact")
	}
	if len(mirror.synced) != 1 {
		t.Fatalf("expected mirror sync once, got %d", len(mirror.synced))
	}

	artifact, err := store.GetSkillArtifact(context.Background(), "import-success-user", result.Artifact.ID)
	if err != nil {
		t.Fatalf("GetSkillArtifact failed: %v", err)
	}
	if artifact.DefinitionID == "" || artifact.RevisionID == "" {
		t.Fatalf("artifact missing definition or revision linkage: %#v", artifact)
	}

	files, err := store.ListSkillArtifactFiles(context.Background(), "import-success-user", result.Artifact.ID)
	if err != nil {
		t.Fatalf("ListSkillArtifactFiles failed: %v", err)
	}
	if len(files) != 1 || files[0].Path != "SKILL.md" {
		t.Fatalf("unexpected artifact files: %#v", files)
	}
}
