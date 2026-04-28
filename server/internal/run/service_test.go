package run

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/store"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/tools"
)

func TestCreateRunResolvesDefaultSkillInstallationByDefinition(t *testing.T) {
	mem := store.NewMemoryStore()
	service := NewService(mem, nil, NewEventBroker(), NewPlanner(), nil, nil, nil, nil, nil)
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	now := time.Now().UTC()
	skill := domain.Skill{
		ID:           "install-1",
		UserID:       user.ID,
		DefinitionID: "def-1",
		RevisionID:   "rev-1",
		Version:      1,
		Slug:         "summary-skill",
		Kind:         domain.SkillKindChatProfile,
		Title:        "Summary Skill",
		Description:  "summarize things",
		Prompt:       "summarize",
		Mode:         "summary",
		Source:       domain.SkillSourceManual,
		Enabled:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := mem.CreateSkill(context.Background(), skill); err != nil {
		t.Fatalf("create skill: %v", err)
	}

	secondInstallation := domain.SkillInstallation{
		ID:                "install-2",
		UserID:            user.ID,
		DefinitionID:      "def-1",
		CurrentRevisionID: "rev-1",
		Name:              "staging",
		IsDefault:         false,
		Enabled:           true,
		CreatedAt:         now.Add(time.Minute),
		UpdatedAt:         now.Add(time.Minute),
	}
	if err := mem.CreateSkillInstallation(context.Background(), secondInstallation); err != nil {
		t.Fatalf("create second installation: %v", err)
	}

	created, err := service.CreateRun(context.Background(), user.ID, CreateRunInput{
		Goal:              "Summarize internal docs",
		SkillDefinitionID: "def-1",
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	if created.SkillInstallationID != "install-1" {
		t.Fatalf("expected default installation install-1, got %s", created.SkillInstallationID)
	}
}

func TestExecuteRunPassesSkillSnapshotToReportWriter(t *testing.T) {
	mem := store.NewMemoryStore()
	writer := &capturingReportWriter{
		output: tools.ReportOutput{
			Summary:           "summary",
			OutlineMarkdown:   "# Outline",
			DraftMarkdown:     "# Draft",
			RetrievalMarkdown: "# Grounding Trace",
			ReportMarkdown:    "# report",
		},
	}
	service := NewService(mem, nil, NewEventBroker(), NewPlanner(), nil, nil, nil, nil, writer)

	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	now := time.Now().UTC()
	skill := domain.Skill{
		ID:            "install-1",
		UserID:        user.ID,
		DefinitionID:  "def-1",
		RevisionID:    "rev-1",
		Version:       1,
		Slug:          "summary-skill",
		Kind:          domain.SkillKindChatProfile,
		Title:         "Summary Skill",
		Description:   "summarize things",
		Prompt:        "summarize in terse bullets",
		Mode:          "summary",
		PlannerPolicy: map[string]string{"force_mode": string(domain.RunModeHybrid)},
		ToolAllowlist: []string{"report.write"},
		Source:        domain.SkillSourceManual,
		Enabled:       true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := mem.CreateSkill(context.Background(), skill); err != nil {
		t.Fatalf("create skill: %v", err)
	}

	run, err := service.CreateRun(context.Background(), user.ID, CreateRunInput{
		Goal:                "Summarize internal docs",
		SkillInstallationID: "install-1",
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}

	if err := service.ExecuteRun(context.Background(), run.ID); err != nil {
		t.Fatalf("execute run: %v", err)
	}

	if writer.snapshot == nil {
		t.Fatalf("expected report writer to receive skill snapshot")
	}
	if writer.snapshot.Scope != domain.SkillRuntimeScopeRun {
		t.Fatalf("expected run scope, got %s", writer.snapshot.Scope)
	}
	if writer.snapshot.InstallationID != "install-1" {
		t.Fatalf("expected installation install-1, got %s", writer.snapshot.InstallationID)
	}
	if writer.snapshot.DefinitionID != "def-1" {
		t.Fatalf("expected definition def-1, got %s", writer.snapshot.DefinitionID)
	}
	if writer.snapshot.Prompt != "summarize in terse bullets" {
		t.Fatalf("expected prompt to round-trip, got %s", writer.snapshot.Prompt)
	}

	storedRun, err := mem.GetRun(context.Background(), user.ID, run.ID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if storedRun.SkillSnapshotID == "" {
		t.Fatalf("expected run to persist skill snapshot id")
	}
	if storedRun.SkillSnapshotID != writer.snapshot.ID {
		t.Fatalf("expected run snapshot id %s, got %s", writer.snapshot.ID, storedRun.SkillSnapshotID)
	}
	if storedRun.Status != domain.RunStatusCompleted {
		t.Fatalf("expected completed run, got %s", storedRun.Status)
	}

	artifacts, err := mem.ListArtifacts(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("list artifacts: %v", err)
	}
	if len(artifacts) != 5 {
		t.Fatalf("expected 5 run artifacts, got %d", len(artifacts))
	}
	if artifacts[0].Kind != domain.ArtifactKindReportOutline {
		t.Fatalf("expected first artifact to be report outline, got %s", artifacts[0].Kind)
	}
	if artifacts[1].Kind != domain.ArtifactKindReportDraft {
		t.Fatalf("expected second artifact to be report draft, got %s", artifacts[1].Kind)
	}
	if artifacts[2].Kind != domain.ArtifactKindReportGrounding {
		t.Fatalf("expected third artifact to be report grounding, got %s", artifacts[2].Kind)
	}
	if artifacts[3].Kind != domain.ArtifactKindReport {
		t.Fatalf("expected fourth artifact to be report, got %s", artifacts[3].Kind)
	}
	if artifacts[4].Kind != domain.ArtifactKindFinalAnswer {
		t.Fatalf("expected fifth artifact to be final answer, got %s", artifacts[4].Kind)
	}
}

func TestExecuteRunUsesEvidenceMerger(t *testing.T) {
	mem := store.NewMemoryStore()
	writer := &capturingReportWriter{
		output: tools.ReportOutput{
			Summary:           "summary",
			OutlineMarkdown:   "# Outline",
			DraftMarkdown:     "# Draft",
			RetrievalMarkdown: "# Grounding Trace",
			ReportMarkdown:    "# report",
		},
	}
	knowledgeTool := &stubKnowledgeSearcher{
		items: []domain.Evidence{
			{Provider: domain.ProviderYuque, ConnectionID: "conn-1", Title: "发布说明", Snippet: "旧内容", Score: 0.5},
			{Provider: domain.ProviderYuque, ConnectionID: "conn-1", Title: "发布说明", Snippet: "新内容", Score: 0.9},
		},
	}
	merger := &capturingEvidenceMerger{
		result: tools.EvidenceMergeResult{
			Evidences: []domain.Evidence{
				{Provider: domain.ProviderYuque, ConnectionID: "conn-1", Title: "发布说明", Snippet: "新内容", Score: 1.2},
			},
			DuplicateCount: 1,
			GroupCount:     1,
			ConflictCount:  0,
			GroupLabels:    []string{"发布说明"},
		},
	}
	service := NewService(mem, nil, NewEventBroker(), NewPlanner(), knowledgeTool, nil, nil, merger, writer)

	user := domain.User{ID: "user-1", Username: "tester", CreatedAt: time.Now().UTC()}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	run, err := service.CreateRun(context.Background(), user.ID, CreateRunInput{
		Goal:                   "结合语雀知识库总结团队内部规范",
		Mode:                   domain.RunModeKBOnly,
		KnowledgeConnectionIDs: []string{"conn-1"},
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}

	if err := service.ExecuteRun(context.Background(), run.ID); err != nil {
		t.Fatalf("execute run: %v", err)
	}
	if !merger.called {
		t.Fatalf("expected evidence merger to be called")
	}
	if len(writer.evidences) != 1 || writer.evidences[0].Score != 1.2 {
		t.Fatalf("expected report writer to receive merged evidences, got %+v", writer.evidences)
	}
	if writer.diagnostics.DuplicateCount != 1 || writer.diagnostics.GroupCount != 1 {
		t.Fatalf("expected report writer to receive merge diagnostics, got %+v", writer.diagnostics)
	}
	steps, err := mem.ListRunSteps(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("list steps: %v", err)
	}
	foundMergeSummary := false
	for _, step := range steps {
		if step.Kind == domain.StepKindEvidenceMerge {
			foundMergeSummary = true
			if !strings.Contains(step.Summary, "removed 1 duplicates") {
				t.Fatalf("expected merge summary to include diagnostics, got %q", step.Summary)
			}
		}
	}
	if !foundMergeSummary {
		t.Fatalf("expected evidence merge step")
	}
	artifacts, err := mem.ListArtifacts(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("list artifacts: %v", err)
	}
	if len(artifacts) != 5 {
		t.Fatalf("expected outline/draft/grounding/report/final artifacts, got %d", len(artifacts))
	}
}

type capturingReportWriter struct {
	snapshot    *domain.SkillRuntimeSnapshot
	evidences   []domain.Evidence
	diagnostics tools.ReportEvidenceDiagnostics
	output      tools.ReportOutput
}

func (w *capturingReportWriter) Write(
	_ context.Context,
	_ string,
	_ string,
	_ domain.RunMode,
	_ []string,
	evidences []domain.Evidence,
	diagnostics tools.ReportEvidenceDiagnostics,
	snapshot *domain.SkillRuntimeSnapshot,
	_ *tools.TraceContext,
) (tools.ReportOutput, error) {
	w.snapshot = snapshot
	w.evidences = append([]domain.Evidence(nil), evidences...)
	w.diagnostics = diagnostics
	if w.output.ReportMarkdown == "" {
		w.output = tools.ReportOutput{
			Summary:        "summary",
			ReportMarkdown: "# report",
		}
	}
	return w.output, nil
}

type stubKnowledgeSearcher struct {
	items []domain.Evidence
}

func (s *stubKnowledgeSearcher) Search(_ context.Context, _ string, _ []string, _ string, _ int, _ *tools.TraceContext) ([]domain.Evidence, error) {
	return append([]domain.Evidence(nil), s.items...), nil
}

type capturingEvidenceMerger struct {
	called bool
	result tools.EvidenceMergeResult
}

func (m *capturingEvidenceMerger) Merge(
	_ context.Context,
	_ string,
	_ string,
	_ domain.RunMode,
	_ []domain.Evidence,
	_ *domain.SkillRuntimeSnapshot,
) (tools.EvidenceMergeResult, error) {
	m.called = true
	return tools.EvidenceMergeResult{
		Evidences:      append([]domain.Evidence(nil), m.result.Evidences...),
		DuplicateCount: m.result.DuplicateCount,
		GroupCount:     m.result.GroupCount,
		ConflictCount:  m.result.ConflictCount,
		GroupLabels:    append([]string(nil), m.result.GroupLabels...),
		Warnings:       append([]string(nil), m.result.Warnings...),
	}, nil
}
