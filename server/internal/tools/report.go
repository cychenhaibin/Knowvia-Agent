package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/provider"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skillruntime"
)

type MarkdownReportWriter struct {
	client  provider.ChatClient
	forward provider.ReportForwardClient
}

func NewMarkdownReportWriter(client provider.ChatClient, forward provider.ReportForwardClient) *MarkdownReportWriter {
	return &MarkdownReportWriter{client: client, forward: forward}
}

func (w *MarkdownReportWriter) Write(
	ctx context.Context,
	userID string,
	goal string,
	mode domain.RunMode,
	connectionIDs []string,
	evidences []domain.Evidence,
	diagnostics ReportEvidenceDiagnostics,
	snapshot *domain.SkillRuntimeSnapshot,
	trace *TraceContext,
) (ReportOutput, error) {
	if w.forward != nil && strings.TrimSpace(userID) != "" {
		result, err := w.forward.GenerateReport(ctx, provider.ForwardedReportRequest{
			UserID:              strings.TrimSpace(userID),
			Goal:                strings.TrimSpace(goal),
			ScopeIDs:            append([]string(nil), connectionIDs...),
			Mode:                string(mode),
			SkillSnapshot:       forwardedSkillSnapshot(snapshot),
			Evidences:           forwardedEvidence(evidences),
			EvidenceDiagnostics: forwardedEvidenceDiagnostics(diagnostics),
			ModelProfile:        &provider.ForwardedModelProfile{Purpose: "report_writer"},
			Trace:               forwardedTraceContext(trace),
		})
		if err == nil && strings.TrimSpace(result.ReportMarkdown) != "" {
			summary := strings.TrimSpace(result.Summary)
			if summary == "" {
				summary = summarizeReport(result.ReportMarkdown)
			}
			return ReportOutput{
				Summary:           summary,
				OutlineMarkdown:   strings.TrimSpace(result.OutlineMarkdown),
				DraftMarkdown:     strings.TrimSpace(result.DraftMarkdown),
				RetrievalMarkdown: strings.TrimSpace(result.RetrievalMarkdown),
				ReportMarkdown:    result.ReportMarkdown,
			}, nil
		}
	}
	if w.client != nil {
		report, err := w.client.Complete(ctx, systemPrompt(snapshot), userPrompt(goal, mode, evidences, diagnostics, snapshot))
		if err == nil && strings.TrimSpace(report) != "" {
			return ReportOutput{
				Summary:        summarizeReport(report),
				ReportMarkdown: report,
			}, nil
		}
	}
	report := fallbackReport(goal, mode, evidences, diagnostics, snapshot)
	return ReportOutput{
		Summary:        summarizeReport(report),
		ReportMarkdown: report,
	}, nil
}

func systemPrompt(snapshot *domain.SkillRuntimeSnapshot) string {
	parts := []string{
		"You are a research agent that writes concise Markdown reports.",
		"Always include: 结论摘要, 关键发现, 知识库依据, 外部来源, 建议动作.",
	}
	if snapshot != nil && strings.TrimSpace(snapshot.Prompt) != "" {
		parts = append(parts, "Skill instructions: "+strings.TrimSpace(snapshot.Prompt))
	}
	return strings.Join(parts, " ")
}

func userPrompt(
	goal string,
	mode domain.RunMode,
	evidences []domain.Evidence,
	diagnostics ReportEvidenceDiagnostics,
	snapshot *domain.SkillRuntimeSnapshot,
) string {
	lines := []string{
		fmt.Sprintf("任务目标: %s", goal),
		fmt.Sprintf("执行模式: %s", mode),
		"证据:",
	}
	if snapshot != nil && strings.TrimSpace(snapshot.RuntimeSpecJSON) != "" {
		if spec, err := skillruntime.ParseSpec(snapshot.RuntimeSpecJSON); err == nil && strings.TrimSpace(spec.Instructions) != "" {
			lines = append(lines, fmt.Sprintf("技能指令: %s", spec.Instructions))
		}
	}
	for _, evidence := range evidences {
		lines = append(lines, fmt.Sprintf("- [%s] %s | %s | %s", evidence.Provider, evidence.Title, evidence.Repo, evidence.Snippet))
	}
	if diagnostics.GroupCount > 0 || diagnostics.DuplicateCount > 0 || diagnostics.ConflictCount > 0 || len(diagnostics.Warnings) > 0 {
		lines = append(lines, "证据整理诊断:")
		lines = append(
			lines,
			fmt.Sprintf(
				"- 分组: %d | 去重: %d | 冲突候选: %d",
				diagnostics.GroupCount,
				diagnostics.DuplicateCount,
				diagnostics.ConflictCount,
			),
		)
		if len(diagnostics.GroupLabels) > 0 {
			lines = append(lines, fmt.Sprintf("- 分组标签: %s", strings.Join(diagnostics.GroupLabels, "、")))
		}
		for _, warning := range diagnostics.Warnings {
			lines = append(lines, fmt.Sprintf("- 风险提示: %s", warning))
		}
	}
	return strings.Join(lines, "\n")
}

func summarizeReport(report string) string {
	lines := strings.Split(report, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(strings.TrimPrefix(line, "#"))
		if line != "" {
			return line
		}
	}
	return "任务已完成"
}

func forwardedSkillSnapshot(snapshot *domain.SkillRuntimeSnapshot) *provider.ForwardedSkillSnapshot {
	if snapshot == nil {
		return nil
	}
	runtimeSpec := map[string]any{}
	if strings.TrimSpace(snapshot.RuntimeSpecJSON) != "" {
		if spec, err := skillruntime.ParseSpec(snapshot.RuntimeSpecJSON); err == nil {
			runtimeSpec = map[string]any{
				"schemaVersion": spec.SchemaVersion,
				"skillKind":     string(spec.SkillKind),
				"responseMode":  spec.ResponseMode,
				"instructions":  spec.Instructions,
				"plannerPolicy": spec.PlannerPolicy,
				"toolPolicy":    spec.ToolPolicy,
				"outputPolicy":  spec.OutputPolicy,
				"metadata":      spec.Metadata,
			}
		}
	}
	return &provider.ForwardedSkillSnapshot{
		SnapshotID:     snapshot.ID,
		InstallationID: snapshot.InstallationID,
		DefinitionID:   snapshot.DefinitionID,
		RevisionID:     snapshot.RevisionID,
		Kind:           string(snapshot.Kind),
		Title:          snapshot.Title,
		Description:    snapshot.Description,
		Mode:           snapshot.Mode,
		Prompt:         snapshot.Prompt,
		RuntimeSpec:    runtimeSpec,
	}
}

func forwardedEvidence(evidences []domain.Evidence) []provider.ForwardedEvidence {
	items := make([]provider.ForwardedEvidence, 0, len(evidences))
	for _, evidence := range evidences {
		items = append(items, provider.ForwardedEvidence{
			Provider:     string(evidence.Provider),
			ConnectionID: evidence.ConnectionID,
			DocumentID:   evidence.DocumentID,
			ChunkID:      evidence.ChunkID,
			Title:        evidence.Title,
			Repo:         evidence.Repo,
			URL:          evidence.URL,
			Snippet:      evidence.Snippet,
			Body:         evidence.Body,
			Score:        evidence.Score,
		})
	}
	return items
}

func forwardedEvidenceDiagnostics(diagnostics ReportEvidenceDiagnostics) *provider.ForwardedEvidenceDiagnostics {
	if diagnostics.GroupCount == 0 && diagnostics.DuplicateCount == 0 && diagnostics.ConflictCount == 0 &&
		len(diagnostics.GroupLabels) == 0 && len(diagnostics.Warnings) == 0 {
		return nil
	}
	return &provider.ForwardedEvidenceDiagnostics{
		DuplicateCount: diagnostics.DuplicateCount,
		GroupCount:     diagnostics.GroupCount,
		ConflictCount:  diagnostics.ConflictCount,
		GroupLabels:    append([]string(nil), diagnostics.GroupLabels...),
		Warnings:       append([]string(nil), diagnostics.Warnings...),
	}
}

func fallbackReport(
	goal string,
	mode domain.RunMode,
	evidences []domain.Evidence,
	diagnostics ReportEvidenceDiagnostics,
	snapshot *domain.SkillRuntimeSnapshot,
) string {
	var knowledgeLines []string
	var webLines []string
	var riskLines []string
	for _, evidence := range evidences {
		line := fmt.Sprintf("- %s", evidence.Title)
		if evidence.Repo != "" {
			line += fmt.Sprintf("（%s）", evidence.Repo)
		}
		if evidence.URL != "" {
			line += fmt.Sprintf(" %s", evidence.URL)
		}
		if evidence.Snippet != "" {
			line += fmt.Sprintf("\n  - %s", evidence.Snippet)
		}
		if evidence.Provider != domain.ProviderWeb {
			knowledgeLines = append(knowledgeLines, line)
		} else {
			webLines = append(webLines, line)
		}
	}
	if len(knowledgeLines) == 0 {
		knowledgeLines = []string{"- 当前任务没有命中已同步的语雀内容。"}
	}
	if len(webLines) == 0 {
		webLines = []string{"- 当前任务没有新增外部网页证据。"}
	}
	if diagnostics.ConflictCount > 0 {
		riskLines = append(riskLines, fmt.Sprintf("- 当前证据整理阶段标记了 %d 组冲突候选，需要人工复核。", diagnostics.ConflictCount))
	}
	for _, warning := range diagnostics.Warnings {
		riskLines = append(riskLines, "- "+warning)
	}
	if len(riskLines) == 0 {
		riskLines = []string{"- 当前没有检测到明显的证据冲突。"}
	}

	skillSummary := ""
	if snapshot != nil && strings.TrimSpace(snapshot.Title) != "" {
		skillSummary = fmt.Sprintf("当前运行绑定了 skill `%s`，并按其指令调整报告风格。", snapshot.Title)
	}

	return strings.Join([]string{
		"# 结论摘要",
		fmt.Sprintf("本次任务围绕“%s”执行，系统采用 `%s` 模式整理内部与外部证据。", goal, mode),
		skillSummary,
		"",
		"## 关键发现",
		"- 当前版本已经完成显式任务规划、证据收集和报告交付。",
		"- 建议优先核查下面列出的知识库依据与外部来源，再决定后续动作。",
		"",
		"## 知识库依据",
		strings.Join(knowledgeLines, "\n"),
		"",
		"## 外部来源",
		strings.Join(webLines, "\n"),
		"",
		"## 风险提示",
		strings.Join(riskLines, "\n"),
		"",
		"## 建议动作",
		"- 对高价值来源做人工复核。",
		"- 如存在冲突候选，优先核对同一主题下的多个版本或来源。",
		"- 如需更强结论，继续追加限定条件或更具体问题。",
	}, "\n")
}
