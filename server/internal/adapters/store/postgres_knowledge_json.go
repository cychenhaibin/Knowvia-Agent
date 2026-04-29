package store

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

type knowledgeConnectionYuquePendingDocJSON struct {
	ExternalID string    `json:"externalId"`
	Slug       string    `json:"slug"`
	Title      string    `json:"title"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

func marshalKnowledgeConnectionYuquePendingDocs(docs []domain.KnowledgeConnectionYuquePendingDoc) string {
	if len(docs) == 0 {
		return "[]"
	}
	items := make([]knowledgeConnectionYuquePendingDocJSON, 0, len(docs))
	for _, doc := range docs {
		items = append(items, knowledgeConnectionYuquePendingDocJSON{
			ExternalID: doc.ExternalID,
			Slug:       doc.Slug,
			Title:      doc.Title,
			UpdatedAt:  doc.UpdatedAt,
		})
	}
	encoded, err := json.Marshal(items)
	if err != nil {
		return "[]"
	}
	return string(encoded)
}

func unmarshalKnowledgeConnectionYuquePendingDocs(raw string) []domain.KnowledgeConnectionYuquePendingDoc {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var items []knowledgeConnectionYuquePendingDocJSON
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil
	}
	if len(items) == 0 {
		return nil
	}
	docs := make([]domain.KnowledgeConnectionYuquePendingDoc, 0, len(items))
	for _, item := range items {
		docs = append(docs, domain.KnowledgeConnectionYuquePendingDoc{
			ExternalID: item.ExternalID,
			Slug:       item.Slug,
			Title:      item.Title,
			UpdatedAt:  item.UpdatedAt,
		})
	}
	return docs
}
