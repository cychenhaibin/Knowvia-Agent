package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/provider"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/provider/yuque"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/queue"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/persistence"
)

type Service struct {
	connectionStore KnowledgeConnectionStore
	syncJobStore    KnowledgeSyncJobStore
	metadataStore   KnowledgeMetadataStore
	corpusStore     KnowledgeCorpusStore
	syncClient      provider.KnowledgeSyncClient
	mirrorClient    provider.MirrorClient
	dispatcher      taskqueue.Dispatcher
}

type yuquePreparedSync struct {
	mergedDocs     []domain.KnowledgeSourceDocument
	upsertDocs     []provider.MirroredKnowledgeDocument
	currentIDs     map[string]struct{}
	addedCount     int
	updatedCount   int
	unchangedCount int
	deferredCount  int
	addedTitles    []string
	updatedTitles  []string
	deferredTitles []string
	deferredDocs   []domain.KnowledgeConnectionYuquePendingDoc
	rateLimitErr   *yuque.APIError
}

type yuqueResumePreparedSync struct {
	mergedDocs     []domain.KnowledgeSourceDocument
	upsertDocs     []provider.MirroredKnowledgeDocument
	addedCount     int
	updatedCount   int
	deferredCount  int
	addedTitles    []string
	updatedTitles  []string
	deferredTitles []string
	deferredDocs   []domain.KnowledgeConnectionYuquePendingDoc
	rateLimitErr   *yuque.APIError
}

func NewService(deps ServiceDeps, syncClient provider.KnowledgeSyncClient, mirrorClient provider.MirrorClient) *Service {
	return &Service{
		connectionStore: deps.Connections,
		syncJobStore:    deps.SyncJobs,
		metadataStore:   deps.Metadata,
		corpusStore:     deps.Corpus,
		syncClient:      syncClient,
		mirrorClient:    mirrorClient,
	}
}

func (s *Service) SetDispatcher(dispatcher taskqueue.Dispatcher) {
	s.dispatcher = dispatcher
}

type ValidationError struct {
	Message string
	Fields  []string
}

func (e *ValidationError) Error() string {
	return e.Message
}

var ErrActiveSyncJob = errors.New("knowledge: active sync job exists")
var ErrConnectionNotFound = errors.New("knowledge: connection not found")
var ErrInvalidRequestBody = errors.New("knowledge: invalid request body")

type MirrorSyncError struct {
	Err error
}

func (e *MirrorSyncError) Error() string {
	if e == nil || e.Err == nil {
		return "knowledge mirror sync failed"
	}
	return e.Err.Error()
}

func (e *MirrorSyncError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type CreateConnectionInput struct {
	UserID      string
	Provider    string
	Name        string
	SyncEnabled bool
	Config      json.RawMessage
}

type CreateYuqueConnectionInput struct {
	UserID      string
	Name        string
	Token       string
	GroupLogin  string
	Namespace   string
	SyncEnabled bool
}

func (s *Service) QueueSync(ctx context.Context, userID, connectionID string) (domain.KnowledgeSyncJob, error) {
	if s.dispatcher == nil {
		return domain.KnowledgeSyncJob{}, errors.New("knowledge dispatcher unavailable")
	}
	connection, err := s.connectionStore.GetKnowledgeConnection(ctx, userID, connectionID)
	if err != nil {
		if errors.Is(err, persistence.ErrNotFound) {
			return domain.KnowledgeSyncJob{}, ErrConnectionNotFound
		}
		return domain.KnowledgeSyncJob{}, err
	}
	jobs, err := s.syncJobStore.ListKnowledgeSyncJobs(ctx, userID, connection.ID)
	if err != nil {
		return domain.KnowledgeSyncJob{}, err
	}
	for _, job := range jobs {
		if job.Status == domain.SyncJobQueued || job.Status == domain.SyncJobRunning {
			return job, nil
		}
	}
	job := domain.KnowledgeSyncJob{
		ID:           uuid.NewString(),
		UserID:       userID,
		ConnectionID: connection.ID,
		Status:       domain.SyncJobQueued,
		Summary:      "Queued for sync.",
		CreatedAt:    time.Now().UTC(),
	}
	if err := s.syncJobStore.CreateKnowledgeSyncJob(ctx, job); err != nil {
		return domain.KnowledgeSyncJob{}, err
	}
	if err := s.dispatcher.EnqueueKnowledgeSync(ctx, connection.ID, job.ID); err != nil {
		return domain.KnowledgeSyncJob{}, err
	}
	return job, nil
}
