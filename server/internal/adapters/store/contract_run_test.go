package store

import (
	"context"
	"testing"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func testRunContract(t *testing.T, s contractStore) {
	t.Helper()

	ctx := context.Background()
	userID := "user-contract-run"
	runPrimary := domain.Run{
		ID:                     "run-primary",
		UserID:                 userID,
		Title:                  "Primary Run",
		Goal:                   "Investigate alpha",
		RequestedMode:          domain.RunModeHybrid,
		KnowledgeConnectionIDs: []string{"conn-a", "conn-b"},
		SkillInstallationID:    "install-1",
		SkillSnapshotID:        "snapshot-1",
		EffectiveMode:          domain.RunModeKBOnly,
		Status:                 domain.RunStatusRunning,
		LatestArtifactID:       "artifact-1",
		ErrorMessage:           "",
		CreatedAt:              contractTime(90),
		UpdatedAt:              contractTime(92),
	}
	runSecondary := domain.Run{
		ID:                     "run-secondary",
		UserID:                 userID,
		Title:                  "Secondary Run",
		Goal:                   "Investigate beta",
		RequestedMode:          domain.RunModeAuto,
		KnowledgeConnectionIDs: nil,
		EffectiveMode:          domain.RunModeWeb,
		Status:                 domain.RunStatusCompleted,
		LatestArtifactID:       "",
		ErrorMessage:           "",
		CreatedAt:              contractTime(89),
		UpdatedAt:              contractTime(91),
	}
	if err := s.CreateRun(ctx, runPrimary); err != nil {
		t.Fatalf("CreateRun primary failed: %v", err)
	}
	if err := s.CreateRun(ctx, runSecondary); err != nil {
		t.Fatalf("CreateRun secondary failed: %v", err)
	}

	gotRun, err := s.GetRun(ctx, userID, runPrimary.ID)
	if err != nil {
		t.Fatalf("GetRun failed: %v", err)
	}
	assertDeepEqual(t, "run by user/id", gotRun, runPrimary)

	gotRunByID, err := s.GetRunByID(ctx, runPrimary.ID)
	if err != nil {
		t.Fatalf("GetRunByID failed: %v", err)
	}
	assertDeepEqual(t, "run by id", gotRunByID, runPrimary)

	runPrimary.Status = domain.RunStatusCompleted
	runPrimary.ErrorMessage = "completed without error"
	runPrimary.UpdatedAt = contractTime(93)
	if err := s.UpdateRun(ctx, runPrimary); err != nil {
		t.Fatalf("UpdateRun failed: %v", err)
	}
	gotUpdatedRun, err := s.GetRun(ctx, userID, runPrimary.ID)
	if err != nil {
		t.Fatalf("GetRun after update failed: %v", err)
	}
	assertDeepEqual(t, "updated run", gotUpdatedRun, runPrimary)

	listedRuns, err := s.ListRuns(ctx, userID)
	if err != nil {
		t.Fatalf("ListRuns failed: %v", err)
	}
	assertDeepEqual(t, "run list order", listedRuns, []domain.Run{runPrimary, runSecondary})

	stepPlanning := domain.RunStep{
		ID:        "step-planning",
		RunID:     runPrimary.ID,
		Kind:      domain.StepKindPlanning,
		Label:     "Planning",
		Status:    domain.StepStatusCompleted,
		Summary:   "planned",
		CreatedAt: contractTime(94),
	}
	stepResearch := domain.RunStep{
		ID:        "step-research",
		RunID:     runPrimary.ID,
		Kind:      domain.StepKindWebSearch,
		Label:     "Research",
		Status:    domain.StepStatusRunning,
		Summary:   "researching",
		CreatedAt: contractTime(95),
	}
	if err := s.UpsertRunStep(ctx, stepResearch); err != nil {
		t.Fatalf("UpsertRunStep research failed: %v", err)
	}
	if err := s.UpsertRunStep(ctx, stepPlanning); err != nil {
		t.Fatalf("UpsertRunStep planning failed: %v", err)
	}
	listedSteps, err := s.ListRunSteps(ctx, runPrimary.ID)
	if err != nil {
		t.Fatalf("ListRunSteps failed: %v", err)
	}
	assertDeepEqual(t, "run steps list order", listedSteps, []domain.RunStep{stepPlanning, stepResearch})

	artifactDraft := domain.RunArtifact{
		ID:              "artifact-draft",
		RunID:           runPrimary.ID,
		Kind:            domain.ArtifactKindReportDraft,
		ContentMarkdown: "# Draft",
		Version:         1,
		CreatedAt:       contractTime(96),
	}
	artifactFinal := domain.RunArtifact{
		ID:              "artifact-final",
		RunID:           runPrimary.ID,
		Kind:            domain.ArtifactKindReport,
		ContentMarkdown: "# Final",
		Version:         2,
		CreatedAt:       contractTime(97),
	}
	if err := s.SaveArtifact(ctx, artifactFinal); err != nil {
		t.Fatalf("SaveArtifact final failed: %v", err)
	}
	if err := s.SaveArtifact(ctx, artifactDraft); err != nil {
		t.Fatalf("SaveArtifact draft failed: %v", err)
	}
	listedArtifacts, err := s.ListArtifacts(ctx, runPrimary.ID)
	if err != nil {
		t.Fatalf("ListArtifacts failed: %v", err)
	}
	assertDeepEqual(t, "run artifacts list order", listedArtifacts, []domain.RunArtifact{artifactDraft, artifactFinal})

	sources := []domain.RunSource{
		{
			ID:           "run-source-a",
			RunID:        runPrimary.ID,
			Provider:     domain.ProviderYuque,
			ConnectionID: "conn-a",
			DocumentID:   "doc-a",
			ChunkID:      "chunk-a",
			Title:        "Run Source A",
			Repo:         "repo-a",
			URL:          "https://example.com/run-a",
			Snippet:      "snippet a",
			Score:        0.91,
			CreatedAt:    contractTime(98),
		},
		{
			ID:        "run-source-b",
			RunID:     runPrimary.ID,
			Provider:  domain.ProviderWeb,
			Title:     "Run Source B",
			URL:       "https://example.com/run-b",
			Snippet:   "snippet b",
			Score:     0.77,
			CreatedAt: contractTime(99),
		},
	}
	if err := s.SaveSources(ctx, runPrimary.ID, sources); err != nil {
		t.Fatalf("SaveSources failed: %v", err)
	}
	listedSources, err := s.ListSources(ctx, runPrimary.ID)
	if err != nil {
		t.Fatalf("ListSources failed: %v", err)
	}
	assertDeepEqual(t, "run sources", listedSources, sources)
}
