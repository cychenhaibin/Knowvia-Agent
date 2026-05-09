package run

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/queue"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/gitrepo"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skillresolver"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/tools"
)

var ErrRunNotFound = errors.New("run not found")
var ErrSkillDefinitionNotFound = errors.New("run: skill definition not found")
var ErrSkillInstallationNotFound = errors.New("run: skill installation not found")
var ErrSkillDefinitionMismatch = errors.New("run: skill definition mismatch")
var ErrSkillInstallationDisabled = errors.New("run: skill installation disabled")
var ErrNoEnabledSkillInstallation = errors.New("run: no enabled skill installation available for definition")

type Service struct {
	runStore       RunStore
	stepStore      RunStepStore
	artifactStore  RunArtifactStore
	sourceStore    RunSourceStore
	skillStore     RunSkillStore
	selectionStore skillresolver.InstallationRecordStore
	taskSessions   TaskSessionStore
	dispatcher     taskqueue.Dispatcher
	broker         *EventBroker
	planner        Planner
	knowledgeTool  tools.KnowledgeSearcher
	webSearchTool  tools.WebSearcher
	webExtractTool tools.WebExtractor
	evidenceMerger tools.EvidenceMerger
	reportWriter   tools.ReportWriter
	githubAnalyzer GitHubAnalyzer
}

type GitHubAnalyzer interface {
	Analyze(ctx context.Context, req gitrepo.AnalyzeRequest) (gitrepo.AnalyzeResult, error)
}

func NewService(
	deps ServiceDeps,
	dispatcher taskqueue.Dispatcher,
	broker *EventBroker,
	planner Planner,
	knowledgeTool tools.KnowledgeSearcher,
	webSearchTool tools.WebSearcher,
	webExtractTool tools.WebExtractor,
	evidenceMerger tools.EvidenceMerger,
	reportWriter tools.ReportWriter,
) *Service {
	return &Service{
		runStore:       deps.Runs,
		stepStore:      deps.Steps,
		artifactStore:  deps.Artifacts,
		sourceStore:    deps.Sources,
		skillStore:     deps.Skills,
		selectionStore: deps.Selection,
		taskSessions:   deps.TaskSessions,
		dispatcher:     dispatcher,
		broker:         broker,
		planner:        planner,
		knowledgeTool:  knowledgeTool,
		webSearchTool:  webSearchTool,
		webExtractTool: webExtractTool,
		evidenceMerger: evidenceMerger,
		reportWriter:   reportWriter,
		githubAnalyzer: gitrepo.NewAnalyzer(""),
	}
}

func (s *Service) SetDispatcher(dispatcher taskqueue.Dispatcher) {
	s.dispatcher = dispatcher
}

func (s *Service) SetGitHubAnalyzer(analyzer GitHubAnalyzer) {
	s.githubAnalyzer = analyzer
}

func (s *Service) CreateRun(ctx context.Context, userID string, input CreateRunInput) (domain.Run, error) {
	now := time.Now().UTC()
	kind := input.Kind
	sourceURL := strings.TrimSpace(input.SourceURL)
	if kind == "" {
		if detectedURL, ok := ExtractGitHubRepoURL(input.Goal); ok {
			kind = domain.RunKindGitHubRepoAnalysis
			sourceURL = detectedURL
		} else {
			kind = domain.RunKindResearch
		}
	}
	if kind == domain.RunKindGitHubRepoAnalysis && sourceURL == "" {
		if detectedURL, ok := ExtractGitHubRepoURL(input.Goal); ok {
			sourceURL = detectedURL
		}
	}
	title := strings.TrimSpace(input.Title)
	if title == "" {
		if kind == domain.RunKindGitHubRepoAnalysis && sourceURL != "" {
			title = deriveGitHubRunTitle(sourceURL)
		} else {
			title = deriveTitle(input.Goal)
		}
	}
	skillInstallationID, err := s.resolveSkillInstallationID(ctx, userID, strings.TrimSpace(input.SkillInstallationID), strings.TrimSpace(input.SkillDefinitionID))
	if err != nil {
		return domain.Run{}, err
	}
	taskSessionID := ""
	taskPrompt := ""
	if kind == domain.RunKindGitHubRepoAnalysis {
		taskSessionID = uuid.NewString()
		taskPrompt = BuildGitHubCodeWikiTaskPrompt(sourceURL, input.Goal)
	}
	run := domain.Run{
		ID:                     uuid.NewString(),
		UserID:                 userID,
		Kind:                   kind,
		Title:                  title,
		Goal:                   strings.TrimSpace(input.Goal),
		SourceURL:              sourceURL,
		TaskSessionID:          taskSessionID,
		TaskPrompt:             taskPrompt,
		RequestedMode:          input.Mode,
		KnowledgeConnectionIDs: slices.Clone(input.KnowledgeConnectionIDs),
		SkillInstallationID:    skillInstallationID,
		EffectiveMode:          domain.RunModeAuto,
		Status:                 domain.RunStatusQueued,
		CreatedAt:              now,
		UpdatedAt:              now,
	}
	if run.KnowledgeConnectionIDs == nil {
		run.KnowledgeConnectionIDs = []string{}
	}
	if run.RequestedMode == "" {
		run.RequestedMode = domain.RunModeAuto
	}
	if run.Kind == "" {
		run.Kind = domain.RunKindResearch
	}
	if run.Kind == domain.RunKindGitHubRepoAnalysis {
		if s.taskSessions == nil {
			return domain.Run{}, errors.New("run: task session store unavailable")
		}
		session := domain.ChatSession{
			ID:        taskSessionID,
			UserID:    userID,
			Title:     title,
			Kind:      domain.ChatSessionKindTask,
			RunID:     run.ID,
			Pinned:    false,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := s.taskSessions.CreateChatSession(ctx, session); err != nil {
			return domain.Run{}, err
		}
	}
	if err := s.runStore.CreateRun(ctx, run); err != nil {
		return domain.Run{}, err
	}
	s.broker.Publish(run.ID, "run.created", map[string]string{
		"title": run.Title,
		"goal":  run.Goal,
	})
	if s.dispatcher != nil && run.Kind != domain.RunKindGitHubRepoAnalysis {
		if err := s.dispatcher.EnqueueRun(ctx, run.ID); err != nil {
			return domain.Run{}, err
		}
	}
	return run, nil
}

var githubRepoURLPattern = regexp.MustCompile(`(?i)https?://github\.com/([A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+)(?:\.git)?(?:[/?#][^\s]*)?`)

func ExtractGitHubRepoURL(text string) (string, bool) {
	match := githubRepoURLPattern.FindStringSubmatch(strings.TrimSpace(text))
	if len(match) < 3 {
		return "", false
	}
	owner := strings.Trim(match[1], "/ ")
	repo := strings.TrimSuffix(strings.Trim(match[2], "/ "), ".git")
	if owner == "" || repo == "" {
		return "", false
	}
	return fmt.Sprintf("https://github.com/%s/%s", owner, repo), true
}

func deriveGitHubRunTitle(sourceURL string) string {
	sourceURL = strings.TrimSpace(sourceURL)
	parts := strings.Split(strings.TrimPrefix(sourceURL, "https://github.com/"), "/")
	if len(parts) >= 2 && parts[0] != "" && parts[1] != "" {
		return fmt.Sprintf("Code Wiki: %s/%s", parts[0], parts[1])
	}
	return "Code Wiki"
}

func BuildGitHubCodeWikiTaskPrompt(sourceURL, goal string) string {
	sourceURL = strings.TrimSpace(sourceURL)
	goal = strings.TrimSpace(goal)
	if goal == "" {
		goal = "分析并理解这个 GitHub 项目仓库，生成结构化的完整 Code Wiki Markdown 文档。"
	}
	return strings.Join([]string{
		"请启动 GitHub Repo Analysis 任务。",
		"",
		"仓库地址：" + sourceURL,
		"",
		"用户目标：" + goal,
		"",
		"请读取仓库结构和关键源码，生成结构化的完整 Code Wiki Markdown 文档。文档需要包括项目整体架构、主要模块职责、关键类与函数说明、依赖关系、项目运行方式、测试方式、扩展点，以及未覆盖文件或分析限制。",
		"",
		"请将最终 Markdown 标记为任务产物。",
	}, "\n")
}

func deriveTitle(goal string) string {
	goal = strings.TrimSpace(goal)
	runes := []rune(goal)
	if len(runes) <= 36 {
		return goal
	}
	return string(runes[:36]) + "..."
}

func mergeEvidence(base, extra []domain.Evidence) []domain.Evidence {
	return tools.LocalMergeEvidence(append(base, extra...))
}
