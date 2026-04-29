package provider

type ForwardedSource struct {
	Type         string   `json:"type"`
	Provider     string   `json:"provider,omitempty"`
	ScopeID      string   `json:"scope_id,omitempty"`
	ConnectionID string   `json:"connection_id,omitempty"`
	DocumentID   string   `json:"document_id,omitempty"`
	ChunkID      string   `json:"chunk_id,omitempty"`
	Title        string   `json:"title"`
	URL          string   `json:"url,omitempty"`
	Repo         string   `json:"repo,omitempty"`
	Snippet      string   `json:"snippet,omitempty"`
	MatchedLines []string `json:"matched_answer_lines,omitempty"`
	Score        float64  `json:"score,omitempty"`
}

type ForwardedModelProfile struct {
	ProfileID   string   `json:"profile_id,omitempty"`
	Purpose     string   `json:"purpose,omitempty"`
	Provider    string   `json:"provider,omitempty"`
	Name        string   `json:"name,omitempty"`
	BaseURL     string   `json:"base_url,omitempty"`
	APIKey      string   `json:"api_key,omitempty"`
	ModelName   string   `json:"model_name,omitempty"`
	Temperature *float64 `json:"temperature,omitempty"`
	MaxTokens   int      `json:"max_tokens,omitempty"`
}

type ForwardedTraceContext struct {
	TraceID   string `json:"trace_id,omitempty"`
	RunID     string `json:"run_id,omitempty"`
	MessageID string `json:"message_id,omitempty"`
	SkillID   string `json:"skill_id,omitempty"`
}

type ForwardedMetrics struct {
	RetrieveMS int `json:"retrieve_ms,omitempty"`
	GenerateMS int `json:"generate_ms,omitempty"`
	TotalMS    int `json:"total_ms,omitempty"`
}

type ForwardedUsage struct {
	PromptTokens     int `json:"prompt_tokens,omitempty"`
	CompletionTokens int `json:"completion_tokens,omitempty"`
	TotalTokens      int `json:"total_tokens,omitempty"`
}

type ForwardedChatRequest struct {
	UserID        string                  `json:"user_id"`
	Message       string                  `json:"message"`
	ConnectionIDs []string                `json:"scope_ids,omitempty"`
	ChatModel     string                  `json:"chat_model,omitempty"`
	ChatAPIBase   string                  `json:"chat_api_base,omitempty"`
	ChatAPIKey    string                  `json:"chat_api_key,omitempty"`
	EnableSearch  bool                    `json:"enable_search,omitempty"`
	SkillID       string                  `json:"skill_id,omitempty"`
	SkillPrompt   string                  `json:"skill_prompt,omitempty"`
	Mode          string                  `json:"mode"`
	SkillSnapshot *ForwardedSkillSnapshot `json:"skill_snapshot,omitempty"`
	ModelProfile  *ForwardedModelProfile  `json:"model_profile,omitempty"`
	Trace         *ForwardedTraceContext  `json:"trace,omitempty"`
}

type ForwardedSkillSnapshot struct {
	SnapshotID     string         `json:"snapshot_id"`
	InstallationID string         `json:"installation_id,omitempty"`
	DefinitionID   string         `json:"definition_id,omitempty"`
	RevisionID     string         `json:"revision_id,omitempty"`
	Kind           string         `json:"kind,omitempty"`
	Title          string         `json:"title,omitempty"`
	Description    string         `json:"description,omitempty"`
	Mode           string         `json:"mode,omitempty"`
	Prompt         string         `json:"prompt,omitempty"`
	RuntimeSpec    map[string]any `json:"runtime_spec,omitempty"`
}

type ForwardedChatResult struct {
	TraceID string            `json:"trace_id,omitempty"`
	Answer  string            `json:"answer"`
	Sources []ForwardedSource `json:"sources,omitempty"`
	Metrics ForwardedMetrics  `json:"metrics,omitempty"`
	Usage   *ForwardedUsage   `json:"usage,omitempty"`
}

type ForwardedRetrieveRequest struct {
	UserID   string                 `json:"user_id"`
	ScopeIDs []string               `json:"scope_ids,omitempty"`
	Query    string                 `json:"query"`
	TopK     int                    `json:"top_k,omitempty"`
	Trace    *ForwardedTraceContext `json:"trace,omitempty"`
}

type ForwardedRetrieveResult struct {
	TraceID string            `json:"trace_id,omitempty"`
	Query   string            `json:"query"`
	Sources []ForwardedSource `json:"sources,omitempty"`
	Metrics ForwardedMetrics  `json:"metrics,omitempty"`
}

type ForwardedMergedEvidence struct {
	Provider     string  `json:"provider,omitempty"`
	ConnectionID string  `json:"connection_id,omitempty"`
	DocumentID   string  `json:"document_id,omitempty"`
	ChunkID      string  `json:"chunk_id,omitempty"`
	Title        string  `json:"title"`
	Repo         string  `json:"repo,omitempty"`
	URL          string  `json:"url,omitempty"`
	Snippet      string  `json:"snippet,omitempty"`
	Body         string  `json:"body,omitempty"`
	Score        float64 `json:"score,omitempty"`
}

type ForwardedEvidenceMergeRequest struct {
	UserID        string                  `json:"user_id"`
	Goal          string                  `json:"goal"`
	Mode          string                  `json:"mode"`
	SkillSnapshot *ForwardedSkillSnapshot `json:"skill_snapshot,omitempty"`
	Evidences     []ForwardedEvidence     `json:"evidences,omitempty"`
	TopK          int                     `json:"top_k,omitempty"`
}

type ForwardedEvidenceMergeResult struct {
	Goal           string                    `json:"goal"`
	Mode           string                    `json:"mode"`
	Evidences      []ForwardedMergedEvidence `json:"evidences,omitempty"`
	DuplicateCount int                       `json:"duplicate_count,omitempty"`
	GroupCount     int                       `json:"group_count,omitempty"`
	ConflictCount  int                       `json:"conflict_count,omitempty"`
	GroupLabels    []string                  `json:"group_labels,omitempty"`
	Warnings       []string                  `json:"warnings,omitempty"`
}

type ForwardedEvidenceDiagnostics struct {
	DuplicateCount int      `json:"duplicate_count,omitempty"`
	GroupCount     int      `json:"group_count,omitempty"`
	ConflictCount  int      `json:"conflict_count,omitempty"`
	GroupLabels    []string `json:"group_labels,omitempty"`
	Warnings       []string `json:"warnings,omitempty"`
}

type ForwardedReportRequest struct {
	UserID              string                        `json:"user_id"`
	Goal                string                        `json:"goal"`
	ScopeIDs            []string                      `json:"scope_ids,omitempty"`
	Mode                string                        `json:"mode"`
	ChatModel           string                        `json:"chat_model,omitempty"`
	ChatAPIBase         string                        `json:"chat_api_base,omitempty"`
	ChatAPIKey          string                        `json:"chat_api_key,omitempty"`
	SkillSnapshot       *ForwardedSkillSnapshot       `json:"skill_snapshot,omitempty"`
	Evidences           []ForwardedEvidence           `json:"evidences,omitempty"`
	EvidenceDiagnostics *ForwardedEvidenceDiagnostics `json:"evidence_diagnostics,omitempty"`
	ModelProfile        *ForwardedModelProfile        `json:"model_profile,omitempty"`
	Trace               *ForwardedTraceContext        `json:"trace,omitempty"`
}

type ForwardedEvidence struct {
	Provider     string  `json:"provider,omitempty"`
	ConnectionID string  `json:"connection_id,omitempty"`
	DocumentID   string  `json:"document_id,omitempty"`
	ChunkID      string  `json:"chunk_id,omitempty"`
	Title        string  `json:"title"`
	Repo         string  `json:"repo,omitempty"`
	URL          string  `json:"url,omitempty"`
	Snippet      string  `json:"snippet,omitempty"`
	Body         string  `json:"body,omitempty"`
	Score        float64 `json:"score,omitempty"`
}

type ForwardedReportResult struct {
	TraceID           string            `json:"trace_id,omitempty"`
	Summary           string            `json:"summary"`
	OutlineMarkdown   string            `json:"outline_markdown,omitempty"`
	DraftMarkdown     string            `json:"draft_markdown,omitempty"`
	RetrievalMarkdown string            `json:"retrieval_markdown,omitempty"`
	ReportMarkdown    string            `json:"report_markdown"`
	Sources           []ForwardedSource `json:"sources,omitempty"`
	Metrics           ForwardedMetrics  `json:"metrics,omitempty"`
}

type MirroredSkill struct {
	ID          string `json:"id"`
	UserID      string `json:"user_id"`
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Prompt      string `json:"prompt"`
	Mode        string `json:"mode"`
	Source      string `json:"source"`
	Enabled     bool   `json:"enabled"`
	RepoURL     string `json:"repo_url,omitempty"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type MirroredConnection struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	GroupLogin string `json:"group_login"`
	Namespace  string `json:"namespace,omitempty"`
}

type MirroredKnowledgeDocument struct {
	DocID     string `json:"doc_id"`
	Title     string `json:"title"`
	Repo      string `json:"repo"`
	DocRef    string `json:"doc_ref"`
	SourceURL string `json:"source_url,omitempty"`
	UpdatedAt string `json:"updated_at"`
	RawBody   string `json:"raw_body"`
}
