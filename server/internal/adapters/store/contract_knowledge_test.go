package store

import (
	"context"
	"errors"
	"testing"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/persistence"
)

func testKnowledgeContract(t *testing.T, s contractStore) {
	t.Helper()

	ctx := context.Background()
	userID := "user-contract-knowledge"
	connPrimary := domain.KnowledgeConnection{
		ID:          "conn-contract-primary",
		UserID:      userID,
		Provider:    domain.ProviderYuque,
		Name:        "Primary Knowledge",
		SyncEnabled: true,
		Yuque: &domain.KnowledgeConnectionYuqueConfig{
			Token:      "secret-token",
			GroupLogin: "quickque",
			Namespace:  "engineering",
			PendingDocs: []domain.KnowledgeConnectionYuquePendingDoc{
				{
					ExternalID: "doc-001",
					Slug:       "intro",
					Title:      "Introduction",
					UpdatedAt:  contractTime(12),
				},
			},
			CreatedAt: contractTime(5),
			UpdatedAt: contractTime(6),
		},
		CreatedAt: contractTime(5),
		UpdatedAt: contractTime(8),
	}
	connSecondary := domain.KnowledgeConnection{
		ID:          "conn-contract-secondary",
		UserID:      userID,
		Provider:    domain.ProviderFeishu,
		Name:        "Secondary Knowledge",
		SyncEnabled: false,
		Feishu: &domain.KnowledgeConnectionFeishuConfig{
			AppID:      "app-id",
			AppSecret:  "app-secret",
			EntryType:  "wiki",
			EntryToken: "wiki-token",
			CreatedAt:  contractTime(6),
			UpdatedAt:  contractTime(7),
		},
		CreatedAt: contractTime(6),
		UpdatedAt: contractTime(7),
	}

	if err := s.CreateKnowledgeConnection(ctx, connPrimary); err != nil {
		t.Fatalf("CreateKnowledgeConnection primary failed: %v", err)
	}
	if err := s.CreateKnowledgeConnection(ctx, connSecondary); err != nil {
		t.Fatalf("CreateKnowledgeConnection secondary failed: %v", err)
	}

	gotPrimary, err := s.GetKnowledgeConnection(ctx, userID, connPrimary.ID)
	if err != nil {
		t.Fatalf("GetKnowledgeConnection failed: %v", err)
	}
	assertDeepEqual(t, "knowledge connection by user/id", gotPrimary, connPrimary)

	gotByID, err := s.GetKnowledgeConnectionByID(ctx, connPrimary.ID)
	if err != nil {
		t.Fatalf("GetKnowledgeConnectionByID failed: %v", err)
	}
	assertDeepEqual(t, "knowledge connection by id", gotByID, connPrimary)

	listedConnections, err := s.ListKnowledgeConnections(ctx, userID)
	if err != nil {
		t.Fatalf("ListKnowledgeConnections failed: %v", err)
	}
	expectedConnections := []domain.KnowledgeConnection{connPrimary, connSecondary}
	assertDeepEqual(t, "knowledge connections list order", listedConnections, expectedConnections)

	updatedPrimary := connPrimary
	lastSyncedAt := contractTime(20)
	updatedPrimary.Name = "Primary Knowledge Updated"
	updatedPrimary.SyncEnabled = false
	updatedPrimary.LastSyncedAt = &lastSyncedAt
	updatedPrimary.UpdatedAt = contractTime(21)
	updatedPrimary.Yuque = &domain.KnowledgeConnectionYuqueConfig{
		Token:      "secret-token-2",
		GroupLogin: "quickque",
		Namespace:  "updated",
		PendingDocs: []domain.KnowledgeConnectionYuquePendingDoc{
			{
				ExternalID: "doc-002",
				Slug:       "updated",
				Title:      "Updated Doc",
				UpdatedAt:  contractTime(22),
			},
		},
		CreatedAt: connPrimary.Yuque.CreatedAt,
		UpdatedAt: contractTime(21),
	}
	if err := s.UpdateKnowledgeConnection(ctx, updatedPrimary); err != nil {
		t.Fatalf("UpdateKnowledgeConnection failed: %v", err)
	}

	gotUpdatedPrimary, err := s.GetKnowledgeConnection(ctx, userID, connPrimary.ID)
	if err != nil {
		t.Fatalf("GetKnowledgeConnection after update failed: %v", err)
	}
	assertDeepEqual(t, "updated knowledge connection", gotUpdatedPrimary, updatedPrimary)

	syncJobOld := domain.KnowledgeSyncJob{
		ID:           "sync-job-old",
		UserID:       userID,
		ConnectionID: connPrimary.ID,
		Status:       domain.SyncJobQueued,
		Summary:      "queued",
		CreatedAt:    contractTime(30),
	}
	syncJobNew := domain.KnowledgeSyncJob{
		ID:           "sync-job-new",
		UserID:       userID,
		ConnectionID: connPrimary.ID,
		Status:       domain.SyncJobQueued,
		Summary:      "queued later",
		CreatedAt:    contractTime(31),
	}
	if err := s.CreateKnowledgeSyncJob(ctx, syncJobOld); err != nil {
		t.Fatalf("CreateKnowledgeSyncJob old failed: %v", err)
	}
	if err := s.CreateKnowledgeSyncJob(ctx, syncJobNew); err != nil {
		t.Fatalf("CreateKnowledgeSyncJob new failed: %v", err)
	}
	startedAt := contractTime(32)
	finishedAt := contractTime(33)
	syncJobNew.Status = domain.SyncJobCompleted
	syncJobNew.Summary = "done"
	syncJobNew.StartedAt = &startedAt
	syncJobNew.FinishedAt = &finishedAt
	if err := s.UpdateKnowledgeSyncJob(ctx, syncJobNew); err != nil {
		t.Fatalf("UpdateKnowledgeSyncJob failed: %v", err)
	}

	gotSyncJob, err := s.GetKnowledgeSyncJob(ctx, syncJobNew.ID)
	if err != nil {
		t.Fatalf("GetKnowledgeSyncJob failed: %v", err)
	}
	assertDeepEqual(t, "updated sync job", gotSyncJob, syncJobNew)

	listedJobs, err := s.ListKnowledgeSyncJobs(ctx, userID, connPrimary.ID)
	if err != nil {
		t.Fatalf("ListKnowledgeSyncJobs failed: %v", err)
	}
	expectedJobs := []domain.KnowledgeSyncJob{syncJobNew, syncJobOld}
	assertDeepEqual(t, "knowledge sync jobs list order", listedJobs, expectedJobs)

	metadata := []domain.KnowledgeDocumentMeta{
		{
			ID:           "meta-1",
			UserID:       userID,
			ConnectionID: connPrimary.ID,
			Repo:         "alpha",
			Title:        "Alpha Doc",
			DocRef:       "alpha-doc",
			SourceURL:    "https://example.com/alpha",
			ChunkCount:   3,
			UpdatedAt:    contractTime(40),
			CreatedAt:    contractTime(41),
		},
		{
			ID:           "meta-2",
			UserID:       userID,
			ConnectionID: connPrimary.ID,
			Repo:         "beta",
			Title:        "Beta Doc",
			DocRef:       "beta-doc",
			SourceURL:    "https://example.com/beta",
			ChunkCount:   1,
			UpdatedAt:    contractTime(39),
			CreatedAt:    contractTime(42),
		},
	}
	if err := s.ReplaceKnowledgeMetadata(ctx, connPrimary.ID, metadata); err != nil {
		t.Fatalf("ReplaceKnowledgeMetadata failed: %v", err)
	}
	gotMetadata, err := s.ListKnowledgeMetadata(ctx, connPrimary.ID)
	if err != nil {
		t.Fatalf("ListKnowledgeMetadata failed: %v", err)
	}
	assertDeepEqual(t, "knowledge metadata", gotMetadata, metadata)

	sourceDocuments := []domain.KnowledgeSourceDocument{
		{
			ID:              "source-1",
			UserID:          userID,
			ConnectionID:    connPrimary.ID,
			Provider:        domain.ProviderYuque,
			ExternalID:      "remote-1",
			Repo:            "alpha",
			Title:           "Alpha Source",
			DocRef:          "alpha-source",
			SourceURL:       "https://example.com/source-alpha",
			RawBody:         "alpha source body",
			BodyHash:        "hash-alpha",
			SourceUpdatedAt: contractTime(50),
			CreatedAt:       contractTime(51),
			UpdatedAt:       contractTime(52),
		},
		{
			ID:              "source-2",
			UserID:          userID,
			ConnectionID:    connPrimary.ID,
			Provider:        domain.ProviderYuque,
			ExternalID:      "remote-2",
			Repo:            "beta",
			Title:           "Beta Source",
			DocRef:          "beta-source",
			SourceURL:       "https://example.com/source-beta",
			RawBody:         "beta source body",
			BodyHash:        "hash-beta",
			SourceUpdatedAt: contractTime(49),
			CreatedAt:       contractTime(53),
			UpdatedAt:       contractTime(54),
		},
	}
	if err := s.ReplaceKnowledgeSourceDocuments(ctx, connPrimary.ID, sourceDocuments); err != nil {
		t.Fatalf("ReplaceKnowledgeSourceDocuments failed: %v", err)
	}
	gotSourceDocuments, err := s.ListKnowledgeSourceDocuments(ctx, connPrimary.ID)
	if err != nil {
		t.Fatalf("ListKnowledgeSourceDocuments failed: %v", err)
	}
	assertDeepEqual(t, "knowledge source documents", gotSourceDocuments, sourceDocuments)

	corpusDocuments := []domain.KnowledgeDocument{
		{
			ID:           "doc-1",
			UserID:       userID,
			ConnectionID: connPrimary.ID,
			Repo:         "alpha",
			Title:        "Corpus Alpha",
			DocRef:       "corpus-alpha",
			SourceURL:    "https://example.com/corpus-alpha",
			Body:         "corpus body alpha",
			BodyHash:     "corpus-hash-alpha",
			UpdatedAt:    contractTime(60),
			CreatedAt:    contractTime(61),
		},
	}
	corpusChunks := []domain.KnowledgeChunk{
		{
			ID:           "chunk-1",
			UserID:       userID,
			ConnectionID: connPrimary.ID,
			DocumentID:   "doc-1",
			ChunkIndex:   0,
			Content:      "alpha chunk content",
			ContentHash:  "chunk-hash-1",
			CreatedAt:    contractTime(62),
		},
		{
			ID:           "chunk-2",
			UserID:       userID,
			ConnectionID: connPrimary.ID,
			DocumentID:   "doc-1",
			ChunkIndex:   1,
			Content:      "alpha chunk content second",
			ContentHash:  "chunk-hash-2",
			CreatedAt:    contractTime(63),
		},
	}
	if err := s.ReplaceKnowledgeCorpus(ctx, connPrimary.ID, corpusDocuments, corpusChunks); err != nil {
		t.Fatalf("ReplaceKnowledgeCorpus failed: %v", err)
	}
	gotCorpusDocuments, gotCorpusChunks, err := s.GetKnowledgeCorpus(ctx, connPrimary.ID)
	if err != nil {
		t.Fatalf("GetKnowledgeCorpus failed: %v", err)
	}
	assertDeepEqual(t, "knowledge corpus documents", gotCorpusDocuments, corpusDocuments)
	assertDeepEqual(t, "knowledge corpus chunks", gotCorpusChunks, corpusChunks)

	if err := s.DeleteKnowledgeConnection(ctx, userID, connPrimary.ID); err != nil {
		t.Fatalf("DeleteKnowledgeConnection failed: %v", err)
	}
	if _, err := s.GetKnowledgeConnection(ctx, userID, connPrimary.ID); !errors.Is(err, persistence.ErrNotFound) {
		t.Fatalf("expected deleted connection to return not found, got %v", err)
	}
	remainingConnections, err := s.ListKnowledgeConnections(ctx, userID)
	if err != nil {
		t.Fatalf("ListKnowledgeConnections after delete failed: %v", err)
	}
	assertDeepEqual(t, "remaining knowledge connections after delete", remainingConnections, []domain.KnowledgeConnection{connSecondary})

	remainingJobs, err := s.ListKnowledgeSyncJobs(ctx, userID, connPrimary.ID)
	if err != nil {
		t.Fatalf("ListKnowledgeSyncJobs after delete failed: %v", err)
	}
	assertDeepEqual(t, "sync jobs after delete", remainingJobs, []domain.KnowledgeSyncJob{})

	remainingMetadata, err := s.ListKnowledgeMetadata(ctx, connPrimary.ID)
	if err != nil {
		t.Fatalf("ListKnowledgeMetadata after delete failed: %v", err)
	}
	assertDeepEqual(t, "metadata after delete", remainingMetadata, []domain.KnowledgeDocumentMeta{})

	remainingSourceDocuments, err := s.ListKnowledgeSourceDocuments(ctx, connPrimary.ID)
	if err != nil {
		t.Fatalf("ListKnowledgeSourceDocuments after delete failed: %v", err)
	}
	assertDeepEqual(t, "source documents after delete", remainingSourceDocuments, []domain.KnowledgeSourceDocument{})

	remainingCorpusDocs, remainingCorpusChunks, err := s.GetKnowledgeCorpus(ctx, connPrimary.ID)
	if err != nil {
		t.Fatalf("GetKnowledgeCorpus after delete failed: %v", err)
	}
	assertDeepEqual(t, "corpus docs after delete", remainingCorpusDocs, []domain.KnowledgeDocument{})
	assertDeepEqual(t, "corpus chunks after delete", remainingCorpusChunks, []domain.KnowledgeChunk{})
}
