package domain

import "time"

type RunMode string

const (
	RunModeAuto   RunMode = "auto"
	RunModeKBOnly RunMode = "kb_only"
	RunModeWeb    RunMode = "web_only"
	RunModeHybrid RunMode = "hybrid"
)

type RunStatus string

const (
	RunStatusQueued    RunStatus = "queued"
	RunStatusPlanning  RunStatus = "planning"
	RunStatusRunning   RunStatus = "running"
	RunStatusCompleted RunStatus = "completed"
	RunStatusFailed    RunStatus = "failed"
)

type StepStatus string

const (
	StepStatusPending   StepStatus = "pending"
	StepStatusRunning   StepStatus = "running"
	StepStatusCompleted StepStatus = "completed"
	StepStatusFailed    StepStatus = "failed"
)

type StepKind string

const (
	StepKindPlanning      StepKind = "planning"
	StepKindYuqueSearch   StepKind = "yuque_search"
	StepKindWebSearch     StepKind = "web_search"
	StepKindWebExtract    StepKind = "web_page_extract"
	StepKindEvidenceMerge StepKind = "evidence_merge"
	StepKindReportWriter  StepKind = "report_writer"
	StepKindFinalize      StepKind = "finalize"
)

type ArtifactKind string

const (
	ArtifactKindReportOutline   ArtifactKind = "report_outline"
	ArtifactKindReportDraft     ArtifactKind = "report_draft"
	ArtifactKindReportGrounding ArtifactKind = "report_grounding"
	ArtifactKindReport          ArtifactKind = "report"
	ArtifactKindFinalAnswer     ArtifactKind = "final_answer"
)

type SyncJobStatus string

const (
	SyncJobQueued    SyncJobStatus = "queued"
	SyncJobRunning   SyncJobStatus = "running"
	SyncJobCompleted SyncJobStatus = "completed"
	SyncJobFailed    SyncJobStatus = "failed"
)

type MirrorTaskStatus string

const (
	MirrorTaskPending   MirrorTaskStatus = "pending"
	MirrorTaskRunning   MirrorTaskStatus = "running"
	MirrorTaskCompleted MirrorTaskStatus = "completed"
)

type MirrorTaskKind string

const (
	MirrorTaskSkillUpsert     MirrorTaskKind = "skill_upsert"
	MirrorTaskSkillDelete     MirrorTaskKind = "skill_delete"
	MirrorTaskKnowledgeUpsert MirrorTaskKind = "knowledge_upsert"
	MirrorTaskKnowledgeDelete MirrorTaskKind = "knowledge_delete"
)

type Provider string

const (
	ProviderYuque  Provider = "yuque"
	ProviderFeishu Provider = "feishu"
	ProviderWeb    Provider = "web"
)

type AuthProvider string

const (
	AuthProviderPassword  AuthProvider = "password"
	AuthProviderGoogle    AuthProvider = "google"
	AuthProviderMicrosoft AuthProvider = "microsoft"
	AuthProviderApple     AuthProvider = "apple"
	AuthProviderFacebook  AuthProvider = "facebook"
)

type ChatRole string

const (
	ChatRoleUser      ChatRole = "user"
	ChatRoleAssistant ChatRole = "assistant"
)

type ChatModelPurpose string

const (
	ChatModelPurposeGeneral   ChatModelPurpose = "general"
	ChatModelPurposeKnowledge ChatModelPurpose = "knowledge"
)

type ChatModelOrigin string

const (
	ChatModelOriginDefault ChatModelOrigin = "default"
	ChatModelOriginCustom  ChatModelOrigin = "custom"
)

const (
	DefaultChatAPIBaseURL              = "http://127.0.0.1:11434/v1"
	DefaultChatAPIKey                  = "ollama"
	DefaultGeneralChatModelName        = "gemma3n:e4b"
	DefaultGeneralChatModelNameLabel   = "Gemma 3n E4B"
	DefaultKnowledgeChatModelName      = "qwen3:8b"
	DefaultKnowledgeChatModelNameLabel = "Qwen3 8B"
	DefaultGeneralChatTemperature      = 0.05
	DefaultKnowledgeChatTemperature    = 0.05
)

type ChatRuntimeConfig struct {
	BaseURL     string  `json:"baseUrl"`
	APIKey      string  `json:"apiKey,omitempty"`
	ModelName   string  `json:"modelName"`
	Temperature float64 `json:"temperature"`
}

func DefaultChatTemperatureForPurpose(purpose ChatModelPurpose) float64 {
	switch purpose {
	case ChatModelPurposeKnowledge:
		return DefaultKnowledgeChatTemperature
	default:
		return DefaultGeneralChatTemperature
	}
}

func DefaultChatModelConfigForPurpose(purpose ChatModelPurpose) (name string, runtime ChatRuntimeConfig) {
	switch purpose {
	case ChatModelPurposeKnowledge:
		return DefaultKnowledgeChatModelNameLabel, ChatRuntimeConfig{
			BaseURL:     DefaultChatAPIBaseURL,
			APIKey:      DefaultChatAPIKey,
			ModelName:   DefaultKnowledgeChatModelName,
			Temperature: DefaultKnowledgeChatTemperature,
		}
	default:
		return DefaultGeneralChatModelNameLabel, ChatRuntimeConfig{
			BaseURL:     DefaultChatAPIBaseURL,
			APIKey:      DefaultChatAPIKey,
			ModelName:   DefaultGeneralChatModelName,
			Temperature: DefaultGeneralChatTemperature,
		}
	}
}

type SkillSource string

const (
	SkillSourceManual SkillSource = "manual"
	SkillSourceGithub SkillSource = "github"
	SkillSourceUpload SkillSource = "upload"
)

type SkillKind string

const (
	SkillKindChatProfile   SkillKind = "chat_profile"
	SkillKindAgentWorkflow SkillKind = "agent_workflow"
)

type SkillRuntimeScope string

const (
	SkillRuntimeScopeChat SkillRuntimeScope = "chat"
	SkillRuntimeScopeRun  SkillRuntimeScope = "run"
)

type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	DisplayName  string    `json:"displayName"`
	Email        string    `json:"email,omitempty"`
	AvatarURL    string    `json:"avatarUrl,omitempty"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"createdAt"`
}

type AuthIdentity struct {
	ID              string       `json:"id"`
	UserID          string       `json:"userId"`
	Provider        AuthProvider `json:"provider"`
	ProviderSubject string       `json:"providerSubject"`
	Email           string       `json:"email,omitempty"`
	EmailVerified   bool         `json:"emailVerified"`
	AvatarURL       string       `json:"avatarUrl,omitempty"`
	CreatedAt       time.Time    `json:"createdAt"`
	UpdatedAt       time.Time    `json:"updatedAt"`
}

type Session struct {
	ID           string     `json:"id"`
	UserID       string     `json:"userId"`
	AccessToken  string     `json:"accessToken"`
	RefreshToken string     `json:"refreshToken"`
	ExpiresAt    time.Time  `json:"expiresAt"`
	CreatedAt    time.Time  `json:"createdAt"`
	RevokedAt    *time.Time `json:"revokedAt,omitempty"`
}

type ChatSession struct {
	ID            string     `json:"id"`
	UserID        string     `json:"userId"`
	Title         string     `json:"title"`
	Pinned        bool       `json:"pinned"`
	LastMessageAt *time.Time `json:"lastMessageAt,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

type ChatMessage struct {
	ID           string     `json:"id"`
	SessionID    string     `json:"sessionId"`
	UserID       string     `json:"userId"`
	Role         ChatRole   `json:"role"`
	Content      string     `json:"content"`
	Skill        string     `json:"skill,omitempty"`
	UseKnowledge bool       `json:"useKnowledge"`
	CreatedAt    time.Time  `json:"createdAt"`
	CompletedAt  *time.Time `json:"completedAt,omitempty"`
}

type ChatMessageSource struct {
	ID           string    `json:"id"`
	MessageID    string    `json:"messageId"`
	Provider     Provider  `json:"provider"`
	ConnectionID string    `json:"connectionId,omitempty"`
	DocumentID   string    `json:"documentId,omitempty"`
	ChunkID      string    `json:"chunkId,omitempty"`
	Title        string    `json:"title"`
	Repo         string    `json:"repo,omitempty"`
	URL          string    `json:"url,omitempty"`
	Snippet      string    `json:"snippet"`
	MatchedLines []string  `json:"matchedAnswerLines,omitempty"`
	Score        float64   `json:"score"`
	CreatedAt    time.Time `json:"createdAt"`
}

type UserChatModel struct {
	ID          string           `json:"id"`
	UserID      string           `json:"userId"`
	Purpose     ChatModelPurpose `json:"purpose"`
	Origin      ChatModelOrigin  `json:"origin"`
	Name        string           `json:"name"`
	BaseURL     string           `json:"baseUrl"`
	APIKey      string           `json:"apiKey,omitempty"`
	ModelName   string           `json:"modelName"`
	Temperature float64          `json:"temperature"`
	IsSelected  bool             `json:"isSelected"`
	Available   bool             `json:"available"`
	CreatedAt   time.Time        `json:"createdAt"`
	UpdatedAt   time.Time        `json:"updatedAt"`
}

func (m UserChatModel) RuntimeConfig() ChatRuntimeConfig {
	return ChatRuntimeConfig{
		BaseURL:     m.BaseURL,
		APIKey:      m.APIKey,
		ModelName:   m.ModelName,
		Temperature: m.Temperature,
	}
}

type Skill struct {
	ID            string            `json:"id"`
	UserID        string            `json:"userId"`
	DefinitionID  string            `json:"definitionId,omitempty"`
	RevisionID    string            `json:"revisionId,omitempty"`
	Version       int               `json:"version,omitempty"`
	Slug          string            `json:"slug"`
	Kind          SkillKind         `json:"kind,omitempty"`
	Title         string            `json:"title"`
	Description   string            `json:"description"`
	Prompt        string            `json:"prompt"`
	Mode          string            `json:"mode"`
	PlannerPolicy map[string]string `json:"plannerPolicy,omitempty"`
	ToolAllowlist []string          `json:"toolAllowlist,omitempty"`
	Source        SkillSource       `json:"source"`
	Enabled       bool              `json:"enabled"`
	RepoURL       string            `json:"repoUrl,omitempty"`
	CreatedAt     time.Time         `json:"createdAt"`
	UpdatedAt     time.Time         `json:"updatedAt"`
}

type SkillDefinition struct {
	ID        string      `json:"id"`
	UserID    string      `json:"userId"`
	Slug      string      `json:"slug"`
	Kind      SkillKind   `json:"kind"`
	Source    SkillSource `json:"source"`
	RepoURL   string      `json:"repoUrl,omitempty"`
	CreatedAt time.Time   `json:"createdAt"`
	UpdatedAt time.Time   `json:"updatedAt"`
}

type SkillRevision struct {
	ID            string            `json:"id"`
	DefinitionID  string            `json:"definitionId"`
	Version       int               `json:"version"`
	Title         string            `json:"title"`
	Description   string            `json:"description"`
	Prompt        string            `json:"prompt"`
	Mode          string            `json:"mode"`
	PlannerPolicy map[string]string `json:"plannerPolicy,omitempty"`
	ToolAllowlist []string          `json:"toolAllowlist,omitempty"`
	ManifestJSON  string            `json:"-"`
	CreatedAt     time.Time         `json:"createdAt"`
}

type SkillInstallation struct {
	ID                string    `json:"id"`
	UserID            string    `json:"userId"`
	DefinitionID      string    `json:"definitionId"`
	CurrentRevisionID string    `json:"currentRevisionId"`
	Name              string    `json:"name"`
	IsDefault         bool      `json:"isDefault"`
	Enabled           bool      `json:"enabled"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

type SkillInstallationRecord struct {
	Installation    SkillInstallation `json:"installation"`
	Definition      SkillDefinition   `json:"definition"`
	CurrentRevision SkillRevision     `json:"currentRevision"`
}

type SkillDefinitionDetails struct {
	Definition    SkillDefinition     `json:"definition"`
	Revisions     []SkillRevision     `json:"revisions"`
	Installations []SkillInstallation `json:"installations"`
}

type SkillImportStatus string

const (
	SkillImportPending   SkillImportStatus = "pending"
	SkillImportCompleted SkillImportStatus = "completed"
	SkillImportFailed    SkillImportStatus = "failed"
)

type SkillArtifact struct {
	ID               string      `json:"id"`
	UserID           string      `json:"userId"`
	DefinitionID     string      `json:"definitionId,omitempty"`
	RevisionID       string      `json:"revisionId,omitempty"`
	Source           SkillSource `json:"source"`
	FileName         string      `json:"fileName"`
	MediaType        string      `json:"mediaType,omitempty"`
	SourceURL        string      `json:"sourceUrl,omitempty"`
	SHA256           string      `json:"sha256"`
	SizeBytes        int64       `json:"sizeBytes"`
	EntryPath        string      `json:"entryPath,omitempty"`
	ManifestPath     string      `json:"manifestPath,omitempty"`
	InstructionsPath string      `json:"instructionsPath,omitempty"`
	ArchiveBytes     []byte      `json:"-"`
	CreatedAt        time.Time   `json:"createdAt"`
}

type SkillArtifactFile struct {
	ID             string    `json:"id"`
	ArtifactID     string    `json:"artifactId"`
	UserID         string    `json:"userId"`
	Path           string    `json:"path"`
	MediaType      string    `json:"mediaType,omitempty"`
	SizeBytes      int64     `json:"sizeBytes"`
	SHA256         string    `json:"sha256"`
	IsManifest     bool      `json:"isManifest"`
	IsInstructions bool      `json:"isInstructions"`
	CreatedAt      time.Time `json:"createdAt"`
}

type SkillImportJob struct {
	ID             string            `json:"id"`
	UserID         string            `json:"userId"`
	Source         SkillSource       `json:"source"`
	Status         SkillImportStatus `json:"status"`
	ArtifactID     string            `json:"artifactId,omitempty"`
	DefinitionID   string            `json:"definitionId,omitempty"`
	RevisionID     string            `json:"revisionId,omitempty"`
	InstallationID string            `json:"installationId,omitempty"`
	ErrorMessage   string            `json:"errorMessage,omitempty"`
	RequestJSON    string            `json:"-"`
	CreatedAt      time.Time         `json:"createdAt"`
	UpdatedAt      time.Time         `json:"updatedAt"`
	CompletedAt    *time.Time        `json:"completedAt,omitempty"`
}

type SkillRuntimeSpec struct {
	SchemaVersion string              `json:"schemaVersion"`
	SkillKind     SkillKind           `json:"skillKind"`
	ResponseMode  string              `json:"responseMode"`
	Instructions  string              `json:"instructions"`
	PlannerPolicy map[string]string   `json:"plannerPolicy,omitempty"`
	ToolPolicy    map[string][]string `json:"toolPolicy,omitempty"`
	OutputPolicy  map[string]string   `json:"outputPolicy,omitempty"`
	Metadata      map[string]string   `json:"metadata,omitempty"`
}

type SkillRuntimeSnapshot struct {
	ID              string            `json:"id"`
	Scope           SkillRuntimeScope `json:"scope"`
	ScopeID         string            `json:"scopeId"`
	UserID          string            `json:"userId"`
	InstallationID  string            `json:"installationId,omitempty"`
	DefinitionID    string            `json:"definitionId"`
	RevisionID      string            `json:"revisionId"`
	Kind            SkillKind         `json:"kind"`
	Title           string            `json:"title"`
	Description     string            `json:"description"`
	Mode            string            `json:"mode"`
	Prompt          string            `json:"prompt"`
	RuntimeSpecJSON string            `json:"runtimeSpecJson"`
	CreatedAt       time.Time         `json:"createdAt"`
}

type KnowledgeConnection struct {
	ID           string                           `json:"id"`
	UserID       string                           `json:"userId"`
	Provider     Provider                         `json:"provider"`
	Name         string                           `json:"name"`
	SyncEnabled  bool                             `json:"syncEnabled"`
	LastSyncedAt *time.Time                       `json:"lastSyncedAt,omitempty"`
	Yuque        *KnowledgeConnectionYuqueConfig  `json:"yuque,omitempty"`
	Feishu       *KnowledgeConnectionFeishuConfig `json:"feishu,omitempty"`
	CreatedAt    time.Time                        `json:"createdAt"`
	UpdatedAt    time.Time                        `json:"updatedAt"`
}

type KnowledgeConnectionYuqueConfig struct {
	Token       string                               `json:"-"`
	GroupLogin  string                               `json:"groupLogin"`
	Namespace   string                               `json:"namespace,omitempty"`
	PendingDocs []KnowledgeConnectionYuquePendingDoc `json:"-"`
	CreatedAt   time.Time                            `json:"createdAt"`
	UpdatedAt   time.Time                            `json:"updatedAt"`
}

type KnowledgeConnectionYuquePendingDoc struct {
	ExternalID string    `json:"externalId"`
	Slug       string    `json:"slug"`
	Title      string    `json:"title"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type KnowledgeConnectionFeishuConfig struct {
	AppID      string    `json:"appId"`
	AppSecret  string    `json:"-"`
	EntryType  string    `json:"entryType"`
	EntryToken string    `json:"entryToken"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type KnowledgeSyncJob struct {
	ID           string        `json:"id"`
	UserID       string        `json:"userId"`
	ConnectionID string        `json:"connectionId"`
	Status       SyncJobStatus `json:"status"`
	Summary      string        `json:"summary"`
	CreatedAt    time.Time     `json:"createdAt"`
	StartedAt    *time.Time    `json:"startedAt,omitempty"`
	FinishedAt   *time.Time    `json:"finishedAt,omitempty"`
}

type KnowledgeDocument struct {
	ID           string    `json:"id"`
	UserID       string    `json:"userId"`
	ConnectionID string    `json:"connectionId"`
	Repo         string    `json:"repo"`
	Title        string    `json:"title"`
	DocRef       string    `json:"docRef"`
	SourceURL    string    `json:"sourceUrl,omitempty"`
	Body         string    `json:"-"`
	BodyHash     string    `json:"bodyHash"`
	UpdatedAt    time.Time `json:"updatedAt"`
	CreatedAt    time.Time `json:"createdAt"`
}

type KnowledgeDocumentMeta struct {
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

type KnowledgeSourceDocument struct {
	ID              string    `json:"id"`
	UserID          string    `json:"userId"`
	ConnectionID    string    `json:"connectionId"`
	Provider        Provider  `json:"provider"`
	ExternalID      string    `json:"externalId"`
	Repo            string    `json:"repo"`
	Title           string    `json:"title"`
	DocRef          string    `json:"docRef"`
	SourceURL       string    `json:"sourceUrl,omitempty"`
	RawBody         string    `json:"-"`
	BodyHash        string    `json:"bodyHash"`
	SourceUpdatedAt time.Time `json:"sourceUpdatedAt"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type KnowledgeChunk struct {
	ID           string    `json:"id"`
	UserID       string    `json:"userId"`
	ConnectionID string    `json:"connectionId"`
	DocumentID   string    `json:"documentId"`
	ChunkIndex   int       `json:"chunkIndex"`
	Content      string    `json:"content"`
	ContentHash  string    `json:"contentHash"`
	CreatedAt    time.Time `json:"createdAt"`
}

type KnowledgeHit struct {
	ConnectionID string  `json:"connectionId"`
	DocumentID   string  `json:"documentId"`
	ChunkID      string  `json:"chunkId"`
	Provider     string  `json:"provider"`
	Title        string  `json:"title"`
	Repo         string  `json:"repo"`
	URL          string  `json:"url,omitempty"`
	Snippet      string  `json:"snippet"`
	Score        float64 `json:"score"`
}

type MirrorTask struct {
	ID          string           `json:"id"`
	Kind        MirrorTaskKind   `json:"kind"`
	UserID      string           `json:"userId"`
	ResourceID  string           `json:"resourceId"`
	Status      MirrorTaskStatus `json:"status"`
	Payload     string           `json:"payload"`
	Attempts    int              `json:"attempts"`
	LastError   string           `json:"lastError,omitempty"`
	NextRetryAt time.Time        `json:"nextRetryAt"`
	CreatedAt   time.Time        `json:"createdAt"`
	UpdatedAt   time.Time        `json:"updatedAt"`
	CompletedAt *time.Time       `json:"completedAt,omitempty"`
}

type Run struct {
	ID                     string    `json:"id"`
	UserID                 string    `json:"userId"`
	Title                  string    `json:"title"`
	Goal                   string    `json:"goal"`
	RequestedMode          RunMode   `json:"requestedMode"`
	KnowledgeConnectionIDs []string  `json:"knowledgeConnectionIds,omitempty"`
	SkillInstallationID    string    `json:"skillInstallationId,omitempty"`
	SkillSnapshotID        string    `json:"skillSnapshotId,omitempty"`
	EffectiveMode          RunMode   `json:"effectiveMode"`
	Status                 RunStatus `json:"status"`
	LatestArtifactID       string    `json:"latestArtifactId,omitempty"`
	ErrorMessage           string    `json:"errorMessage,omitempty"`
	CreatedAt              time.Time `json:"createdAt"`
	UpdatedAt              time.Time `json:"updatedAt"`
}

type RunStep struct {
	ID         string     `json:"id"`
	RunID      string     `json:"runId"`
	Kind       StepKind   `json:"kind"`
	Label      string     `json:"label"`
	Status     StepStatus `json:"status"`
	Summary    string     `json:"summary"`
	StartedAt  *time.Time `json:"startedAt,omitempty"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
}

type RunArtifact struct {
	ID              string       `json:"id"`
	RunID           string       `json:"runId"`
	Kind            ArtifactKind `json:"kind"`
	ContentMarkdown string       `json:"contentMarkdown"`
	Version         int          `json:"version"`
	CreatedAt       time.Time    `json:"createdAt"`
}

type RunSource struct {
	ID           string    `json:"id"`
	RunID        string    `json:"runId"`
	Provider     Provider  `json:"provider"`
	ConnectionID string    `json:"connectionId,omitempty"`
	DocumentID   string    `json:"documentId,omitempty"`
	ChunkID      string    `json:"chunkId,omitempty"`
	Title        string    `json:"title"`
	Repo         string    `json:"repo,omitempty"`
	URL          string    `json:"url,omitempty"`
	Snippet      string    `json:"snippet"`
	Score        float64   `json:"score"`
	CreatedAt    time.Time `json:"createdAt"`
}

type RunDetails struct {
	Run       Run           `json:"run"`
	Steps     []RunStep     `json:"steps"`
	Artifacts []RunArtifact `json:"artifacts"`
	Sources   []RunSource   `json:"sources"`
}

type RunEvent struct {
	Type      string      `json:"type"`
	RunID     string      `json:"runId"`
	Timestamp time.Time   `json:"timestamp"`
	Payload   interface{} `json:"payload,omitempty"`
}

type Evidence struct {
	Provider     Provider `json:"provider"`
	ConnectionID string   `json:"connectionId,omitempty"`
	DocumentID   string   `json:"documentId,omitempty"`
	ChunkID      string   `json:"chunkId,omitempty"`
	Title        string   `json:"title"`
	Repo         string   `json:"repo,omitempty"`
	URL          string   `json:"url,omitempty"`
	Snippet      string   `json:"snippet"`
	Body         string   `json:"body,omitempty"`
	MatchedLines []string `json:"matchedAnswerLines,omitempty"`
	Score        float64  `json:"score"`
}
