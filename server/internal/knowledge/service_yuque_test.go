package knowledge

import (
	"errors"
	"testing"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/provider/yuque"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func TestPrepareYuqueSyncDocumentsBuildsMergedSnapshot(t *testing.T) {
	now := time.Date(2026, 4, 28, 9, 0, 0, 0, time.UTC)
	connection := domain.KnowledgeConnection{
		ID:       "conn-1",
		UserID:   "user-1",
		Provider: domain.ProviderYuque,
	}
	currentDocs := []yuque.DocMeta{
		{ID: 1, Slug: "existing", Title: "Existing", UpdatedAt: "2026-04-01T00:00:00Z"},
		{ID: 2, Slug: "updated", Title: "Updated", UpdatedAt: "2026-04-28T08:00:00Z"},
		{ID: 3, Slug: "new-doc", Title: "New", UpdatedAt: "2026-04-28T08:30:00Z"},
	}
	storedByExternalID := map[string]domain.KnowledgeSourceDocument{
		"1": {
			ID:              "stored-1",
			UserID:          "user-1",
			ConnectionID:    "conn-1",
			Provider:        domain.ProviderYuque,
			ExternalID:      "1",
			Repo:            "team/repo",
			Title:           "Existing",
			DocRef:          "existing",
			SourceURL:       "https://www.yuque.com/team/repo/existing",
			RawBody:         "existing body",
			BodyHash:        sha256Hex("existing body"),
			SourceUpdatedAt: mustParseRFC3339(t, "2026-04-01T00:00:00Z"),
			CreatedAt:       now.Add(-24 * time.Hour),
			UpdatedAt:       now.Add(-24 * time.Hour),
		},
		"2": {
			ID:              "stored-2",
			UserID:          "user-1",
			ConnectionID:    "conn-1",
			Provider:        domain.ProviderYuque,
			ExternalID:      "2",
			Repo:            "team/repo",
			Title:           "Updated",
			DocRef:          "updated",
			SourceURL:       "https://www.yuque.com/team/repo/updated",
			RawBody:         "old updated body",
			BodyHash:        sha256Hex("old updated body"),
			SourceUpdatedAt: mustParseRFC3339(t, "2026-04-27T08:00:00Z"),
			CreatedAt:       now.Add(-48 * time.Hour),
			UpdatedAt:       now.Add(-48 * time.Hour),
		},
	}

	prepared, err := prepareYuqueSyncDocuments(
		currentDocs,
		storedByExternalID,
		connection,
		"team/repo",
		now,
		func(slug string) (string, error) {
			switch slug {
			case "updated":
				return "fresh updated body", nil
			case "new-doc":
				return "brand new body", nil
			default:
				return "", errors.New("unexpected slug: " + slug)
			}
		},
	)
	if err != nil {
		t.Fatalf("prepareYuqueSyncDocuments returned error: %v", err)
	}

	if prepared.addedCount != 1 || prepared.updatedCount != 1 || prepared.unchangedCount != 1 {
		t.Fatalf("unexpected counts: added=%d updated=%d unchanged=%d", prepared.addedCount, prepared.updatedCount, prepared.unchangedCount)
	}
	if prepared.deferredCount != 0 || prepared.rateLimitErr != nil {
		t.Fatalf("expected no deferred docs, got deferred=%d rateLimit=%v", prepared.deferredCount, prepared.rateLimitErr)
	}
	if len(prepared.mergedDocs) != 3 || len(prepared.upsertDocs) != 3 {
		t.Fatalf("expected three merged docs, got merged=%d upsert=%d", len(prepared.mergedDocs), len(prepared.upsertDocs))
	}
}

func TestPrepareYuqueSyncDocumentsDefersRemainingChangesAfterRateLimit(t *testing.T) {
	now := time.Date(2026, 4, 28, 9, 0, 0, 0, time.UTC)
	connection := domain.KnowledgeConnection{
		ID:       "conn-1",
		UserID:   "user-1",
		Provider: domain.ProviderYuque,
	}
	currentDocs := []yuque.DocMeta{
		{ID: 1, Slug: "existing", Title: "Existing", UpdatedAt: "2026-04-01T00:00:00Z"},
		{ID: 2, Slug: "updated", Title: "Updated", UpdatedAt: "2026-04-28T08:00:00Z"},
		{ID: 3, Slug: "new-doc", Title: "New", UpdatedAt: "2026-04-28T08:30:00Z"},
	}
	storedByExternalID := map[string]domain.KnowledgeSourceDocument{
		"1": {
			ID:              "stored-1",
			UserID:          "user-1",
			ConnectionID:    "conn-1",
			Provider:        domain.ProviderYuque,
			ExternalID:      "1",
			Repo:            "team/repo",
			Title:           "Existing",
			DocRef:          "existing",
			SourceURL:       "https://www.yuque.com/team/repo/existing",
			RawBody:         "existing body",
			BodyHash:        sha256Hex("existing body"),
			SourceUpdatedAt: mustParseRFC3339(t, "2026-04-01T00:00:00Z"),
			CreatedAt:       now.Add(-24 * time.Hour),
			UpdatedAt:       now.Add(-24 * time.Hour),
		},
		"2": {
			ID:              "stored-2",
			UserID:          "user-1",
			ConnectionID:    "conn-1",
			Provider:        domain.ProviderYuque,
			ExternalID:      "2",
			Repo:            "team/repo",
			Title:           "Updated",
			DocRef:          "updated",
			SourceURL:       "https://www.yuque.com/team/repo/updated",
			RawBody:         "old updated body",
			BodyHash:        sha256Hex("old updated body"),
			SourceUpdatedAt: mustParseRFC3339(t, "2026-04-27T08:00:00Z"),
			CreatedAt:       now.Add(-48 * time.Hour),
			UpdatedAt:       now.Add(-48 * time.Hour),
		},
	}

	prepared, err := prepareYuqueSyncDocuments(
		currentDocs,
		storedByExternalID,
		connection,
		"team/repo",
		now,
		func(slug string) (string, error) {
			if slug == "updated" {
				return "", &yuque.APIError{StatusCode: 429, Message: "rate limited", RetryAfter: 2 * time.Hour}
			}
			t.Fatalf("fetchBody should stop before requesting slug %q", slug)
			return "", nil
		},
	)
	if err != nil {
		t.Fatalf("prepareYuqueSyncDocuments returned error: %v", err)
	}

	if prepared.rateLimitErr == nil {
		t.Fatal("expected rate limit error to be captured")
	}
	if prepared.updatedCount != 0 || prepared.addedCount != 0 {
		t.Fatalf("expected changed docs to be deferred, got added=%d updated=%d", prepared.addedCount, prepared.updatedCount)
	}
	if prepared.unchangedCount != 1 || prepared.deferredCount != 2 {
		t.Fatalf("unexpected counts: unchanged=%d deferred=%d", prepared.unchangedCount, prepared.deferredCount)
	}
	if len(prepared.deferredTitles) != 2 {
		t.Fatalf("expected deferred titles for updated and new docs, got %#v", prepared.deferredTitles)
	}
	if len(prepared.deferredDocs) != 2 {
		t.Fatalf("expected two deferred docs in queue, got %#v", prepared.deferredDocs)
	}
	if prepared.deferredDocs[0].ExternalID != "2" || prepared.deferredDocs[1].ExternalID != "3" {
		t.Fatalf("unexpected deferred queue order: %#v", prepared.deferredDocs)
	}
	if len(prepared.mergedDocs) != 2 {
		t.Fatalf("expected only existing snapshots to remain, got %d docs", len(prepared.mergedDocs))
	}
	if prepared.mergedDocs[1].ExternalID != "2" || prepared.mergedDocs[1].RawBody != "old updated body" {
		t.Fatalf("expected stored snapshot for updated doc to be preserved, got %#v", prepared.mergedDocs[1])
	}
	if _, exists := prepared.currentIDs["3"]; !exists {
		t.Fatal("expected deferred new doc to stay in currentIDs so it is not treated as deleted")
	}
}

func TestPrepareYuquePendingSyncDocumentsResumesQueuedDocsWithoutListPass(t *testing.T) {
	now := time.Date(2026, 4, 29, 9, 0, 0, 0, time.UTC)
	connection := domain.KnowledgeConnection{
		ID:       "conn-1",
		UserID:   "user-1",
		Provider: domain.ProviderYuque,
	}
	storedDocs := []domain.KnowledgeSourceDocument{
		{
			ID:              "stored-1",
			UserID:          "user-1",
			ConnectionID:    "conn-1",
			Provider:        domain.ProviderYuque,
			ExternalID:      "1",
			Repo:            "team/repo",
			Title:           "Existing",
			DocRef:          "existing",
			SourceURL:       "https://www.yuque.com/team/repo/existing",
			RawBody:         "existing body",
			BodyHash:        sha256Hex("existing body"),
			SourceUpdatedAt: mustParseRFC3339(t, "2026-04-01T00:00:00Z"),
			CreatedAt:       now.Add(-24 * time.Hour),
			UpdatedAt:       now.Add(-24 * time.Hour),
		},
		{
			ID:              "stored-2",
			UserID:          "user-1",
			ConnectionID:    "conn-1",
			Provider:        domain.ProviderYuque,
			ExternalID:      "2",
			Repo:            "team/repo",
			Title:           "Updated",
			DocRef:          "updated",
			SourceURL:       "https://www.yuque.com/team/repo/updated",
			RawBody:         "old updated body",
			BodyHash:        sha256Hex("old updated body"),
			SourceUpdatedAt: mustParseRFC3339(t, "2026-04-27T08:00:00Z"),
			CreatedAt:       now.Add(-48 * time.Hour),
			UpdatedAt:       now.Add(-48 * time.Hour),
		},
	}
	storedByExternalID := map[string]domain.KnowledgeSourceDocument{
		"1": storedDocs[0],
		"2": storedDocs[1],
	}
	pendingDocs := []domain.KnowledgeConnectionYuquePendingDoc{
		{
			ExternalID: "2",
			Slug:       "updated",
			Title:      "Updated",
			UpdatedAt:  mustParseRFC3339(t, "2026-04-28T08:00:00Z"),
		},
		{
			ExternalID: "3",
			Slug:       "new-doc",
			Title:      "New",
			UpdatedAt:  mustParseRFC3339(t, "2026-04-28T08:30:00Z"),
		},
	}

	prepared, err := prepareYuquePendingSyncDocuments(
		pendingDocs,
		storedDocs,
		storedByExternalID,
		connection,
		"team/repo",
		now,
		func(slug string) (string, error) {
			switch slug {
			case "updated":
				return "fresh updated body", nil
			case "new-doc":
				return "brand new body", nil
			default:
				return "", errors.New("unexpected slug: " + slug)
			}
		},
	)
	if err != nil {
		t.Fatalf("prepareYuquePendingSyncDocuments returned error: %v", err)
	}
	if prepared.rateLimitErr != nil || prepared.deferredCount != 0 {
		t.Fatalf("expected queue to fully drain, got rateLimit=%v deferred=%d", prepared.rateLimitErr, prepared.deferredCount)
	}
	if prepared.updatedCount != 1 || prepared.addedCount != 1 {
		t.Fatalf("expected one updated and one added doc, got updated=%d added=%d", prepared.updatedCount, prepared.addedCount)
	}
	if len(prepared.mergedDocs) != 3 || len(prepared.upsertDocs) != 3 {
		t.Fatalf("expected three merged docs after resume, got merged=%d upsert=%d", len(prepared.mergedDocs), len(prepared.upsertDocs))
	}
}

func mustParseRFC3339(t *testing.T, raw string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		t.Fatalf("parse time %q: %v", raw, err)
	}
	return parsed
}
