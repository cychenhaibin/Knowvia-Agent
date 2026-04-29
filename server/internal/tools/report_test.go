package tools

import (
	"context"
	"testing"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/provider"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

type fakeReportForwardClient struct {
	lastRequest provider.ForwardedReportRequest
	result      provider.ForwardedReportResult
	err         error
}

func (f *fakeReportForwardClient) GenerateReport(
	_ context.Context,
	req provider.ForwardedReportRequest,
) (provider.ForwardedReportResult, error) {
	f.lastRequest = req
	return f.result, f.err
}

type fakeChatClient struct {
	answer string
	err    error
}

func (f *fakeChatClient) Complete(_ context.Context, _, _ string) (string, error) {
	return f.answer, f.err
}

func TestMarkdownReportWriterPrefersForwardedReport(t *testing.T) {
	forward := &fakeReportForwardClient{
		result: provider.ForwardedReportResult{
			Summary:           "summary from llm",
			OutlineMarkdown:   "# Outline\n\n- Executive Summary",
			DraftMarkdown:     "# Draft\n\ncontent draft",
			RetrievalMarkdown: "# Grounding Trace\n\n- Used in report grounding: `chunk-1`",
			ReportMarkdown:    "# Report\n\ncontent",
		},
	}
	writer := NewMarkdownReportWriter(&fakeChatClient{answer: "local fallback"}, forward)

	snapshot := &domain.SkillRuntimeSnapshot{
		ID:             "snap-1",
		UserID:         "user-1",
		InstallationID: "install-1",
		DefinitionID:   "def-1",
		RevisionID:     "rev-1",
		Kind:           domain.SkillKindChatProfile,
		Title:          "Skill",
		Prompt:         "be concise",
		RuntimeSpecJSON: `{
			"schemaVersion":"2026-04-21",
			"skillKind":"chat_profile",
			"responseMode":"summary",
			"instructions":"be concise"
		}`,
		CreatedAt: time.Now().UTC(),
	}

	output, err := writer.Write(
		context.Background(),
		"user-1",
		"Summarize the current situation",
		domain.RunModeHybrid,
		[]string{"conn-1", "conn-2"},
		[]domain.Evidence{
			{
				Provider:     domain.ProviderYuque,
				ConnectionID: "conn-1",
				DocumentID:   "doc-1",
				Title:        "发布说明",
				Repo:         "产品文档",
				URL:          "https://example.com/release-note",
				Snippet:      "新增 timeline 和 artifact 视图。",
				Body:         "Knowvia 在四月新增了 timeline 和 artifact 视图。",
				Score:        0.9,
			},
		},
		ReportEvidenceDiagnostics{
			DuplicateCount: 1,
			GroupCount:     1,
			ConflictCount:  1,
			GroupLabels:    []string{"发布说明"},
			Warnings:       []string{"Evidence group '发布说明' contains 2 non-identical variants."},
		},
		snapshot,
		&TraceContext{RunID: "run-1", SkillID: "skill-1"},
	)
	if err != nil {
		t.Fatalf("write report: %v", err)
	}
	if output.Summary != "summary from llm" {
		t.Fatalf("expected forwarded summary, got %q", output.Summary)
	}
	if output.OutlineMarkdown != "# Outline\n\n- Executive Summary" {
		t.Fatalf("expected forwarded outline, got %q", output.OutlineMarkdown)
	}
	if output.DraftMarkdown != "# Draft\n\ncontent draft" {
		t.Fatalf("expected forwarded draft, got %q", output.DraftMarkdown)
	}
	if output.RetrievalMarkdown != "# Grounding Trace\n\n- Used in report grounding: `chunk-1`" {
		t.Fatalf("expected forwarded retrieval markdown, got %q", output.RetrievalMarkdown)
	}
	if output.ReportMarkdown != "# Report\n\ncontent" {
		t.Fatalf("expected forwarded report, got %q", output.ReportMarkdown)
	}
	if forward.lastRequest.UserID != "user-1" {
		t.Fatalf("expected user id to be forwarded, got %q", forward.lastRequest.UserID)
	}
	if len(forward.lastRequest.ScopeIDs) != 2 || forward.lastRequest.ScopeIDs[0] != "conn-1" {
		t.Fatalf("expected connection ids to be forwarded as scope ids, got %#v", forward.lastRequest.ScopeIDs)
	}
	if len(forward.lastRequest.Evidences) != 1 || forward.lastRequest.Evidences[0].Body == "" {
		t.Fatalf("expected evidence body to be forwarded, got %#v", forward.lastRequest.Evidences)
	}
	if forward.lastRequest.EvidenceDiagnostics == nil || forward.lastRequest.EvidenceDiagnostics.ConflictCount != 1 {
		t.Fatalf("expected evidence diagnostics to be forwarded, got %#v", forward.lastRequest.EvidenceDiagnostics)
	}
	if forward.lastRequest.SkillSnapshot == nil || forward.lastRequest.SkillSnapshot.Prompt != "be concise" {
		t.Fatalf("expected skill snapshot to be forwarded, got %#v", forward.lastRequest.SkillSnapshot)
	}
	if forward.lastRequest.Trace == nil || forward.lastRequest.Trace.RunID != "run-1" {
		t.Fatalf("expected trace context to be forwarded, got %#v", forward.lastRequest.Trace)
	}
	if forward.lastRequest.ModelProfile == nil || forward.lastRequest.ModelProfile.Purpose != "report_writer" {
		t.Fatalf("expected report model profile hint, got %#v", forward.lastRequest.ModelProfile)
	}
}

func TestMarkdownReportWriterFallsBackToLocalClient(t *testing.T) {
	forward := &fakeReportForwardClient{err: context.DeadlineExceeded}
	writer := NewMarkdownReportWriter(&fakeChatClient{answer: "# 本地报告\n\n内容"}, forward)

	output, err := writer.Write(
		context.Background(),
		"user-1",
		"Summarize the current situation",
		domain.RunModeHybrid,
		[]string{"conn-1"},
		nil,
		ReportEvidenceDiagnostics{ConflictCount: 1, Warnings: []string{"同一主题存在多个版本。"}},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("write report: %v", err)
	}
	if output.Summary != "本地报告" {
		t.Fatalf("expected local summary, got %q", output.Summary)
	}
	if output.ReportMarkdown != "# 本地报告\n\n内容" {
		t.Fatalf("expected local report, got %q", output.ReportMarkdown)
	}
}
