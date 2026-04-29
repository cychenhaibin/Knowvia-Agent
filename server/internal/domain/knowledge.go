package domain

import "time"

type SyncJobStatus string

const (
	SyncJobQueued    SyncJobStatus = "queued"
	SyncJobRunning   SyncJobStatus = "running"
	SyncJobCompleted SyncJobStatus = "completed"
	SyncJobFailed    SyncJobStatus = "failed"
)

type KnowledgeConnection struct {
	ID           string
	UserID       string
	Provider     Provider
	Name         string
	SyncEnabled  bool
	LastSyncedAt *time.Time
	Yuque        *KnowledgeConnectionYuqueConfig
	Feishu       *KnowledgeConnectionFeishuConfig
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type KnowledgeConnectionYuqueConfig struct {
	Token       string
	GroupLogin  string
	Namespace   string
	PendingDocs []KnowledgeConnectionYuquePendingDoc
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type KnowledgeConnectionYuquePendingDoc struct {
	ExternalID string
	Slug       string
	Title      string
	UpdatedAt  time.Time
}

type KnowledgeConnectionFeishuConfig struct {
	AppID      string
	AppSecret  string
	EntryType  string
	EntryToken string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type KnowledgeSyncJob struct {
	ID           string
	UserID       string
	ConnectionID string
	Status       SyncJobStatus
	Summary      string
	CreatedAt    time.Time
	StartedAt    *time.Time
	FinishedAt   *time.Time
}

type KnowledgeDocument struct {
	ID           string
	UserID       string
	ConnectionID string
	Repo         string
	Title        string
	DocRef       string
	SourceURL    string
	Body         string
	BodyHash     string
	UpdatedAt    time.Time
	CreatedAt    time.Time
}

type KnowledgeDocumentMeta struct {
	ID           string
	UserID       string
	ConnectionID string
	Repo         string
	Title        string
	DocRef       string
	SourceURL    string
	ChunkCount   int
	UpdatedAt    time.Time
	CreatedAt    time.Time
}

type KnowledgeSourceDocument struct {
	ID              string
	UserID          string
	ConnectionID    string
	Provider        Provider
	ExternalID      string
	Repo            string
	Title           string
	DocRef          string
	SourceURL       string
	RawBody         string
	BodyHash        string
	SourceUpdatedAt time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type KnowledgeChunk struct {
	ID           string
	UserID       string
	ConnectionID string
	DocumentID   string
	ChunkIndex   int
	Content      string
	ContentHash  string
	CreatedAt    time.Time
}

type KnowledgeHit struct {
	ConnectionID string
	DocumentID   string
	ChunkID      string
	Provider     string
	Title        string
	Repo         string
	URL          string
	Snippet      string
	Score        float64
}
