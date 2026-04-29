package run

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/queue"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
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
	dispatcher     taskqueue.Dispatcher
	broker         *EventBroker
	planner        Planner
	knowledgeTool  tools.KnowledgeSearcher
	webSearchTool  tools.WebSearcher
	webExtractTool tools.WebExtractor
	evidenceMerger tools.EvidenceMerger
	reportWriter   tools.ReportWriter
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
		dispatcher:     dispatcher,
		broker:         broker,
		planner:        planner,
		knowledgeTool:  knowledgeTool,
		webSearchTool:  webSearchTool,
		webExtractTool: webExtractTool,
		evidenceMerger: evidenceMerger,
		reportWriter:   reportWriter,
	}
}

func (s *Service) SetDispatcher(dispatcher taskqueue.Dispatcher) {
	s.dispatcher = dispatcher
}

func (s *Service) CreateRun(ctx context.Context, userID string, input CreateRunInput) (domain.Run, error) {
	now := time.Now().UTC()
	title := strings.TrimSpace(input.Title)
	if title == "" {
		title = deriveTitle(input.Goal)
	}
	skillInstallationID, err := s.resolveSkillInstallationID(ctx, userID, strings.TrimSpace(input.SkillInstallationID), strings.TrimSpace(input.SkillDefinitionID))
	if err != nil {
		return domain.Run{}, err
	}
	run := domain.Run{
		ID:                     uuid.NewString(),
		UserID:                 userID,
		Title:                  title,
		Goal:                   strings.TrimSpace(input.Goal),
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
	if err := s.runStore.CreateRun(ctx, run); err != nil {
		return domain.Run{}, err
	}
	s.broker.Publish(run.ID, "run.created", map[string]string{
		"title": run.Title,
		"goal":  run.Goal,
	})
	if s.dispatcher != nil {
		if err := s.dispatcher.EnqueueRun(ctx, run.ID); err != nil {
			return domain.Run{}, err
		}
	}
	return run, nil
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
