package knowledge

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/provider"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/store"
	"github.com/google/uuid"
)

type Service struct {
	store        store.Store
	syncClient   provider.KnowledgeSyncClient
	mirrorClient provider.MirrorClient
}

type incrementalSyncSummary struct {
	Kind           string   `json:"kind"`
	Documents      int      `json:"documents,omitempty"`
	Chunks         int      `json:"chunks,omitempty"`
	Added          int      `json:"added,omitempty"`
	Updated        int      `json:"updated,omitempty"`
	Deleted        int      `json:"deleted,omitempty"`
	Unchanged      int      `json:"unchanged,omitempty"`
	Deferred       int      `json:"deferred,omitempty"`
	RetryAfterSec  int      `json:"retryAfterSec,omitempty"`
	AddedTitles    []string `json:"addedTitles,omitempty"`
	UpdatedTitles  []string `json:"updatedTitles,omitempty"`
	DeletedTitles  []string `json:"deletedTitles,omitempty"`
	DeferredTitles []string `json:"deferredTitles,omitempty"`
	SyncedTitles   []string `json:"syncedTitles,omitempty"`
}

type syncErrorSummary struct {
	Kind     string `json:"kind"`
	Provider string `json:"provider"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	LogID    string `json:"logId,omitempty"`
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
	rateLimitErr   *APIError
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
	rateLimitErr   *APIError
}

func NewService(st store.Store, syncClient provider.KnowledgeSyncClient, mirrorClient provider.MirrorClient) *Service {
	return &Service{
		store:        st,
		syncClient:   syncClient,
		mirrorClient: mirrorClient,
	}
}

func (s *Service) ExecuteSync(ctx context.Context, connectionID, jobID string) error {
	job, err := s.store.GetKnowledgeSyncJob(ctx, jobID)
	if err != nil {
		return err
	}
	connection, err := s.store.GetKnowledgeConnectionByID(ctx, connectionID)
	if err != nil {
		return err
	}
	if s.syncClient == nil {
		return s.failJob(
			ctx,
			job,
			fmt.Errorf(
				"python knowledge sync unavailable; set QQA_PYTHON_PROXY_BASE_URL and start quickque-agent/llm/run_server.sh",
			),
		)
	}

	now := time.Now().UTC()
	job.Status = domain.SyncJobRunning
	job.StartedAt = &now
	job.Summary = "Preparing knowledge sync."
	if err := s.store.UpdateKnowledgeSyncJob(ctx, job); err != nil {
		return err
	}

	// Yuque now syncs incrementally in Go: compare the latest doc list with the
	// stored snapshot, fetch only changed bodies, then send the merged full set
	// of documents to llm for the existing chunk/embed/index pipeline.
	if connection.Provider == domain.ProviderYuque {
		return s.executeYuqueSync(ctx, connection, job)
	}

	// Other providers still forward the fetched raw payload to Python.
	var result provider.SyncedKnowledgeSourceResult
	switch connection.Provider {
	case domain.ProviderFeishu:
		if connection.Feishu == nil {
			return s.failJob(ctx, job, fmt.Errorf("missing feishu config"))
		}
		rawPayload, fetchErr := newFeishuClient().FetchRawPayload(
			ctx,
			connection.Feishu.AppID,
			connection.Feishu.AppSecret,
			connection.Feishu.EntryType,
			connection.Feishu.EntryToken,
		)
		if fetchErr != nil {
			return s.failJob(ctx, job, fetchErr)
		}
		result, err = s.syncClient.SyncKnowledgeSource(ctx, provider.SyncedKnowledgeSourceRequest{
			UserID:       connection.UserID,
			ConnectionID: connection.ID,
			Name:         connection.Name,
			Provider:     string(connection.Provider),
			RawPayload:   rawPayload,
		})
	default:
		return s.failJob(ctx, job, fmt.Errorf("knowledge provider %q is not supported", connection.Provider))
	}
	if err != nil {
		return s.failJob(ctx, job, err)
	}

	metadata := toKnowledgeMetadata(connection, result.Documents)
	if err := s.store.ReplaceKnowledgeMetadata(ctx, connection.ID, metadata); err != nil {
		return s.failJob(ctx, job, err)
	}

	finishedAt := time.Now().UTC()
	connection.LastSyncedAt = &finishedAt
	connection.UpdatedAt = finishedAt
	job.Status = domain.SyncJobCompleted
	job.FinishedAt = &finishedAt
	job.Summary = marshalIncrementalSyncSummary(incrementalSyncSummary{
		Kind:         "full_sync",
		Documents:    result.DocumentCount,
		Chunks:       result.ChunkCount,
		SyncedTitles: limitSyncedKnowledgeTitles(result.Documents),
	})
	if result.DocumentCount == 0 {
		job.Summary = marshalIncrementalSyncSummary(incrementalSyncSummary{
			Kind: "full_sync_empty",
		})
	}

	if err := s.store.UpdateKnowledgeConnection(ctx, connection); err != nil {
		return err
	}
	return s.store.UpdateKnowledgeSyncJob(ctx, job)
}

func (s *Service) executeYuqueSync(
	ctx context.Context,
	connection domain.KnowledgeConnection,
	job domain.KnowledgeSyncJob,
) error {
	if connection.Yuque == nil {
		return s.failJob(ctx, job, fmt.Errorf("missing yuque config"))
	}
	if s.mirrorClient == nil {
		return s.failJob(
			ctx,
			job,
			fmt.Errorf("python knowledge upsert unavailable; set QQA_PYTHON_PROXY_BASE_URL and start quickque-agent/llm/run_server.sh"),
		)
	}

	client := NewClient()
	namespace := normalizeNamespace(connection.Yuque.GroupLogin, connection.Yuque.Namespace)

	// Step 2: load the last successful document snapshot from the database.
	storedDocs, err := s.store.ListKnowledgeSourceDocuments(ctx, connection.ID)
	if err != nil {
		return s.failJob(ctx, job, err)
	}
	storedByExternalID := make(map[string]domain.KnowledgeSourceDocument, len(storedDocs))
	for _, doc := range storedDocs {
		storedByExternalID[doc.ExternalID] = doc
	}

	// Step 3: compare list metadata to decide which bodies must be refetched.
	now := time.Now().UTC()
	if len(connection.Yuque.PendingDocs) > 0 {
		return s.executeYuquePendingSync(ctx, connection, job, namespace, storedDocs, storedByExternalID, now, client)
	}

	// Step 1: pull the current Yuque doc list for this single namespace.
	currentDocs, err := client.ListDocs(ctx, connection.Yuque.Token, namespace)
	if err != nil {
		return s.failJob(ctx, job, err)
	}

	prepared, err := prepareYuqueSyncDocuments(
		currentDocs,
		storedByExternalID,
		connection,
		namespace,
		now,
		func(slug string) (string, error) {
			return client.GetDocBody(ctx, connection.Yuque.Token, namespace, slug)
		},
	)
	if err != nil {
		return s.failJob(ctx, job, err)
	}

	sort.Slice(prepared.upsertDocs, func(i, j int) bool {
		if prepared.upsertDocs[i].Repo == prepared.upsertDocs[j].Repo {
			return prepared.upsertDocs[i].DocRef < prepared.upsertDocs[j].DocRef
		}
		return prepared.upsertDocs[i].Repo < prepared.upsertDocs[j].Repo
	})
	deletedCount := 0
	deletedTitles := make([]string, 0)
	for _, doc := range storedDocs {
		if _, exists := prepared.currentIDs[doc.ExternalID]; !exists {
			deletedCount++
			deletedTitles = append(deletedTitles, strings.TrimSpace(doc.Title))
		}
	}

	// Step 6: send the merged full document set to llm for chunking/indexing.
	result, err := s.mirrorClient.UpsertKnowledge(ctx, provider.MirroredKnowledgeUpsertRequest{
		UserID:       connection.UserID,
		ConnectionID: connection.ID,
		ConnectionMeta: provider.MirroredConnection{
			ID:         connection.ID,
			Name:       connection.Name,
			GroupLogin: connection.Yuque.GroupLogin,
			Namespace:  namespace,
		},
		Documents: prepared.upsertDocs,
	})
	if err != nil {
		return s.failJob(ctx, job, err)
	}

	// Step 7: persist the new full snapshot and the UI-facing metadata only
	// after llm indexing succeeds, so Go and llm stay in sync.
	if err := s.store.ReplaceKnowledgeSourceDocuments(ctx, connection.ID, prepared.mergedDocs); err != nil {
		return s.failJob(ctx, job, err)
	}
	metadata := toKnowledgeMetadata(connection, result.Documents)
	if err := s.store.ReplaceKnowledgeMetadata(ctx, connection.ID, metadata); err != nil {
		return s.failJob(ctx, job, err)
	}

	finishedAt := time.Now().UTC()
	connection.LastSyncedAt = &finishedAt
	connection.UpdatedAt = finishedAt
	job.Status = domain.SyncJobCompleted
	job.FinishedAt = &finishedAt
	connection.Yuque.PendingDocs = nil
	job.Summary = marshalIncrementalSyncSummary(incrementalSyncSummary{
		Kind:          "incremental_sync",
		Documents:     result.DocumentCount,
		Chunks:        result.ChunkCount,
		Added:         prepared.addedCount,
		Updated:       prepared.updatedCount,
		Deleted:       deletedCount,
		Unchanged:     prepared.unchangedCount,
		AddedTitles:   limitSummaryTitles(prepared.addedTitles),
		UpdatedTitles: limitSummaryTitles(prepared.updatedTitles),
		DeletedTitles: limitSummaryTitles(deletedTitles),
	})
	if prepared.rateLimitErr != nil {
		connection.Yuque.PendingDocs = prepared.deferredDocs
		job.Summary = marshalIncrementalSyncSummary(incrementalSyncSummary{
			Kind:           "incremental_sync_rate_limited",
			Documents:      result.DocumentCount,
			Chunks:         result.ChunkCount,
			Added:          prepared.addedCount,
			Updated:        prepared.updatedCount,
			Deleted:        deletedCount,
			Unchanged:      prepared.unchangedCount,
			Deferred:       prepared.deferredCount,
			RetryAfterSec:  retryAfterSeconds(prepared.rateLimitErr.RetryAfter),
			AddedTitles:    limitSummaryTitles(prepared.addedTitles),
			UpdatedTitles:  limitSummaryTitles(prepared.updatedTitles),
			DeletedTitles:  limitSummaryTitles(deletedTitles),
			DeferredTitles: limitSummaryTitles(prepared.deferredTitles),
		})
	} else if result.DocumentCount == 0 {
		job.Summary = marshalIncrementalSyncSummary(incrementalSyncSummary{
			Kind:          "incremental_sync_empty",
			Deleted:       deletedCount,
			DeletedTitles: limitSummaryTitles(deletedTitles),
		})
	}

	if err := s.store.UpdateKnowledgeConnection(ctx, connection); err != nil {
		return err
	}
	return s.store.UpdateKnowledgeSyncJob(ctx, job)
}

func (s *Service) executeYuquePendingSync(
	ctx context.Context,
	connection domain.KnowledgeConnection,
	job domain.KnowledgeSyncJob,
	namespace string,
	storedDocs []domain.KnowledgeSourceDocument,
	storedByExternalID map[string]domain.KnowledgeSourceDocument,
	now time.Time,
	client *Client,
) error {
	prepared, err := prepareYuquePendingSyncDocuments(
		connection.Yuque.PendingDocs,
		storedDocs,
		storedByExternalID,
		connection,
		namespace,
		now,
		func(slug string) (string, error) {
			return client.GetDocBody(ctx, connection.Yuque.Token, namespace, slug)
		},
	)
	if err != nil {
		return s.failJob(ctx, job, err)
	}

	sort.Slice(prepared.upsertDocs, func(i, j int) bool {
		if prepared.upsertDocs[i].Repo == prepared.upsertDocs[j].Repo {
			return prepared.upsertDocs[i].DocRef < prepared.upsertDocs[j].DocRef
		}
		return prepared.upsertDocs[i].Repo < prepared.upsertDocs[j].Repo
	})

	result, err := s.mirrorClient.UpsertKnowledge(ctx, provider.MirroredKnowledgeUpsertRequest{
		UserID:       connection.UserID,
		ConnectionID: connection.ID,
		ConnectionMeta: provider.MirroredConnection{
			ID:         connection.ID,
			Name:       connection.Name,
			GroupLogin: connection.Yuque.GroupLogin,
			Namespace:  namespace,
		},
		Documents: prepared.upsertDocs,
	})
	if err != nil {
		return s.failJob(ctx, job, err)
	}

	if err := s.store.ReplaceKnowledgeSourceDocuments(ctx, connection.ID, prepared.mergedDocs); err != nil {
		return s.failJob(ctx, job, err)
	}
	metadata := toKnowledgeMetadata(connection, result.Documents)
	if err := s.store.ReplaceKnowledgeMetadata(ctx, connection.ID, metadata); err != nil {
		return s.failJob(ctx, job, err)
	}

	finishedAt := time.Now().UTC()
	connection.LastSyncedAt = &finishedAt
	connection.UpdatedAt = finishedAt
	job.Status = domain.SyncJobCompleted
	job.FinishedAt = &finishedAt
	connection.Yuque.PendingDocs = nil
	job.Summary = marshalIncrementalSyncSummary(incrementalSyncSummary{
		Kind:          "incremental_sync",
		Documents:     result.DocumentCount,
		Chunks:        result.ChunkCount,
		Added:         prepared.addedCount,
		Updated:       prepared.updatedCount,
		Deleted:       0,
		AddedTitles:   limitSummaryTitles(prepared.addedTitles),
		UpdatedTitles: limitSummaryTitles(prepared.updatedTitles),
	})
	if prepared.rateLimitErr != nil {
		connection.Yuque.PendingDocs = prepared.deferredDocs
		job.Summary = marshalIncrementalSyncSummary(incrementalSyncSummary{
			Kind:           "incremental_sync_rate_limited",
			Documents:      result.DocumentCount,
			Chunks:         result.ChunkCount,
			Added:          prepared.addedCount,
			Updated:        prepared.updatedCount,
			Deleted:        0,
			Deferred:       prepared.deferredCount,
			RetryAfterSec:  retryAfterSeconds(prepared.rateLimitErr.RetryAfter),
			AddedTitles:    limitSummaryTitles(prepared.addedTitles),
			UpdatedTitles:  limitSummaryTitles(prepared.updatedTitles),
			DeferredTitles: limitSummaryTitles(prepared.deferredTitles),
		})
	}

	if err := s.store.UpdateKnowledgeConnection(ctx, connection); err != nil {
		return err
	}
	return s.store.UpdateKnowledgeSyncJob(ctx, job)
}

func toKnowledgeMetadata(
	connection domain.KnowledgeConnection,
	documents []provider.SyncedKnowledgeDocument,
) []domain.KnowledgeDocumentMeta {
	items := make([]domain.KnowledgeDocumentMeta, 0, len(documents))
	now := time.Now().UTC()
	for _, doc := range documents {
		updatedAt, _ := time.Parse(time.RFC3339Nano, strings.TrimSpace(doc.UpdatedAt))
		if updatedAt.IsZero() {
			updatedAt, _ = time.Parse(time.RFC3339, strings.TrimSpace(doc.UpdatedAt))
		}
		if updatedAt.IsZero() {
			updatedAt = now
		}
		items = append(items, domain.KnowledgeDocumentMeta{
			ID:           strings.TrimSpace(doc.DocID),
			UserID:       connection.UserID,
			ConnectionID: connection.ID,
			Repo:         strings.TrimSpace(doc.Repo),
			Title:        strings.TrimSpace(doc.Title),
			DocRef:       strings.TrimSpace(doc.DocRef),
			SourceURL:    strings.TrimSpace(doc.SourceURL),
			ChunkCount:   doc.ChunkCount,
			UpdatedAt:    updatedAt,
			CreatedAt:    now,
		})
	}
	return items
}

func (s *Service) failJob(ctx context.Context, job domain.KnowledgeSyncJob, err error) error {
	finishedAt := time.Now().UTC()
	job.Status = domain.SyncJobFailed
	job.FinishedAt = &finishedAt
	job.Summary = err.Error()

	var feishuErr *feishuAPIError
	if errors.As(err, &feishuErr) {
		job.Summary = marshalSyncErrorSummary(syncErrorSummary{
			Kind:     "sync_error",
			Provider: string(domain.ProviderFeishu),
			Code:     feishuErr.Code,
			Message:  feishuErr.Message,
			LogID:    feishuErr.LogID,
		})
	}
	_ = s.store.UpdateKnowledgeSyncJob(ctx, job)
	return err
}

func normalizeNamespace(groupLogin, namespace string) string {
	groupLogin = strings.TrimSpace(groupLogin)
	namespace = strings.Trim(strings.TrimSpace(namespace), "/")
	if namespace == "" || strings.Contains(namespace, "/") || groupLogin == "" {
		return namespace
	}
	return fmt.Sprintf("%s/%s", groupLogin, namespace)
}

func parseKnowledgeTime(raw string, fallback time.Time) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	if parsed, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return parsed
	}
	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		return parsed
	}
	return fallback
}

func stableKnowledgeSourceDocumentID(connectionID, externalID string) string {
	digest := sha256.Sum256([]byte(connectionID + ":" + externalID))
	return uuid.NewSHA1(uuid.NameSpaceOID, digest[:]).String()
}

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%x", sum[:])
}

func chooseCreatedAt(hasStored bool, stored time.Time, fallback time.Time) time.Time {
	if hasStored && !stored.IsZero() {
		return stored
	}
	return fallback
}

func prepareYuqueSyncDocuments(
	currentDocs []DocMeta,
	storedByExternalID map[string]domain.KnowledgeSourceDocument,
	connection domain.KnowledgeConnection,
	namespace string,
	now time.Time,
	fetchBody func(slug string) (string, error),
) (yuquePreparedSync, error) {
	prepared := yuquePreparedSync{
		mergedDocs:     make([]domain.KnowledgeSourceDocument, 0, len(currentDocs)),
		upsertDocs:     make([]provider.MirroredKnowledgeDocument, 0, len(currentDocs)),
		currentIDs:     make(map[string]struct{}, len(currentDocs)),
		addedTitles:    make([]string, 0, len(currentDocs)),
		updatedTitles:  make([]string, 0, len(currentDocs)),
		deferredTitles: make([]string, 0, len(currentDocs)),
	}

	for _, meta := range currentDocs {
		externalID := strconv.FormatInt(meta.ID, 10)
		if externalID == "" {
			continue
		}
		prepared.currentIDs[externalID] = struct{}{}
		slug := strings.TrimSpace(meta.Slug)
		if slug == "" {
			continue
		}
		sourceUpdatedAt := parseKnowledgeTime(meta.UpdatedAt, now)
		stored, hasStored := storedByExternalID[externalID]
		needsRefetch := !hasStored || !stored.SourceUpdatedAt.Equal(sourceUpdatedAt)

		// Once Yuque reports daily quota/rate limiting, keep syncing unchanged
		// documents but defer any remaining new/updated bodies to the next run.
		if prepared.rateLimitErr != nil && needsRefetch {
			prepared.deferDoc(externalID, slug, meta.Title, sourceUpdatedAt)
			if hasStored {
				prepared.appendStoredSnapshot(stored)
			}
			continue
		}

		rawBody := stored.RawBody
		if !hasStored {
			fetchedBody, err := fetchBody(slug)
			if err != nil {
				if apiErr, ok := asRateLimitedAPIError(err); ok {
					prepared.rateLimitErr = apiErr
					prepared.deferDoc(externalID, slug, meta.Title, sourceUpdatedAt)
					continue
				}
				return yuquePreparedSync{}, err
			}
			rawBody = fetchedBody
			prepared.addedCount++
			prepared.addedTitles = append(prepared.addedTitles, strings.TrimSpace(meta.Title))
		} else if !stored.SourceUpdatedAt.Equal(sourceUpdatedAt) {
			fetchedBody, err := fetchBody(slug)
			if err != nil {
				if apiErr, ok := asRateLimitedAPIError(err); ok {
					prepared.rateLimitErr = apiErr
					prepared.deferDoc(externalID, slug, meta.Title, sourceUpdatedAt)
					prepared.appendStoredSnapshot(stored)
					continue
				}
				return yuquePreparedSync{}, err
			}
			rawBody = fetchedBody
			prepared.updatedCount++
			prepared.updatedTitles = append(prepared.updatedTitles, strings.TrimSpace(meta.Title))
		} else {
			prepared.unchangedCount++
		}

		if strings.TrimSpace(rawBody) == "" {
			continue
		}

		snapshot := domain.KnowledgeSourceDocument{
			ID:              stableKnowledgeSourceDocumentID(connection.ID, externalID),
			UserID:          connection.UserID,
			ConnectionID:    connection.ID,
			Provider:        connection.Provider,
			ExternalID:      externalID,
			Repo:            namespace,
			Title:           strings.TrimSpace(meta.Title),
			DocRef:          slug,
			SourceURL:       fmt.Sprintf("https://www.yuque.com/%s/%s", namespace, slug),
			RawBody:         rawBody,
			BodyHash:        sha256Hex(rawBody),
			SourceUpdatedAt: sourceUpdatedAt,
			CreatedAt:       chooseCreatedAt(hasStored, stored.CreatedAt, now),
			UpdatedAt:       now,
		}
		prepared.appendSnapshot(snapshot)
	}

	return prepared, nil
}

func (p *yuquePreparedSync) appendSnapshot(snapshot domain.KnowledgeSourceDocument) {
	p.mergedDocs = append(p.mergedDocs, snapshot)
	p.upsertDocs = append(p.upsertDocs, provider.MirroredKnowledgeDocument{
		DocID:     snapshot.ExternalID,
		Title:     snapshot.Title,
		Repo:      snapshot.Repo,
		DocRef:    snapshot.DocRef,
		SourceURL: snapshot.SourceURL,
		UpdatedAt: snapshot.SourceUpdatedAt.Format(time.RFC3339Nano),
		RawBody:   snapshot.RawBody,
	})
}

func (p *yuquePreparedSync) appendStoredSnapshot(snapshot domain.KnowledgeSourceDocument) {
	if strings.TrimSpace(snapshot.RawBody) == "" {
		return
	}
	p.appendSnapshot(snapshot)
}

func (p *yuquePreparedSync) deferDoc(externalID, slug, title string, updatedAt time.Time) {
	p.deferredCount++
	p.deferredTitles = append(p.deferredTitles, strings.TrimSpace(title))
	p.deferredDocs = append(p.deferredDocs, domain.KnowledgeConnectionYuquePendingDoc{
		ExternalID: externalID,
		Slug:       strings.TrimSpace(slug),
		Title:      strings.TrimSpace(title),
		UpdatedAt:  updatedAt,
	})
}

func asRateLimitedAPIError(err error) (*APIError, bool) {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return nil, false
	}
	if apiErr.StatusCode != 429 {
		return nil, false
	}
	return apiErr, true
}

func retryAfterSeconds(delay time.Duration) int {
	if delay <= 0 {
		return 0
	}
	return int(delay.Round(time.Second) / time.Second)
}

func prepareYuquePendingSyncDocuments(
	pendingDocs []domain.KnowledgeConnectionYuquePendingDoc,
	storedDocs []domain.KnowledgeSourceDocument,
	storedByExternalID map[string]domain.KnowledgeSourceDocument,
	connection domain.KnowledgeConnection,
	namespace string,
	now time.Time,
	fetchBody func(slug string) (string, error),
) (yuqueResumePreparedSync, error) {
	prepared := yuqueResumePreparedSync{
		mergedDocs:     make([]domain.KnowledgeSourceDocument, 0, len(storedDocs)+len(pendingDocs)),
		upsertDocs:     make([]provider.MirroredKnowledgeDocument, 0, len(storedDocs)+len(pendingDocs)),
		addedTitles:    make([]string, 0, len(pendingDocs)),
		updatedTitles:  make([]string, 0, len(pendingDocs)),
		deferredTitles: make([]string, 0, len(pendingDocs)),
	}

	mergedByExternalID := make(map[string]domain.KnowledgeSourceDocument, len(storedDocs)+len(pendingDocs))
	for _, doc := range storedDocs {
		mergedByExternalID[doc.ExternalID] = doc
	}

	for index, pending := range pendingDocs {
		if prepared.rateLimitErr != nil {
			prepared.deferPendingDocs(pendingDocs[index:]...)
			break
		}

		stored, hasStored := storedByExternalID[pending.ExternalID]
		rawBody, err := fetchBody(strings.TrimSpace(pending.Slug))
		if err != nil {
			if apiErr, ok := asRateLimitedAPIError(err); ok {
				prepared.rateLimitErr = apiErr
				prepared.deferPendingDocs(pendingDocs[index:]...)
				break
			}
			return yuqueResumePreparedSync{}, err
		}
		if strings.TrimSpace(rawBody) == "" {
			continue
		}

		snapshot := domain.KnowledgeSourceDocument{
			ID:              stableKnowledgeSourceDocumentID(connection.ID, pending.ExternalID),
			UserID:          connection.UserID,
			ConnectionID:    connection.ID,
			Provider:        connection.Provider,
			ExternalID:      pending.ExternalID,
			Repo:            namespace,
			Title:           strings.TrimSpace(pending.Title),
			DocRef:          strings.TrimSpace(pending.Slug),
			SourceURL:       fmt.Sprintf("https://www.yuque.com/%s/%s", namespace, strings.TrimSpace(pending.Slug)),
			RawBody:         rawBody,
			BodyHash:        sha256Hex(rawBody),
			SourceUpdatedAt: pending.UpdatedAt,
			CreatedAt:       chooseCreatedAt(hasStored, stored.CreatedAt, now),
			UpdatedAt:       now,
		}
		mergedByExternalID[pending.ExternalID] = snapshot
		if hasStored {
			prepared.updatedCount++
			prepared.updatedTitles = append(prepared.updatedTitles, strings.TrimSpace(pending.Title))
		} else {
			prepared.addedCount++
			prepared.addedTitles = append(prepared.addedTitles, strings.TrimSpace(pending.Title))
		}
	}

	mergedDocs := make([]domain.KnowledgeSourceDocument, 0, len(mergedByExternalID))
	for _, doc := range mergedByExternalID {
		if strings.TrimSpace(doc.RawBody) == "" {
			continue
		}
		mergedDocs = append(mergedDocs, doc)
	}
	sort.Slice(mergedDocs, func(i, j int) bool {
		if mergedDocs[i].Repo == mergedDocs[j].Repo {
			if mergedDocs[i].DocRef == mergedDocs[j].DocRef {
				return mergedDocs[i].Title < mergedDocs[j].Title
			}
			return mergedDocs[i].DocRef < mergedDocs[j].DocRef
		}
		return mergedDocs[i].Repo < mergedDocs[j].Repo
	})
	prepared.mergedDocs = mergedDocs
	for _, snapshot := range mergedDocs {
		prepared.upsertDocs = append(prepared.upsertDocs, provider.MirroredKnowledgeDocument{
			DocID:     snapshot.ExternalID,
			Title:     snapshot.Title,
			Repo:      snapshot.Repo,
			DocRef:    snapshot.DocRef,
			SourceURL: snapshot.SourceURL,
			UpdatedAt: snapshot.SourceUpdatedAt.Format(time.RFC3339Nano),
			RawBody:   snapshot.RawBody,
		})
	}

	return prepared, nil
}

func (p *yuqueResumePreparedSync) deferPendingDocs(pendingDocs ...domain.KnowledgeConnectionYuquePendingDoc) {
	for _, pending := range pendingDocs {
		p.deferredCount++
		p.deferredTitles = append(p.deferredTitles, strings.TrimSpace(pending.Title))
		p.deferredDocs = append(p.deferredDocs, pending)
	}
}

func limitSummaryTitles(titles []string) []string {
	if len(titles) == 0 {
		return nil
	}
	filtered := make([]string, 0, len(titles))
	for _, title := range titles {
		title = strings.TrimSpace(title)
		if title == "" {
			continue
		}
		filtered = append(filtered, title)
		if len(filtered) == 5 {
			break
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

func marshalSyncErrorSummary(summary syncErrorSummary) string {
	encoded, err := json.Marshal(summary)
	if err != nil {
		return summary.Message
	}
	return string(encoded)
}

func marshalIncrementalSyncSummary(summary incrementalSyncSummary) string {
	payload, err := json.Marshal(summary)
	if err != nil {
		return "Knowledge sync completed."
	}
	return string(payload)
}

func limitSyncedKnowledgeTitles(documents []provider.SyncedKnowledgeDocument) []string {
	if len(documents) == 0 {
		return nil
	}
	titles := make([]string, 0, len(documents))
	for _, doc := range documents {
		title := strings.TrimSpace(doc.Title)
		if title == "" {
			continue
		}
		titles = append(titles, title)
		if len(titles) == 5 {
			break
		}
	}
	if len(titles) == 0 {
		return nil
	}
	return titles
}
