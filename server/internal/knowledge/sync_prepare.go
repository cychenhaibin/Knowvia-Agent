package knowledge

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/provider"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/provider/yuque"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

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
	currentDocs []yuque.DocMeta,
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

func asRateLimitedAPIError(err error) (*yuque.APIError, bool) {
	var apiErr *yuque.APIError
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
