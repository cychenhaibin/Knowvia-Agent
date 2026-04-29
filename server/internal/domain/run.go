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

type Run struct {
	ID                     string
	UserID                 string
	Title                  string
	Goal                   string
	RequestedMode          RunMode
	KnowledgeConnectionIDs []string
	SkillInstallationID    string
	SkillSnapshotID        string
	EffectiveMode          RunMode
	Status                 RunStatus
	LatestArtifactID       string
	ErrorMessage           string
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type RunStep struct {
	ID         string
	RunID      string
	Kind       StepKind
	Label      string
	Status     StepStatus
	Summary    string
	StartedAt  *time.Time
	FinishedAt *time.Time
	CreatedAt  time.Time
}

type RunArtifact struct {
	ID              string
	RunID           string
	Kind            ArtifactKind
	ContentMarkdown string
	Version         int
	CreatedAt       time.Time
}

type RunSource struct {
	ID           string
	RunID        string
	Provider     Provider
	ConnectionID string
	DocumentID   string
	ChunkID      string
	Title        string
	Repo         string
	URL          string
	Snippet      string
	Score        float64
	CreatedAt    time.Time
}

type RunDetails struct {
	Run       Run
	Steps     []RunStep
	Artifacts []RunArtifact
	Sources   []RunSource
}

type RunEvent struct {
	Type      string
	RunID     string
	Timestamp time.Time
	Payload   interface{}
}

type Evidence struct {
	Provider     Provider
	ConnectionID string
	DocumentID   string
	ChunkID      string
	Title        string
	Repo         string
	URL          string
	Snippet      string
	Body         string
	MatchedLines []string
	Score        float64
}
