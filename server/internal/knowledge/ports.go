package knowledge

import (
	"context"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

type KnowledgeConnectionStore interface {
	CreateKnowledgeConnection(ctx context.Context, connection domain.KnowledgeConnection) error
	UpdateKnowledgeConnection(ctx context.Context, connection domain.KnowledgeConnection) error
	GetKnowledgeConnection(ctx context.Context, userID, connectionID string) (domain.KnowledgeConnection, error)
	GetKnowledgeConnectionByID(ctx context.Context, connectionID string) (domain.KnowledgeConnection, error)
	ListKnowledgeConnections(ctx context.Context, userID string) ([]domain.KnowledgeConnection, error)
	DeleteKnowledgeConnection(ctx context.Context, userID, connectionID string) error
}

type KnowledgeSyncJobStore interface {
	CreateKnowledgeSyncJob(ctx context.Context, job domain.KnowledgeSyncJob) error
	UpdateKnowledgeSyncJob(ctx context.Context, job domain.KnowledgeSyncJob) error
	GetKnowledgeSyncJob(ctx context.Context, jobID string) (domain.KnowledgeSyncJob, error)
	ListKnowledgeSyncJobs(ctx context.Context, userID, connectionID string) ([]domain.KnowledgeSyncJob, error)
}

type KnowledgeMetadataStore interface {
	ListKnowledgeMetadata(ctx context.Context, connectionID string) ([]domain.KnowledgeDocumentMeta, error)
	ReplaceKnowledgeMetadata(ctx context.Context, connectionID string, docs []domain.KnowledgeDocumentMeta) error
	ListKnowledgeSourceDocuments(ctx context.Context, connectionID string) ([]domain.KnowledgeSourceDocument, error)
	ReplaceKnowledgeSourceDocuments(ctx context.Context, connectionID string, docs []domain.KnowledgeSourceDocument) error
}

type KnowledgeCorpusStore interface {
	GetKnowledgeCorpus(ctx context.Context, connectionID string) ([]domain.KnowledgeDocument, []domain.KnowledgeChunk, error)
}

type ServiceDeps struct {
	Connections KnowledgeConnectionStore
	SyncJobs    KnowledgeSyncJobStore
	Metadata    KnowledgeMetadataStore
	Corpus      KnowledgeCorpusStore
}
