package httpapi

import (
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/knowledge"
)

type knowledgeConnectionListDTO struct {
	Items []knowledgeConnectionDTO `json:"items"`
}

type knowledgeSyncJobListDTO struct {
	Items []knowledgeSyncJobDTO `json:"items"`
}

type knowledgeConnectionDTO struct {
	ID           string                    `json:"id"`
	UserID       string                    `json:"userId"`
	Provider     domain.Provider           `json:"provider"`
	Name         string                    `json:"name"`
	SyncEnabled  bool                      `json:"syncEnabled"`
	LastSyncedAt *time.Time                `json:"lastSyncedAt,omitempty"`
	Yuque        *knowledgeYuqueConfigDTO  `json:"yuque,omitempty"`
	Feishu       *knowledgeFeishuConfigDTO `json:"feishu,omitempty"`
	CreatedAt    time.Time                 `json:"createdAt"`
	UpdatedAt    time.Time                 `json:"updatedAt"`
}

type knowledgeYuqueConfigDTO struct {
	GroupLogin string    `json:"groupLogin"`
	Namespace  string    `json:"namespace,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type knowledgeFeishuConfigDTO struct {
	AppID      string    `json:"appId"`
	EntryType  string    `json:"entryType"`
	EntryToken string    `json:"entryToken"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type knowledgeSyncJobDTO struct {
	ID           string               `json:"id"`
	UserID       string               `json:"userId"`
	ConnectionID string               `json:"connectionId"`
	Status       domain.SyncJobStatus `json:"status"`
	Summary      string               `json:"summary"`
	CreatedAt    time.Time            `json:"createdAt"`
	StartedAt    *time.Time           `json:"startedAt,omitempty"`
	FinishedAt   *time.Time           `json:"finishedAt,omitempty"`
}

type knowledgeRepoSummaryDTO struct {
	Name          string `json:"name"`
	DocumentCount int    `json:"documentCount"`
}

type knowledgeDocumentDTO struct {
	ID           string    `json:"id"`
	UserID       string    `json:"userId"`
	ConnectionID string    `json:"connectionId"`
	Repo         string    `json:"repo"`
	Title        string    `json:"title"`
	DocRef       string    `json:"docRef"`
	SourceURL    string    `json:"sourceUrl,omitempty"`
	BodyHash     string    `json:"bodyHash"`
	UpdatedAt    time.Time `json:"updatedAt"`
	CreatedAt    time.Time `json:"createdAt"`
}

type knowledgeDocumentMetaDTO struct {
	ID           string    `json:"id"`
	UserID       string    `json:"userId"`
	ConnectionID string    `json:"connectionId"`
	Repo         string    `json:"repo"`
	Title        string    `json:"title"`
	DocRef       string    `json:"docRef"`
	SourceURL    string    `json:"sourceUrl,omitempty"`
	ChunkCount   int       `json:"chunkCount,omitempty"`
	UpdatedAt    time.Time `json:"updatedAt"`
	CreatedAt    time.Time `json:"createdAt"`
}

type knowledgeConnectionDocumentsDTO struct {
	Connection    knowledgeConnectionDTO    `json:"connection"`
	RepoCount     int                       `json:"repoCount"`
	DocumentCount int                       `json:"documentCount"`
	ChunkCount    int                       `json:"chunkCount"`
	Repos         []knowledgeRepoSummaryDTO `json:"repos"`
	Documents     any                       `json:"documents"`
}

func mapKnowledgeConnection(connection domain.KnowledgeConnection) knowledgeConnectionDTO {
	dto := knowledgeConnectionDTO{
		ID:           connection.ID,
		UserID:       connection.UserID,
		Provider:     connection.Provider,
		Name:         connection.Name,
		SyncEnabled:  connection.SyncEnabled,
		LastSyncedAt: connection.LastSyncedAt,
		CreatedAt:    connection.CreatedAt,
		UpdatedAt:    connection.UpdatedAt,
	}
	if connection.Yuque != nil {
		dto.Yuque = &knowledgeYuqueConfigDTO{
			GroupLogin: connection.Yuque.GroupLogin,
			Namespace:  connection.Yuque.Namespace,
			CreatedAt:  connection.Yuque.CreatedAt,
			UpdatedAt:  connection.Yuque.UpdatedAt,
		}
	}
	if connection.Feishu != nil {
		dto.Feishu = &knowledgeFeishuConfigDTO{
			AppID:      connection.Feishu.AppID,
			EntryType:  connection.Feishu.EntryType,
			EntryToken: connection.Feishu.EntryToken,
			CreatedAt:  connection.Feishu.CreatedAt,
			UpdatedAt:  connection.Feishu.UpdatedAt,
		}
	}
	return dto
}

func mapKnowledgeConnections(connections []domain.KnowledgeConnection) []knowledgeConnectionDTO {
	items := make([]knowledgeConnectionDTO, 0, len(connections))
	for _, connection := range connections {
		items = append(items, mapKnowledgeConnection(connection))
	}
	return items
}

func mapKnowledgeSyncJob(job domain.KnowledgeSyncJob) knowledgeSyncJobDTO {
	return knowledgeSyncJobDTO{
		ID:           job.ID,
		UserID:       job.UserID,
		ConnectionID: job.ConnectionID,
		Status:       job.Status,
		Summary:      job.Summary,
		CreatedAt:    job.CreatedAt,
		StartedAt:    job.StartedAt,
		FinishedAt:   job.FinishedAt,
	}
}

func mapKnowledgeSyncJobs(jobs []domain.KnowledgeSyncJob) []knowledgeSyncJobDTO {
	items := make([]knowledgeSyncJobDTO, 0, len(jobs))
	for _, job := range jobs {
		items = append(items, mapKnowledgeSyncJob(job))
	}
	return items
}

func mapRepoSummaries(repos []knowledge.RepoSummary) []knowledgeRepoSummaryDTO {
	items := make([]knowledgeRepoSummaryDTO, 0, len(repos))
	for _, repo := range repos {
		items = append(items, knowledgeRepoSummaryDTO{
			Name:          repo.Name,
			DocumentCount: repo.DocumentCount,
		})
	}
	return items
}

func mapKnowledgeDocument(doc domain.KnowledgeDocument) knowledgeDocumentDTO {
	return knowledgeDocumentDTO{
		ID:           doc.ID,
		UserID:       doc.UserID,
		ConnectionID: doc.ConnectionID,
		Repo:         doc.Repo,
		Title:        doc.Title,
		DocRef:       doc.DocRef,
		SourceURL:    doc.SourceURL,
		BodyHash:     doc.BodyHash,
		UpdatedAt:    doc.UpdatedAt,
		CreatedAt:    doc.CreatedAt,
	}
}

func mapKnowledgeDocuments(docs []domain.KnowledgeDocument) []knowledgeDocumentDTO {
	items := make([]knowledgeDocumentDTO, 0, len(docs))
	for _, doc := range docs {
		items = append(items, mapKnowledgeDocument(doc))
	}
	return items
}

func mapKnowledgeDocumentMeta(doc domain.KnowledgeDocumentMeta) knowledgeDocumentMetaDTO {
	return knowledgeDocumentMetaDTO{
		ID:           doc.ID,
		UserID:       doc.UserID,
		ConnectionID: doc.ConnectionID,
		Repo:         doc.Repo,
		Title:        doc.Title,
		DocRef:       doc.DocRef,
		SourceURL:    doc.SourceURL,
		ChunkCount:   doc.ChunkCount,
		UpdatedAt:    doc.UpdatedAt,
		CreatedAt:    doc.CreatedAt,
	}
}

func mapKnowledgeDocumentMetas(docs []domain.KnowledgeDocumentMeta) []knowledgeDocumentMetaDTO {
	items := make([]knowledgeDocumentMetaDTO, 0, len(docs))
	for _, doc := range docs {
		items = append(items, mapKnowledgeDocumentMeta(doc))
	}
	return items
}

func mapConnectionDocumentsResult(result knowledge.ListConnectionDocumentsResult) knowledgeConnectionDocumentsDTO {
	var documents any
	switch docs := result.Documents.(type) {
	case []domain.KnowledgeDocument:
		documents = mapKnowledgeDocuments(docs)
	case []domain.KnowledgeDocumentMeta:
		documents = mapKnowledgeDocumentMetas(docs)
	default:
		documents = result.Documents
	}
	return knowledgeConnectionDocumentsDTO{
		Connection:    mapKnowledgeConnection(result.Connection),
		RepoCount:     result.RepoCount,
		DocumentCount: result.DocumentCount,
		ChunkCount:    result.ChunkCount,
		Repos:         mapRepoSummaries(result.Repos),
		Documents:     documents,
	}
}
