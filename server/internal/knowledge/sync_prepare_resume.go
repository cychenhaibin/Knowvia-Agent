package knowledge

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/provider"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

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
