package run

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skillruntime"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/tools"
)

func (s *Service) ExecuteRun(ctx context.Context, runID string) error {
	run, err := s.runStore.GetRunByID(ctx, runID)
	if err != nil {
		return err
	}
	var runtimeSnapshot *domain.SkillRuntimeSnapshot
	var runtimeSpec *domain.SkillRuntimeSpec
	if strings.TrimSpace(run.SkillInstallationID) != "" {
		skill, err := s.skillStore.GetSkill(ctx, run.UserID, run.SkillInstallationID)
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
		if err := s.skillStore.CreateSkillRuntimeSnapshot(ctx, snapshot); err != nil {
			return err
		}
		spec, err := skillruntime.ParseSpec(snapshot.RuntimeSpecJSON)
		if err != nil {
			return err
		}
		run.SkillSnapshotID = snapshot.ID
		run.UpdatedAt = time.Now().UTC()
		if err := s.runStore.UpdateRun(ctx, run); err != nil {
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
	if err := s.runStore.UpdateRun(ctx, run); err != nil {
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
		if err := s.stepStore.UpsertRunStep(ctx, step); err != nil {
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
				if err := s.artifactStore.SaveArtifact(ctx, artifact); err != nil {
					return s.failRun(ctx, run, step, err)
				}
			}
			run.LatestArtifactID = reportArtifact.ID
			run.UpdatedAt = time.Now().UTC()
			if err := s.runStore.UpdateRun(ctx, run); err != nil {
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
			if err := s.sourceStore.SaveSources(ctx, run.ID, sources); err != nil {
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
	if err := s.runStore.UpdateRun(ctx, run); err != nil {
		return err
	}
	s.broker.Publish(run.ID, "run.completed", map[string]string{
		"latestArtifactId": run.LatestArtifactID,
	})
	return nil
}
