package run

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/chat"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/gitrepo"
)

func (s *Service) RunTaskConversation(
	ctx context.Context,
	req chat.TaskConversationRequest,
	onChunk func(string) error,
) (chat.TaskConversationResult, error) {
	run, err := s.runStore.GetRun(ctx, req.UserID, req.RunID)
	if err != nil {
		return chat.TaskConversationResult{}, err
	}
	if run.Kind != domain.RunKindGitHubRepoAnalysis {
		return chat.TaskConversationResult{Handled: false}, nil
	}
	if run.Status == domain.RunStatusCompleted {
		return chat.TaskConversationResult{Handled: false}, nil
	}

	sourceURL := strings.TrimSpace(run.SourceURL)
	if sourceURL == "" {
		if detected, ok := ExtractGitHubRepoURL(firstNonEmpty(req.Message, run.Goal)); ok {
			sourceURL = detected
			run.SourceURL = sourceURL
		}
	}
	if sourceURL == "" {
		return chat.TaskConversationResult{Handled: true}, fmt.Errorf("github repository URL is required")
	}

	steps, err := s.createGitHubTaskSteps(ctx, run.ID)
	if err != nil {
		return chat.TaskConversationResult{}, err
	}

	fetchStep := steps[0]
	if err := s.startStep(ctx, &run, &fetchStep); err != nil {
		return chat.TaskConversationResult{}, err
	}
	analyzer := s.githubAnalyzer
	if analyzer == nil {
		analyzer = gitrepo.NewAnalyzer("")
	}
	analysis, err := analyzer.Analyze(ctx, gitrepo.AnalyzeRequest{
		RepoURL: sourceURL,
		Goal:    firstNonEmpty(req.Message, run.Goal),
	})
	if err != nil {
		return chat.TaskConversationResult{Handled: true}, s.failRun(ctx, run, fetchStep, err)
	}
	fetchStep.Summary = "Fetched repository and prepared a temporary read-only workspace."
	if err := s.completeStep(ctx, &run, &fetchStep); err != nil {
		return chat.TaskConversationResult{}, err
	}

	scanStep := steps[1]
	if err := s.startStep(ctx, &run, &scanStep); err != nil {
		return chat.TaskConversationResult{}, err
	}
	scanStep.Summary = fmt.Sprintf("Scanned repository tree and selected %d key files.", len(analysis.Scan.KeyFiles))
	if err := s.completeStep(ctx, &run, &scanStep); err != nil {
		return chat.TaskConversationResult{}, err
	}

	analyzeStep := steps[2]
	if err := s.startStep(ctx, &run, &analyzeStep); err != nil {
		return chat.TaskConversationResult{}, err
	}
	analyzeStep.Summary = fmt.Sprintf("Detected repository languages: %s.", strings.Join(analysis.Scan.Languages, ", "))
	if strings.TrimSpace(strings.Join(analysis.Scan.Languages, "")) == "" {
		analyzeStep.Summary = "Prepared repository structure summary from selected text files."
	}
	if err := s.completeStep(ctx, &run, &analyzeStep); err != nil {
		return chat.TaskConversationResult{}, err
	}

	writerStep := steps[3]
	if err := s.startStep(ctx, &run, &writerStep); err != nil {
		return chat.TaskConversationResult{}, err
	}
	markdown := strings.TrimSpace(analysis.Markdown)
	if markdown == "" {
		markdown = gitrepo.BuildFallbackCodeWiki(firstNonEmpty(req.Message, run.Goal), analysis.Scan)
	}
	artifact := domain.RunArtifact{
		ID:              uuid.NewString(),
		RunID:           run.ID,
		Kind:            domain.ArtifactKindCodeWiki,
		ContentMarkdown: markdown,
		Version:         1,
		CreatedAt:       time.Now().UTC(),
	}
	if err := s.artifactStore.SaveArtifact(ctx, artifact); err != nil {
		return chat.TaskConversationResult{Handled: true}, s.failRun(ctx, run, writerStep, err)
	}
	run.LatestArtifactID = artifact.ID
	writerStep.Summary = "Generated Code Wiki Markdown artifact."
	if err := s.completeStep(ctx, &run, &writerStep); err != nil {
		return chat.TaskConversationResult{}, err
	}
	if s.broker != nil {
		s.broker.Publish(run.ID, "artifact.updated", map[string]string{
			"artifactId": artifact.ID,
			"kind":       string(artifact.Kind),
		})
	}

	finalStep := steps[4]
	if err := s.startStep(ctx, &run, &finalStep); err != nil {
		return chat.TaskConversationResult{}, err
	}
	finalStep.Summary = "Stored Code Wiki artifact and completed GitHub repository analysis."
	if err := s.completeStep(ctx, &run, &finalStep); err != nil {
		return chat.TaskConversationResult{}, err
	}

	run.Status = domain.RunStatusCompleted
	run.UpdatedAt = time.Now().UTC()
	if err := s.runStore.UpdateRun(ctx, run); err != nil {
		return chat.TaskConversationResult{}, err
	}
	if s.broker != nil {
		s.broker.Publish(run.ID, "run.completed", map[string]string{
			"latestArtifactId": run.LatestArtifactID,
		})
	}
	if onChunk != nil {
		for _, chunk := range chunkMarkdown(markdown, 96) {
			if err := onChunk(chunk); err != nil {
				return chat.TaskConversationResult{Handled: true, Answer: markdown}, err
			}
		}
	}
	return chat.TaskConversationResult{Handled: true, Answer: markdown}, nil
}

func (s *Service) createGitHubTaskSteps(ctx context.Context, runID string) ([]domain.RunStep, error) {
	items := []struct {
		kind    domain.StepKind
		label   string
		summary string
	}{
		{domain.StepKindGitHubRepoFetch, "获取 GitHub 仓库", "Clone or download the public repository into a temporary workspace."},
		{domain.StepKindGitHubRepoScan, "扫描仓库结构", "Build a file tree and select important text files."},
		{domain.StepKindCodeStructureAnalyze, "分析代码结构", "Infer languages, modules, run commands, and key definitions."},
		{domain.StepKindCodeWikiWriter, "生成 Code Wiki", "Produce the structured Markdown Code Wiki artifact."},
		{domain.StepKindFinalize, "交付最终结果", "Store artifacts and complete the task."},
	}
	steps := make([]domain.RunStep, 0, len(items))
	for idx, item := range items {
		step := domain.RunStep{
			ID:        uuid.NewString(),
			RunID:     runID,
			Kind:      item.kind,
			Label:     item.label,
			Status:    domain.StepStatusPending,
			Summary:   item.summary,
			CreatedAt: time.Now().UTC().Add(time.Duration(idx) * time.Millisecond),
		}
		if err := s.stepStore.UpsertRunStep(ctx, step); err != nil {
			return nil, err
		}
		steps = append(steps, step)
	}
	if s.broker != nil {
		s.broker.Publish(runID, "run.planned", map[string]interface{}{
			"effectiveMode": domain.RunModeWeb,
			"steps":         steps,
		})
	}
	return steps, nil
}

func chunkMarkdown(markdown string, chunkSize int) []string {
	if chunkSize <= 0 {
		return []string{markdown}
	}
	runes := []rune(markdown)
	chunks := make([]string, 0, len(runes)/chunkSize+1)
	for start := 0; start < len(runes); start += chunkSize {
		end := start + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[start:end]))
	}
	return chunks
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
