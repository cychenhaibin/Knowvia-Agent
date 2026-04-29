package store

import (
	"context"
	"testing"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func testSkillContract(t *testing.T, s contractStore) {
	t.Helper()

	ctx := context.Background()
	user := domain.User{
		ID:           "user-contract-skill",
		Username:     "contract-skill-user",
		DisplayName:  "Contract Skill User",
		Email:        "skill@example.com",
		PasswordHash: "hashed-skill",
		CreatedAt:    contractTime(110),
	}
	if err := s.UpsertUser(ctx, user); err != nil {
		t.Fatalf("UpsertUser for skill contract failed: %v", err)
	}

	definition := domain.SkillDefinition{
		ID:        "skill-def-1",
		UserID:    user.ID,
		Slug:      "research-helper",
		Kind:      domain.SkillKindChatProfile,
		Source:    domain.SkillSourceManual,
		RepoURL:   "",
		CreatedAt: contractTime(111),
		UpdatedAt: contractTime(112),
	}
	revisionOne := domain.SkillRevision{
		ID:           "skill-rev-1",
		DefinitionID: definition.ID,
		Version:      1,
		Title:        "Research Helper",
		Description:  "First revision",
		Prompt:       "Help with research",
		Mode:         "answer",
		ManifestJSON: "{}",
		CreatedAt:    contractTime(113),
	}
	if err := s.CreateSkillDefinitionRevision(ctx, definition, revisionOne); err != nil {
		t.Fatalf("CreateSkillDefinitionRevision failed: %v", err)
	}

	installation := domain.SkillInstallation{
		ID:                "skill-install-1",
		UserID:            user.ID,
		DefinitionID:      definition.ID,
		CurrentRevisionID: revisionOne.ID,
		Name:              "Research Helper Default",
		IsDefault:         true,
		Enabled:           true,
		CreatedAt:         contractTime(114),
		UpdatedAt:         contractTime(115),
	}
	if err := s.CreateSkillInstallation(ctx, installation); err != nil {
		t.Fatalf("CreateSkillInstallation failed: %v", err)
	}

	listedDefinitions, err := s.ListSkillDefinitions(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListSkillDefinitions failed: %v", err)
	}
	assertDeepEqual(t, "skill definitions", listedDefinitions, []domain.SkillDefinition{definition})

	listedRevisions, err := s.ListSkillRevisions(ctx, user.ID, definition.ID)
	if err != nil {
		t.Fatalf("ListSkillRevisions failed: %v", err)
	}
	assertDeepEqual(t, "skill revisions", listedRevisions, []domain.SkillRevision{revisionOne})

	gotInstallationRecord, err := s.GetSkillInstallationRecord(ctx, user.ID, installation.ID)
	if err != nil {
		t.Fatalf("GetSkillInstallationRecord failed: %v", err)
	}
	assertDeepEqual(t, "skill installation record", gotInstallationRecord, domain.SkillInstallationRecord{
		Installation:    installation,
		Definition:      definition,
		CurrentRevision: revisionOne,
	})

	gotDefaultInstallationRecord, err := s.GetDefaultSkillInstallationRecord(ctx, user.ID, definition.ID)
	if err != nil {
		t.Fatalf("GetDefaultSkillInstallationRecord failed: %v", err)
	}
	assertDeepEqual(t, "default skill installation record", gotDefaultInstallationRecord, domain.SkillInstallationRecord{
		Installation:    installation,
		Definition:      definition,
		CurrentRevision: revisionOne,
	})

	listedInstallations, err := s.ListSkillInstallations(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListSkillInstallations failed: %v", err)
	}
	assertDeepEqual(t, "skill installations", listedInstallations, []domain.SkillInstallationRecord{
		{
			Installation:    installation,
			Definition:      definition,
			CurrentRevision: revisionOne,
		},
	})

	gotSkill, err := s.GetSkill(ctx, user.ID, installation.ID)
	if err != nil {
		t.Fatalf("GetSkill failed: %v", err)
	}
	assertDeepEqual(t, "materialized skill", gotSkill, domain.Skill{
		ID:           installation.ID,
		UserID:       user.ID,
		DefinitionID: definition.ID,
		RevisionID:   revisionOne.ID,
		Version:      revisionOne.Version,
		Slug:         definition.Slug,
		Kind:         definition.Kind,
		Title:        revisionOne.Title,
		Description:  revisionOne.Description,
		Prompt:       revisionOne.Prompt,
		Mode:         revisionOne.Mode,
		Source:       definition.Source,
		Enabled:      installation.Enabled,
		RepoURL:      definition.RepoURL,
		CreatedAt:    installation.CreatedAt,
		UpdatedAt:    installation.UpdatedAt,
	})

	listedSkills, err := s.ListSkills(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListSkills failed: %v", err)
	}
	assertDeepEqual(t, "listed skills", listedSkills, []domain.Skill{
		{
			ID:           installation.ID,
			UserID:       user.ID,
			DefinitionID: definition.ID,
			RevisionID:   revisionOne.ID,
			Version:      revisionOne.Version,
			Slug:         definition.Slug,
			Kind:         definition.Kind,
			Title:        revisionOne.Title,
			Description:  revisionOne.Description,
			Prompt:       revisionOne.Prompt,
			Mode:         revisionOne.Mode,
			Source:       definition.Source,
			Enabled:      installation.Enabled,
			RepoURL:      definition.RepoURL,
			CreatedAt:    installation.CreatedAt,
			UpdatedAt:    installation.UpdatedAt,
		},
	})

	details, err := s.GetSkillDefinitionDetails(ctx, user.ID, definition.ID)
	if err != nil {
		t.Fatalf("GetSkillDefinitionDetails failed: %v", err)
	}
	assertDeepEqual(t, "skill definition details", details, domain.SkillDefinitionDetails{
		Definition:    definition,
		Revisions:     []domain.SkillRevision{revisionOne},
		Installations: []domain.SkillInstallation{installation},
	})

	artifact := domain.SkillArtifact{
		ID:               "artifact-1",
		UserID:           user.ID,
		DefinitionID:     definition.ID,
		RevisionID:       revisionOne.ID,
		Source:           domain.SkillSourceUpload,
		FileName:         "skill.zip",
		MediaType:        "application/zip",
		SourceURL:        "https://example.com/skill.zip",
		SHA256:           "sha256-skill",
		SizeBytes:        1234,
		EntryPath:        "entry.txt",
		ManifestPath:     "skill.json",
		InstructionsPath: "README.md",
		ArchiveBytes:     []byte("archive-bytes"),
		CreatedAt:        contractTime(118),
	}
	if err := s.CreateSkillArtifact(ctx, artifact); err != nil {
		t.Fatalf("CreateSkillArtifact failed: %v", err)
	}
	gotArtifact, err := s.GetSkillArtifact(ctx, user.ID, artifact.ID)
	if err != nil {
		t.Fatalf("GetSkillArtifact failed: %v", err)
	}
	assertDeepEqual(t, "skill artifact", gotArtifact, artifact)

	artifactFiles := []domain.SkillArtifactFile{
		{
			ID:             "artifact-file-2",
			ArtifactID:     artifact.ID,
			UserID:         user.ID,
			Path:           "README.md",
			MediaType:      "text/markdown",
			SizeBytes:      10,
			SHA256:         "sha-readme",
			IsManifest:     false,
			IsInstructions: true,
			CreatedAt:      contractTime(120),
		},
		{
			ID:             "artifact-file-1",
			ArtifactID:     artifact.ID,
			UserID:         user.ID,
			Path:           "skill.json",
			MediaType:      "application/json",
			SizeBytes:      20,
			SHA256:         "sha-manifest",
			IsManifest:     true,
			IsInstructions: false,
			CreatedAt:      contractTime(119),
		},
	}
	if err := s.ReplaceSkillArtifactFiles(ctx, artifact.ID, artifactFiles); err != nil {
		t.Fatalf("ReplaceSkillArtifactFiles failed: %v", err)
	}
	gotArtifactFiles, err := s.ListSkillArtifactFiles(ctx, user.ID, artifact.ID)
	if err != nil {
		t.Fatalf("ListSkillArtifactFiles failed: %v", err)
	}
	assertDeepEqual(t, "skill artifact files", gotArtifactFiles, []domain.SkillArtifactFile{
		artifactFiles[0],
		artifactFiles[1],
	})

	importJobOld := domain.SkillImportJob{
		ID:          "import-job-old",
		UserID:      user.ID,
		Source:      domain.SkillSourceGithub,
		Status:      domain.SkillImportPending,
		RequestJSON: `{"repoUrl":"https://example.com/old"}`,
		CreatedAt:   contractTime(121),
		UpdatedAt:   contractTime(121),
	}
	importJobNew := domain.SkillImportJob{
		ID:          "import-job-new",
		UserID:      user.ID,
		Source:      domain.SkillSourceUpload,
		Status:      domain.SkillImportPending,
		RequestJSON: `{"fileName":"skill.zip"}`,
		CreatedAt:   contractTime(122),
		UpdatedAt:   contractTime(122),
	}
	if err := s.CreateSkillImportJob(ctx, importJobOld); err != nil {
		t.Fatalf("CreateSkillImportJob old failed: %v", err)
	}
	if err := s.CreateSkillImportJob(ctx, importJobNew); err != nil {
		t.Fatalf("CreateSkillImportJob new failed: %v", err)
	}
	completedAt := contractTime(124)
	importJobNew.Status = domain.SkillImportCompleted
	importJobNew.ArtifactID = artifact.ID
	importJobNew.DefinitionID = definition.ID
	importJobNew.RevisionID = revisionOne.ID
	importJobNew.InstallationID = installation.ID
	importJobNew.UpdatedAt = contractTime(123)
	importJobNew.CompletedAt = &completedAt
	if err := s.UpdateSkillImportJob(ctx, importJobNew); err != nil {
		t.Fatalf("UpdateSkillImportJob failed: %v", err)
	}
	gotImportJob, err := s.GetSkillImportJob(ctx, user.ID, importJobNew.ID)
	if err != nil {
		t.Fatalf("GetSkillImportJob failed: %v", err)
	}
	assertDeepEqual(t, "skill import job", gotImportJob, importJobNew)

	listedImportJobs, err := s.ListSkillImportJobs(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListSkillImportJobs failed: %v", err)
	}
	assertDeepEqual(t, "skill import jobs order", listedImportJobs, []domain.SkillImportJob{importJobNew, importJobOld})
}
