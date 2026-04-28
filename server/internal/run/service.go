package run

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skillresolver"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skillruntime"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/taskqueue"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/tools"
)

type CreateRunInput struct {
	Title                  string         `json:"title"`
	Goal                   string         `json:"goal"`
	Mode                   domain.RunMode `json:"mode"`
	KnowledgeConnectionIDs []string       `json:"knowledge_connection_ids"`
	SkillInstallationID    string         `json:"skill_installation_id"`
	SkillDefinitionID      string         `json:"skill_definition_id"`
}

type RunStore interface {
	CreateRun(ctx context.Context, run domain.Run) error
	GetRun(ctx context.Context, userID, runID string) (domain.Run, error)
	GetRunByID(ctx context.Context, runID string) (domain.Run, error)
	UpdateRun(ctx context.Context, run domain.Run) error
	ListRuns(ctx context.Context, userID string) ([]domain.Run, error)
	UpsertRunStep(ctx context.Context, step domain.RunStep) error
	ListRunSteps(ctx context.Context, runID string) ([]domain.RunStep, error)
	SaveArtifact(ctx context.Context, artifact domain.RunArtifact) error
	ListArtifacts(ctx context.Context, runID string) ([]domain.RunArtifact, error)
	SaveSources(ctx context.Context, runID string, sources []domain.RunSource) error
	ListSources(ctx context.Context, runID string) ([]domain.RunSource, error)
	GetSkill(ctx context.Context, userID, skillID string) (domain.Skill, error)
	CreateSkillRuntimeSnapshot(ctx context.Context, snapshot domain.SkillRuntimeSnapshot) error
	skillresolver.InstallationRecordStore
}

type Service struct {
	store          RunStore
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
	st RunStore,
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
		store:          st,
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
	if err := s.store.CreateRun(ctx, run); err != nil {
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

func (s *Service) resolveSkillInstallationID(ctx context.Context, userID, installationID, definitionID string) (string, error) {
	if installationID == "" && definitionID == "" {
		return "", nil
	}
	record, err := skillresolver.ResolveInstallationRecord(ctx, s.store, userID, installationID, definitionID)
	if err != nil {
		return "", err
	}
	return record.Installation.ID, nil
}

func (s *Service) GetRunDetails(ctx context.Context, userID, runID string) (domain.RunDetails, error) {
	run, err := s.store.GetRun(ctx, userID, runID)
	if err != nil {
		return domain.RunDetails{}, err
	}
	steps, _ := s.store.ListRunSteps(ctx, runID)
	artifacts, _ := s.store.ListArtifacts(ctx, runID)
	sources, _ := s.store.ListSources(ctx, runID)
	return domain.RunDetails{
		Run:       run,
		Steps:     steps,
		Artifacts: artifacts,
		Sources:   sources,
	}, nil
}

func (s *Service) ExecuteRun(ctx context.Context, runID string) error {
	run, err := s.store.GetRunByID(ctx, runID)
	if err != nil {
		return err
	}
	var runtimeSnapshot *domain.SkillRuntimeSnapshot
	var runtimeSpec *domain.SkillRuntimeSpec
	if strings.TrimSpace(run.SkillInstallationID) != "" {
		skill, err := s.store.GetSkill(ctx, run.UserID, run.SkillInstallationID)
		if err != nil {
			return err
		}
		snapshot, err := skillruntime.BuildSnapshot(
			skill,
			domain.SkillRuntimeScopeRun,
			run.ID,
			uuid.NewString(),
			time.Now().UTC(),
		)
		if err != nil {
			return err
		}
		if err := s.store.CreateSkillRuntimeSnapshot(ctx, snapshot); err != nil {
			return err
		}
		spec, err := skillruntime.ParseSpec(snapshot.RuntimeSpecJSON)
		if err != nil {
			return err
		}
		run.SkillSnapshotID = snapshot.ID
		run.UpdatedAt = time.Now().UTC()
		if err := s.store.UpdateRun(ctx, run); err != nil {
			return err
		}
		runtimeSnapshot = &snapshot
		runtimeSpec = &spec
	}

	plan := s.planner.BuildWithRuntimeSpecAndKnowledge(
		run.Goal,
		run.RequestedMode,
		runtimeSpec,
		len(run.KnowledgeConnectionIDs) > 0,
	)
	run.Status = domain.RunStatusPlanning
	run.EffectiveMode = plan.EffectiveMode
	run.UpdatedAt = time.Now().UTC()
	if err := s.store.UpdateRun(ctx, run); err != nil {
		return err
	}

	steps := make([]domain.RunStep, 0, len(plan.Steps))
	for idx, item := range plan.Steps {
		step := domain.RunStep{
			ID:        uuid.NewString(),
			RunID:     run.ID,
			Kind:      item.Kind,
			Label:     item.Label,
			Status:    domain.StepStatusPending,
			Summary:   item.Summary,
			CreatedAt: time.Now().UTC().Add(time.Duration(idx) * time.Millisecond),
		}
		if err := s.store.UpsertRunStep(ctx, step); err != nil {
			return err
		}
		steps = append(steps, step)
	}
	s.broker.Publish(run.ID, "run.planned", map[string]interface{}{
		"effectiveMode": plan.EffectiveMode,
		"steps":         plan.Steps,
	})

	evidences := []domain.Evidence{}
	reportDiagnostics := tools.ReportEvidenceDiagnostics{}
	for _, step := range steps {
		if err := s.startStep(ctx, &run, &step); err != nil {
			return s.failRun(ctx, run, step, err)
		}

		switch step.Kind {
		case domain.StepKindPlanning:
			step.Summary = fmt.Sprintf("Task classified as %s.", plan.EffectiveMode)
		case domain.StepKindYuqueSearch:
			found, err := s.knowledgeTool.Search(
				ctx,
				run.UserID,
				run.KnowledgeConnectionIDs,
				run.Goal,
				6,
				&tools.TraceContext{
					RunID:   run.ID,
					SkillID: skillSnapshotID(runtimeSnapshot),
				},
			)
			if err != nil {
				return s.failRun(ctx, run, step, err)
			}
			evidences = append(evidences, found...)
			step.Summary = fmt.Sprintf("Collected %d internal knowledge hits.", len(found))
		case domain.StepKindWebSearch:
			found, err := s.webSearchTool.Search(ctx, run.Goal, 5)
			if err != nil {
				return s.failRun(ctx, run, step, err)
			}
			evidences = append(evidences, found...)
			step.Summary = fmt.Sprintf("Collected %d web search hits.", len(found))
		case domain.StepKindWebExtract:
			extracted, err := s.webExtractTool.Extract(ctx, evidences, 3)
			if err != nil {
				return s.failRun(ctx, run, step, err)
			}
			evidences = mergeEvidence(evidences, extracted)
			step.Summary = fmt.Sprintf("Extracted readable content from %d pages.", len(extracted))
		case domain.StepKindEvidenceMerge:
			if s.evidenceMerger != nil {
				mergeResult, err := s.evidenceMerger.Merge(
					ctx,
					run.UserID,
					run.Goal,
					plan.EffectiveMode,
					evidences,
					runtimeSnapshot,
				)
				if err != nil {
					return s.failRun(ctx, run, step, err)
				}
				evidences = mergeResult.Evidences
				reportDiagnostics = tools.ReportEvidenceDiagnostics{
					DuplicateCount: mergeResult.DuplicateCount,
					GroupCount:     mergeResult.GroupCount,
					ConflictCount:  mergeResult.ConflictCount,
					GroupLabels:    append([]string(nil), mergeResult.GroupLabels...),
					Warnings:       append([]string(nil), mergeResult.Warnings...),
				}
				summary := fmt.Sprintf(
					"Ranked %d evidence items into %d groups and removed %d duplicates.",
					len(mergeResult.Evidences),
					mergeResult.GroupCount,
					mergeResult.DuplicateCount,
				)
				if mergeResult.ConflictCount > 0 {
					summary += fmt.Sprintf(" Flagged %d conflict candidates.", mergeResult.ConflictCount)
				}
				step.Summary = summary
			} else {
				evidences = tools.LocalMergeEvidence(evidences)
				reportDiagnostics = tools.ReportEvidenceDiagnostics{
					GroupCount: len(evidences),
				}
				step.Summary = fmt.Sprintf("Ranked and deduplicated %d evidence items.", len(evidences))
			}
		case domain.StepKindReportWriter:
			reportOutput, err := s.reportWriter.Write(
				ctx,
				run.UserID,
				run.Goal,
				plan.EffectiveMode,
				run.KnowledgeConnectionIDs,
				evidences,
				reportDiagnostics,
				runtimeSnapshot,
				&tools.TraceContext{
					RunID:   run.ID,
					SkillID: skillSnapshotID(runtimeSnapshot),
				},
			)
			if err != nil {
				return s.failRun(ctx, run, step, err)
			}
			artifactsToSave := make([]domain.RunArtifact, 0, 5)
			if strings.TrimSpace(reportOutput.OutlineMarkdown) != "" {
				artifactsToSave = append(artifactsToSave, domain.RunArtifact{
					ID:              uuid.NewString(),
					RunID:           run.ID,
					Kind:            domain.ArtifactKindReportOutline,
					ContentMarkdown: reportOutput.OutlineMarkdown,
					Version:         1,
					CreatedAt:       time.Now().UTC(),
				})
			}
			if strings.TrimSpace(reportOutput.DraftMarkdown) != "" {
				artifactsToSave = append(artifactsToSave, domain.RunArtifact{
					ID:              uuid.NewString(),
					RunID:           run.ID,
					Kind:            domain.ArtifactKindReportDraft,
					ContentMarkdown: reportOutput.DraftMarkdown,
					Version:         1,
					CreatedAt:       time.Now().UTC(),
				})
			}
			if strings.TrimSpace(reportOutput.RetrievalMarkdown) != "" {
				artifactsToSave = append(artifactsToSave, domain.RunArtifact{
					ID:              uuid.NewString(),
					RunID:           run.ID,
					Kind:            domain.ArtifactKindReportGrounding,
					ContentMarkdown: reportOutput.RetrievalMarkdown,
					Version:         1,
					CreatedAt:       time.Now().UTC(),
				})
			}
			reportArtifact := domain.RunArtifact{
				ID:              uuid.NewString(),
				RunID:           run.ID,
				Kind:            domain.ArtifactKindReport,
				ContentMarkdown: reportOutput.ReportMarkdown,
				Version:         1,
				CreatedAt:       time.Now().UTC(),
			}
			artifactsToSave = append(artifactsToSave, reportArtifact)
			finalArtifact := domain.RunArtifact{
				ID:              uuid.NewString(),
				RunID:           run.ID,
				Kind:            domain.ArtifactKindFinalAnswer,
				ContentMarkdown: reportOutput.Summary,
				Version:         1,
				CreatedAt:       time.Now().UTC(),
			}
			artifactsToSave = append(artifactsToSave, finalArtifact)
			for _, artifact := range artifactsToSave {
				if err := s.store.SaveArtifact(ctx, artifact); err != nil {
					return s.failRun(ctx, run, step, err)
				}
			}
			run.LatestArtifactID = reportArtifact.ID
			run.UpdatedAt = time.Now().UTC()
			if err := s.store.UpdateRun(ctx, run); err != nil {
				return err
			}
			s.broker.Publish(run.ID, "artifact.updated", map[string]string{
				"artifactId": reportArtifact.ID,
				"kind":       string(reportArtifact.Kind),
			})
			step.Summary = fmt.Sprintf(
				"Generated %d report-stage artifacts plus the final answer.",
				len(artifactsToSave)-1,
			)
		case domain.StepKindFinalize:
			sources := evidenceToSources(run.ID, evidences)
			if err := s.store.SaveSources(ctx, run.ID, sources); err != nil {
				return s.failRun(ctx, run, step, err)
			}
			step.Summary = fmt.Sprintf("Stored %d sources and finalized the run.", len(sources))
		}

		if err := s.completeStep(ctx, &run, &step); err != nil {
			return err
		}
	}

	run.Status = domain.RunStatusCompleted
	run.UpdatedAt = time.Now().UTC()
	if err := s.store.UpdateRun(ctx, run); err != nil {
		return err
	}
	s.broker.Publish(run.ID, "run.completed", map[string]string{
		"latestArtifactId": run.LatestArtifactID,
	})
	return nil
}

func (s *Service) startStep(ctx context.Context, run *domain.Run, step *domain.RunStep) error {
	now := time.Now().UTC()
	run.Status = domain.RunStatusRunning
	run.UpdatedAt = now
	if err := s.store.UpdateRun(ctx, *run); err != nil {
		return err
	}
	step.Status = domain.StepStatusRunning
	step.StartedAt = &now
	if err := s.store.UpsertRunStep(ctx, *step); err != nil {
		return err
	}
	s.broker.Publish(run.ID, "step.started", map[string]string{
		"stepId": step.ID,
		"label":  step.Label,
	})
	return nil
}

func (s *Service) completeStep(ctx context.Context, run *domain.Run, step *domain.RunStep) error {
	now := time.Now().UTC()
	step.Status = domain.StepStatusCompleted
	step.FinishedAt = &now
	if err := s.store.UpsertRunStep(ctx, *step); err != nil {
		return err
	}
	run.UpdatedAt = now
	if err := s.store.UpdateRun(ctx, *run); err != nil {
		return err
	}
	s.broker.Publish(run.ID, "step.completed", map[string]string{
		"stepId":  step.ID,
		"label":   step.Label,
		"summary": step.Summary,
	})
	return nil
}

func (s *Service) failRun(ctx context.Context, run domain.Run, step domain.RunStep, err error) error {
	now := time.Now().UTC()
	step.Status = domain.StepStatusFailed
	step.FinishedAt = &now
	step.Summary = err.Error()
	_ = s.store.UpsertRunStep(ctx, step)
	run.Status = domain.RunStatusFailed
	run.ErrorMessage = err.Error()
	run.UpdatedAt = now
	_ = s.store.UpdateRun(ctx, run)
	s.broker.Publish(run.ID, "run.failed", map[string]string{
		"stepId": step.ID,
		"error":  err.Error(),
	})
	return err
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

func skillSnapshotID(snapshot *domain.SkillRuntimeSnapshot) string {
	if snapshot == nil {
		return ""
	}
	return snapshot.ID
}

func evidenceToSources(runID string, evidences []domain.Evidence) []domain.RunSource {
	sources := make([]domain.RunSource, 0, len(evidences))
	for _, evidence := range evidences {
		sources = append(sources, domain.RunSource{
			ID:           uuid.NewString(),
			RunID:        runID,
			Provider:     evidence.Provider,
			ConnectionID: evidence.ConnectionID,
			DocumentID:   evidence.DocumentID,
			ChunkID:      evidence.ChunkID,
			Title:        evidence.Title,
			Repo:         evidence.Repo,
			URL:          evidence.URL,
			Snippet:      evidence.Snippet,
			Score:        evidence.Score,
			CreatedAt:    time.Now().UTC(),
		})
	}
	return sources
}
