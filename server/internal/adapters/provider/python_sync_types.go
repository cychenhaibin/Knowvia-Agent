package provider

type MirroredKnowledgeUpsertRequest struct {
	UserID         string                      `json:"user_id"`
	ConnectionID   string                      `json:"connection_id"`
	ConnectionMeta MirroredConnection          `json:"connection_meta"`
	Documents      []MirroredKnowledgeDocument `json:"documents"`
}

type MirroredKnowledgeUpsertResult struct {
	DocumentCount    int                       `json:"document_count"`
	ChunkCount       int                       `json:"chunk_count"`
	IndexVersion     string                    `json:"index_version"`
	ChangedDocuments map[string]int            `json:"changed_documents,omitempty"`
	Documents        []SyncedKnowledgeDocument `json:"documents,omitempty"`
}

type SyncedKnowledgeSourceRequest struct {
	UserID       string `json:"user_id"`
	ConnectionID string `json:"connection_id"`
	Name         string `json:"name"`
	Provider     string `json:"provider"`
	RawPayload   any    `json:"raw_payload,omitempty"`
}

type SyncedKnowledgeDocument struct {
	DocID      string `json:"doc_id"`
	Title      string `json:"title"`
	Repo       string `json:"repo"`
	DocRef     string `json:"doc_ref"`
	SourceURL  string `json:"source_url,omitempty"`
	UpdatedAt  string `json:"updated_at"`
	ChunkCount int    `json:"chunk_count,omitempty"`
}

type SyncedKnowledgeSourceResult struct {
	DocumentCount    int                       `json:"document_count"`
	ChunkCount       int                       `json:"chunk_count"`
	IndexVersion     string                    `json:"index_version"`
	ChangedDocuments map[string]int            `json:"changed_documents,omitempty"`
	Documents        []SyncedKnowledgeDocument `json:"documents"`
}

type indexDocumentInput struct {
	ExternalID string `json:"external_id"`
	Title      string `json:"title"`
	Repo       string `json:"repo"`
	DocRef     string `json:"doc_ref"`
	SourceURL  string `json:"source_url,omitempty"`
	UpdatedAt  string `json:"updated_at"`
	RawBody    string `json:"raw_body"`
	Provider   string `json:"provider,omitempty"`
}

type indexUpsertBatchRequest struct {
	UserID    string               `json:"user_id"`
	ScopeID   string               `json:"scope_id"`
	Name      string               `json:"name"`
	ScopeType string               `json:"scope_type"`
	Provider  string               `json:"provider"`
	SyncMode  string               `json:"sync_mode,omitempty"`
	Documents []indexDocumentInput `json:"documents"`
}

type indexSyncSourceRequest struct {
	UserID     string `json:"user_id"`
	ScopeID    string `json:"scope_id"`
	Name       string `json:"name"`
	Provider   string `json:"provider"`
	RawPayload any    `json:"raw_payload,omitempty"`
}
