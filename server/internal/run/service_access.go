package run

import (
	"context"
	"errors"
	"strings"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/persistence"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skillresolver"
)

func (s *Service) resolveSkillInstallationID(ctx context.Context, userID, installationID, definitionID string) (string, error) {
	if installationID == "" && definitionID == "" {
		return "", nil
	}
	record, err := skillresolver.ResolveInstallationRecord(ctx, s.selectionStore, userID, installationID, definitionID)
	if err != nil {
		return "", mapRunSkillSelectionError(installationID, definitionID, err)
	}
	return record.Installation.ID, nil
}

func mapRunSkillSelectionError(installationID, definitionID string, err error) error {
	definitionOnly := strings.TrimSpace(definitionID) != "" && strings.TrimSpace(installationID) == ""
	switch {
	case errors.Is(err, persistence.ErrNotFound) && definitionOnly:
		return ErrSkillDefinitionNotFound
	case errors.Is(err, persistence.ErrNotFound):
		return ErrSkillInstallationNotFound
	case errors.Is(err, skillresolver.ErrDefinitionMismatch):
		return ErrSkillDefinitionMismatch
	case errors.Is(err, skillresolver.ErrDisabled) && definitionOnly:
		return ErrNoEnabledSkillInstallation
	case errors.Is(err, skillresolver.ErrDisabled):
		return ErrSkillInstallationDisabled
	default:
		return err
	}
}

func (s *Service) GetRunDetails(ctx context.Context, userID, runID string) (domain.RunDetails, error) {
	run, err := s.runStore.GetRun(ctx, userID, runID)
	if err != nil {
		if errors.Is(err, persistence.ErrNotFound) {
			return domain.RunDetails{}, ErrRunNotFound
		}
		return domain.RunDetails{}, err
	}
	steps, _ := s.stepStore.ListRunSteps(ctx, runID)
	artifacts, _ := s.artifactStore.ListArtifacts(ctx, runID)
	sources, _ := s.sourceStore.ListSources(ctx, runID)
	return domain.RunDetails{
		Run:       run,
		Steps:     steps,
		Artifacts: artifacts,
		Sources:   sources,
	}, nil
}

func (s *Service) ListRuns(ctx context.Context, userID string) ([]domain.Run, error) {
	return s.runStore.ListRuns(ctx, userID)
}

func (s *Service) EnsureRunAccess(ctx context.Context, userID, runID string) error {
	_, err := s.runStore.GetRun(ctx, userID, runID)
	if errors.Is(err, persistence.ErrNotFound) {
		return ErrRunNotFound
	}
	return err
}

func (s *Service) SubscribeEvents(runID string) (<-chan domain.RunEvent, func()) {
	if s.broker == nil {
		ch := make(chan domain.RunEvent)
		close(ch)
		return ch, func() {}
	}
	return s.broker.Subscribe(runID)
}
