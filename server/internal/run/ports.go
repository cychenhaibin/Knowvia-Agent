package run

import (
	"context"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skillresolver"
)

type RunStore interface {
	CreateRun(ctx context.Context, run domain.Run) error
	GetRun(ctx context.Context, userID, runID string) (domain.Run, error)
	GetRunByID(ctx context.Context, runID string) (domain.Run, error)
	UpdateRun(ctx context.Context, run domain.Run) error
	ListRuns(ctx context.Context, userID string) ([]domain.Run, error)
}

type RunStepStore interface {
	UpsertRunStep(ctx context.Context, step domain.RunStep) error
	ListRunSteps(ctx context.Context, runID string) ([]domain.RunStep, error)
}

type RunArtifactStore interface {
	SaveArtifact(ctx context.Context, artifact domain.RunArtifact) error
	ListArtifacts(ctx context.Context, runID string) ([]domain.RunArtifact, error)
}

type RunSourceStore interface {
	SaveSources(ctx context.Context, runID string, sources []domain.RunSource) error
	ListSources(ctx context.Context, runID string) ([]domain.RunSource, error)
}

type RunSkillStore interface {
	GetSkill(ctx context.Context, userID, skillID string) (domain.Skill, error)
	CreateSkillRuntimeSnapshot(ctx context.Context, snapshot domain.SkillRuntimeSnapshot) error
}

type ServiceDeps struct {
	Runs      RunStore
	Steps     RunStepStore
	Artifacts RunArtifactStore
	Sources   RunSourceStore
	Skills    RunSkillStore
	Selection skillresolver.InstallationRecordStore
}
