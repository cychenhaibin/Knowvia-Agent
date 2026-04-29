package provider

import (
	"context"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

type ForwardChatClient interface {
	StreamKnowledgeChat(
		ctx context.Context,
		req ForwardedChatRequest,
		onRetrieval func([]ForwardedSource) error,
		onDelta func(string) error,
	) (ForwardedChatResult, error)
}

type KnowledgeRetrieveForwardClient interface {
	RetrieveKnowledge(ctx context.Context, req ForwardedRetrieveRequest) (ForwardedRetrieveResult, error)
}

type EvidenceMergeForwardClient interface {
	MergeEvidence(ctx context.Context, req ForwardedEvidenceMergeRequest) (ForwardedEvidenceMergeResult, error)
}

type ReportForwardClient interface {
	GenerateReport(ctx context.Context, req ForwardedReportRequest) (ForwardedReportResult, error)
}

type MirrorClient interface {
	UpsertSkill(ctx context.Context, skill domain.Skill) error
	DeleteSkill(ctx context.Context, userID, skillID string) error
	UpsertKnowledge(ctx context.Context, req MirroredKnowledgeUpsertRequest) (MirroredKnowledgeUpsertResult, error)
	DeleteKnowledge(ctx context.Context, userID, connectionID string) error
}

type KnowledgeSyncClient interface {
	SyncKnowledgeSource(ctx context.Context, req SyncedKnowledgeSourceRequest) (SyncedKnowledgeSourceResult, error)
}
