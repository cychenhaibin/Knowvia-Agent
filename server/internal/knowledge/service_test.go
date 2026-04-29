package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/provider"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/store"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

type dispatcherStub struct {
	connectionID string
	jobID        string
	err          error
}

func (d *dispatcherStub) EnqueueRun(context.Context, string) error {
	return nil
}

func (d *dispatcherStub) EnqueueKnowledgeSync(_ context.Context, connectionID, jobID string) error {
	d.connectionID = connectionID
	d.jobID = jobID
	return d.err
}

func (d *dispatcherStub) Close() error {
	return nil
}

func TestCreateConnectionBuildsYuqueConnection(t *testing.T) {
	mem := store.NewMemoryStore()
	service := NewService(ServiceDeps{
		Connections: mem,
		SyncJobs:    mem,
		Metadata:    mem,
		Corpus:      mem,
	}, nil, nil)

	cfg, _ := json.Marshal(map[string]any{
		"token":      "token-1",
		"groupLogin": "team",
		"namespace":  "repo",
	})

	connection, err := service.CreateConnection(context.Background(), CreateConnectionInput{
		UserID:      "user-1",
		Provider:    string(domain.ProviderYuque),
		Name:        "",
		SyncEnabled: true,
		Config:      cfg,
	})
	if err != nil {
		t.Fatalf("create connection: %v", err)
	}
	if connection.Name != "Yuque Connection" {
		t.Fatalf("expected default name, got %q", connection.Name)
	}
	if connection.Yuque == nil || connection.Yuque.Token != "token-1" {
		t.Fatalf("expected yuque config to be populated, got %#v", connection.Yuque)
	}
}

func TestQueueSyncReturnsExistingActiveJob(t *testing.T) {
	mem := store.NewMemoryStore()
	service := NewService(ServiceDeps{
		Connections: mem,
		SyncJobs:    mem,
		Metadata:    mem,
		Corpus:      mem,
	}, nil, nil)
	dispatcher := &dispatcherStub{}
	service.SetDispatcher(dispatcher)

	now := time.Now().UTC()
	connection := domain.KnowledgeConnection{
		ID:        "conn-1",
		UserID:    "user-1",
		Provider:  domain.ProviderYuque,
		Name:      "Yuque",
		CreatedAt: now,
		UpdatedAt: now,
		Yuque: &domain.KnowledgeConnectionYuqueConfig{
			Token:      "token",
			GroupLogin: "team",
			Namespace:  "repo",
			CreatedAt:  now,
			UpdatedAt:  now,
		},
	}
	if err := mem.CreateKnowledgeConnection(context.Background(), connection); err != nil {
		t.Fatalf("create connection: %v", err)
	}
	existing := domain.KnowledgeSyncJob{
		ID:           "job-1",
		UserID:       "user-1",
		ConnectionID: connection.ID,
		Status:       domain.SyncJobRunning,
		CreatedAt:    now,
	}
	if err := mem.CreateKnowledgeSyncJob(context.Background(), existing); err != nil {
		t.Fatalf("create job: %v", err)
	}

	job, err := service.QueueSync(context.Background(), "user-1", connection.ID)
	if err != nil {
		t.Fatalf("queue sync: %v", err)
	}
	if job.ID != existing.ID {
		t.Fatalf("expected existing job %q, got %q", existing.ID, job.ID)
	}
	if dispatcher.jobID != "" {
		t.Fatalf("expected dispatcher not to run for active job, got %q", dispatcher.jobID)
	}
}

type errorMirrorClient struct {
	deleteKnowledgeErr error
}

func (c *errorMirrorClient) UpsertSkill(context.Context, domain.Skill) error   { return nil }
func (c *errorMirrorClient) DeleteSkill(context.Context, string, string) error { return nil }
func (c *errorMirrorClient) UpsertKnowledge(context.Context, provider.MirroredKnowledgeUpsertRequest) (provider.MirroredKnowledgeUpsertResult, error) {
	return provider.MirroredKnowledgeUpsertResult{}, nil
}
func (c *errorMirrorClient) DeleteKnowledge(context.Context, string, string) error {
	return c.deleteKnowledgeErr
}

func TestDeleteConnectionReturnsActiveJobError(t *testing.T) {
	mem := store.NewMemoryStore()
	service := NewService(ServiceDeps{
		Connections: mem,
		SyncJobs:    mem,
		Metadata:    mem,
		Corpus:      mem,
	}, nil, nil)

	now := time.Now().UTC()
	connection := domain.KnowledgeConnection{
		ID:        "conn-1",
		UserID:    "user-1",
		Provider:  domain.ProviderYuque,
		Name:      "Yuque",
		CreatedAt: now,
		UpdatedAt: now,
		Yuque: &domain.KnowledgeConnectionYuqueConfig{
			Token:      "token",
			GroupLogin: "team",
			Namespace:  "repo",
			CreatedAt:  now,
			UpdatedAt:  now,
		},
	}
	if err := mem.CreateKnowledgeConnection(context.Background(), connection); err != nil {
		t.Fatalf("create connection: %v", err)
	}
	if err := mem.CreateKnowledgeSyncJob(context.Background(), domain.KnowledgeSyncJob{
		ID:           "job-1",
		UserID:       "user-1",
		ConnectionID: "conn-1",
		Status:       domain.SyncJobRunning,
		CreatedAt:    now,
	}); err != nil {
		t.Fatalf("create job: %v", err)
	}

	err := service.DeleteConnection(context.Background(), "user-1", "conn-1")
	if !errors.Is(err, ErrActiveSyncJob) {
		t.Fatalf("expected ErrActiveSyncJob, got %v", err)
	}
}

func TestListConnectionDocumentsBuildsReadableSummary(t *testing.T) {
	mem := store.NewMemoryStore()
	service := NewService(ServiceDeps{
		Connections: mem,
		SyncJobs:    mem,
		Metadata:    mem,
		Corpus:      mem,
	}, nil, nil)

	now := time.Now().UTC()
	connection := domain.KnowledgeConnection{
		ID:        "conn-1",
		UserID:    "user-1",
		Provider:  domain.ProviderYuque,
		Name:      "Yuque",
		CreatedAt: now,
		UpdatedAt: now,
		Yuque: &domain.KnowledgeConnectionYuqueConfig{
			Token:      "token",
			GroupLogin: "team",
			Namespace:  "repo",
			CreatedAt:  now,
			UpdatedAt:  now,
		},
	}
	if err := mem.CreateKnowledgeConnection(context.Background(), connection); err != nil {
		t.Fatalf("create connection: %v", err)
	}
	if err := mem.ReplaceKnowledgeMetadata(context.Background(), connection.ID, []domain.KnowledgeDocumentMeta{
		{
			ID:           "doc-1",
			UserID:       "user-1",
			ConnectionID: connection.ID,
			Repo:         "repo-a",
			Title:        "A",
			ChunkCount:   3,
			UpdatedAt:    now.Add(time.Minute),
			CreatedAt:    now,
		},
		{
			ID:           "doc-2",
			UserID:       "user-1",
			ConnectionID: connection.ID,
			Repo:         "repo-a",
			Title:        "B",
			ChunkCount:   2,
			UpdatedAt:    now,
			CreatedAt:    now,
		},
	}); err != nil {
		t.Fatalf("replace metadata: %v", err)
	}

	result, err := service.ListConnectionDocuments(context.Background(), "user-1", connection.ID)
	if err != nil {
		t.Fatalf("list connection documents: %v", err)
	}
	if result.DocumentCount != 2 {
		t.Fatalf("expected 2 documents, got %d", result.DocumentCount)
	}
	if result.ChunkCount != 5 {
		t.Fatalf("expected 5 chunks, got %d", result.ChunkCount)
	}
	if len(result.Repos) != 1 || result.Repos[0].DocumentCount != 2 {
		t.Fatalf("expected repo summary to aggregate counts, got %#v", result.Repos)
	}
	docs, ok := result.Documents.([]domain.KnowledgeDocumentMeta)
	if !ok {
		t.Fatalf("expected metadata documents, got %T", result.Documents)
	}
	if len(docs) != 2 || docs[0].Title != "A" {
		t.Fatalf("expected documents sorted by updatedAt desc, got %#v", docs)
	}
}
