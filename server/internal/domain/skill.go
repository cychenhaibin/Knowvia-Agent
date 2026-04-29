package domain

import "time"

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

type Skill struct {
	ID            string
	UserID        string
	DefinitionID  string
	RevisionID    string
	Version       int
	Slug          string
	Kind          SkillKind
	Title         string
	Description   string
	Prompt        string
	Mode          string
	PlannerPolicy map[string]string
	ToolAllowlist []string
	Source        SkillSource
	Enabled       bool
	RepoURL       string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type SkillDefinition struct {
	ID        string
	UserID    string
	Slug      string
	Kind      SkillKind
	Source    SkillSource
	RepoURL   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type SkillRevision struct {
	ID            string
	DefinitionID  string
	Version       int
	Title         string
	Description   string
	Prompt        string
	Mode          string
	PlannerPolicy map[string]string
	ToolAllowlist []string
	ManifestJSON  string
	CreatedAt     time.Time
}

type SkillInstallation struct {
	ID                string
	UserID            string
	DefinitionID      string
	CurrentRevisionID string
	Name              string
	IsDefault         bool
	Enabled           bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type SkillInstallationRecord struct {
	Installation    SkillInstallation
	Definition      SkillDefinition
	CurrentRevision SkillRevision
}

type SkillDefinitionDetails struct {
	Definition    SkillDefinition
	Revisions     []SkillRevision
	Installations []SkillInstallation
}

type SkillImportStatus string

const (
	SkillImportPending   SkillImportStatus = "pending"
	SkillImportCompleted SkillImportStatus = "completed"
	SkillImportFailed    SkillImportStatus = "failed"
)

type SkillArtifact struct {
	ID               string
	UserID           string
	DefinitionID     string
	RevisionID       string
	Source           SkillSource
	FileName         string
	MediaType        string
	SourceURL        string
	SHA256           string
	SizeBytes        int64
	EntryPath        string
	ManifestPath     string
	InstructionsPath string
	ArchiveBytes     []byte
	CreatedAt        time.Time
}

type SkillArtifactFile struct {
	ID             string
	ArtifactID     string
	UserID         string
	Path           string
	MediaType      string
	SizeBytes      int64
	SHA256         string
	IsManifest     bool
	IsInstructions bool
	CreatedAt      time.Time
}

type SkillImportJob struct {
	ID             string
	UserID         string
	Source         SkillSource
	Status         SkillImportStatus
	ArtifactID     string
	DefinitionID   string
	RevisionID     string
	InstallationID string
	ErrorMessage   string
	RequestJSON    string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	CompletedAt    *time.Time
}

type SkillRuntimeSpec struct {
	SchemaVersion string
	SkillKind     SkillKind
	ResponseMode  string
	Instructions  string
	PlannerPolicy map[string]string
	ToolPolicy    map[string][]string
	OutputPolicy  map[string]string
	Metadata      map[string]string
}

type SkillRuntimeSnapshot struct {
	ID              string
	Scope           SkillRuntimeScope
	ScopeID         string
	UserID          string
	InstallationID  string
	DefinitionID    string
	RevisionID      string
	Kind            SkillKind
	Title           string
	Description     string
	Mode            string
	Prompt          string
	RuntimeSpecJSON string
	CreatedAt       time.Time
}
