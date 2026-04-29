package domain

import "time"

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

type MirrorTask struct {
	ID          string
	Kind        MirrorTaskKind
	UserID      string
	ResourceID  string
	Status      MirrorTaskStatus
	Payload     string
	Attempts    int
	LastError   string
	NextRetryAt time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
	CompletedAt *time.Time
}
