package tools

import (
	"context"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

type KnowledgeSearcher interface {
	Search(ctx context.Context, userID string, connectionIDs []string, query string, limit int, trace *TraceContext) ([]domain.Evidence, error)
}

type WebSearcher interface {
	Search(ctx context.Context, query string, limit int) ([]domain.Evidence, error)
}

type WebExtractor interface {
	Extract(ctx context.Context, evidences []domain.Evidence, limit int) ([]domain.Evidence, error)
}

type EvidenceMergeResult struct {
	Evidences      []domain.Evidence
	DuplicateCount int
	GroupCount     int
	ConflictCount  int
	GroupLabels    []string
	Warnings       []string
}

type ReportEvidenceDiagnostics struct {
	DuplicateCount int
	GroupCount     int
	ConflictCount  int
	GroupLabels    []string
	Warnings       []string
}

type TraceContext struct {
	TraceID   string
	RunID     string
	MessageID string
	SkillID   string
}

type ReportOutput struct {
	Summary           string
	OutlineMarkdown   string
	DraftMarkdown     string
	RetrievalMarkdown string
	ReportMarkdown    string
}

type EvidenceMerger interface {
	Merge(
		ctx context.Context,
		userID string,
		goal string,
		mode domain.RunMode,
		evidences []domain.Evidence,
		snapshot *domain.SkillRuntimeSnapshot,
	) (EvidenceMergeResult, error)
}

type ReportWriter interface {
	Write(
		ctx context.Context,
		userID string,
		goal string,
		mode domain.RunMode,
		connectionIDs []string,
		evidences []domain.Evidence,
		diagnostics ReportEvidenceDiagnostics,
		snapshot *domain.SkillRuntimeSnapshot,
		trace *TraceContext,
	) (ReportOutput, error)
}
