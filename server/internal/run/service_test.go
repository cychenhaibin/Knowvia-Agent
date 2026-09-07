package run

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/store"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/chat"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/gitrepo"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/tools"
)

func TestCreateRunResolvesDefaultSkillInstallationByDefinition(t *testing.T) {
	mem := store.NewMemoryStore()
	service := NewService(ServiceDeps{
		Runs:      mem,
		Steps:     mem,
		Artifacts: mem,
		Sources:   mem,
		Skills:    mem,
		Selection: mem,
	}, nil, NewEventBroker(), NewPlanner(), nil, nil, nil, nil, nil)
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

func TestCreateRunWithGitHubURLCreatesTaskSessionAndPrompt(t *testing.T) {
	mem := store.NewMemoryStore()
	service := NewService(ServiceDeps{
		Runs:         mem,
		Steps:        mem,
		Artifacts:    mem,
		Sources:      mem,
		Skills:       mem,
		Selection:    mem,
		TaskSessions: mem,
	}, nil, NewEventBroker(), NewPlanner(), nil, nil, nil, nil, nil)

	user := domain.User{ID: "user-1", Username: "tester", CreatedAt: time.Now().UTC()}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	created, err := service.CreateRun(context.Background(), user.ID, CreateRunInput{
		Goal: "https://github.com/bytedance/trae-agent 分析并理解这个项目仓库，生成结构化的完整的Code Wiki文档(md文件)",
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}

	if created.Kind != domain.RunKindGitHubRepoAnalysis {
		t.Fatalf("expected github repo analysis run, got %s", created.Kind)
	}
	if created.SourceURL != "https://github.com/bytedance/trae-agent" {
		t.Fatalf("expected normalized source url, got %s", created.SourceURL)
	}
	if created.TaskSessionID == "" {
		t.Fatalf("expected task session id")
	}
	if !strings.Contains(created.TaskPrompt, "https://github.com/bytedance/trae-agent") {
		t.Fatalf("expected task prompt to include repo URL, got %q", created.TaskPrompt)
	}
	if !strings.Contains(created.TaskPrompt, "Code Wiki") {
		t.Fatalf("expected task prompt to include Code Wiki instructions, got %q", created.TaskPrompt)
	}

	session, err := mem.GetChatSession(context.Background(), user.ID, created.TaskSessionID)
	if err != nil {
		t.Fatalf("get task session: %v", err)
	}
	if session.Kind != domain.ChatSessionKindTask {
		t.Fatalf("expected task session kind, got %s", session.Kind)
	}
	if session.RunID != created.ID {
		t.Fatalf("expected task session run id %s, got %s", created.ID, session.RunID)
	}

	recentSessions, err := mem.ListChatSessions(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("list chat sessions: %v", err)
	}
	if len(recentSessions) != 0 {
		t.Fatalf("expected task session to be hidden from recent sessions, got %#v", recentSessions)
	}
}

func TestRunTaskConversationGeneratesCodeWikiArtifact(t *testing.T) {
	mem := store.NewMemoryStore()
	analyzer := &fakeGitHubAnalyzer{
		result: gitrepo.AnalyzeResult{
			Scan: gitrepo.WorkspaceScan{
				RepoURL:   "https://github.com/bytedance/trae-agent",
				Languages: []string{"Python"},
				KeyFiles: []gitrepo.FileSummary{
					{Path: "README.md", Language: "Markdown", Excerpt: "# Trae Agent"},
				},
			},
			Markdown: "# Code Wiki\n\n## 项目整体架构\n\nTrae Agent architecture.",
		},
	}
	service := NewService(ServiceDeps{
		Runs:         mem,
		Steps:        mem,
		Artifacts:    mem,
		Sources:      mem,
		Skills:       mem,
		Selection:    mem,
		TaskSessions: mem,
	}, nil, NewEventBroker(), NewPlanner(), nil, nil, nil, nil, nil)
	service.SetGitHubAnalyzer(analyzer)

	user := domain.User{ID: "user-1", Username: "tester", CreatedAt: time.Now().UTC()}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}
	run, err := service.CreateRun(context.Background(), user.ID, CreateRunInput{
		Goal: "https://github.com/bytedance/trae-agent 生成 Code Wiki",
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}

	chunks := []string{}
	result, err := service.RunTaskConversation(
		context.Background(),
		chat.TaskConversationRequest{
			UserID:    user.ID,
			RunID:     run.ID,
			SessionID: run.TaskSessionID,
			Message:   run.TaskPrompt,
		},
		func(chunk string) error {
			chunks = append(chunks, chunk)
			return nil
		},
	)
	if err != nil {
		t.Fatalf("run task conversation: %v", err)
	}
	if !result.Handled {
		t.Fatalf("expected task conversation to be handled")
	}
	if !strings.Contains(result.Answer, "# Code Wiki") {
		t.Fatalf("expected code wiki answer, got %q", result.Answer)
	}
	if strings.Join(chunks, "") != result.Answer {
		t.Fatalf("expected chunks to stream answer, got %#v", chunks)
	}
	if analyzer.request.RepoURL != "https://github.com/bytedance/trae-agent" {
		t.Fatalf("expected analyzer repo URL, got %s", analyzer.request.RepoURL)
	}

	storedRun, err := mem.GetRun(context.Background(), user.ID, run.ID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if storedRun.Status != domain.RunStatusCompleted {
		t.Fatalf("expected completed run, got %s", storedRun.Status)
	}
	if storedRun.LatestArtifactID == "" {
		t.Fatalf("expected latest artifact id")
	}
	artifacts, err := mem.ListArtifacts(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("list artifacts: %v", err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("expected one code wiki artifact, got %d", len(artifacts))
	}
	if artifacts[0].Kind != domain.ArtifactKindCodeWiki {
		t.Fatalf("expected code wiki artifact, got %s", artifacts[0].Kind)
	}
	if artifacts[0].ContentMarkdown != result.Answer {
		t.Fatalf("expected artifact content to match answer")
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
	service := NewService(ServiceDeps{
		Runs:      mem,
		Steps:     mem,
		Artifacts: mem,
		Sources:   mem,
		Skills:    mem,
		Selection: mem,
	}, nil, NewEventBroker(), NewPlanner(), nil, nil, nil, nil, writer)

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
	service := NewService(ServiceDeps{
		Runs:      mem,
		Steps:     mem,
		Artifacts: mem,
		Sources:   mem,
		Skills:    mem,
		Selection: mem,
	}, nil, NewEventBroker(), NewPlanner(), knowledgeTool, nil, nil, merger, writer)

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

type fakeGitHubAnalyzer struct {
	request gitrepo.AnalyzeRequest
	result  gitrepo.AnalyzeResult
}

func (a *fakeGitHubAnalyzer) Analyze(_ context.Context, req gitrepo.AnalyzeRequest) (gitrepo.AnalyzeResult, error) {
	a.request = req
	return a.result, nil
}
