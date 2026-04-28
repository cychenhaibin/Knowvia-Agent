package store

import (
	"context"
	"errors"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

var ErrNotFound = errors.New("store: not found")
var ErrConflict = errors.New("store: conflict")

type Store interface {
	UpsertUser(ctx context.Context, user domain.User) error
	GetUserByUsername(ctx context.Context, username string) (domain.User, error)
	GetUserByID(ctx context.Context, userID string) (domain.User, error)
	GetUserByAuthIdentity(ctx context.Context, provider domain.AuthProvider, subject string) (domain.User, error)
	UpsertAuthIdentity(ctx context.Context, identity domain.AuthIdentity) error

	CreateSession(ctx context.Context, session domain.Session) error
	GetSessionByRefreshToken(ctx context.Context, refreshToken string) (domain.Session, error)
	RevokeSession(ctx context.Context, sessionID string) error
	EnsureUserChatModelDefaults(ctx context.Context, userID string) error
	ListUserChatModels(ctx context.Context, userID string) ([]domain.UserChatModel, error)
	GetUserChatModel(ctx context.Context, userID, modelID string) (domain.UserChatModel, error)
	GetSelectedUserChatModel(ctx context.Context, userID string, purpose domain.ChatModelPurpose) (domain.UserChatModel, error)
	CreateUserChatModel(ctx context.Context, model domain.UserChatModel) error
	UpdateUserChatModel(ctx context.Context, model domain.UserChatModel) error
	SelectUserChatModel(ctx context.Context, userID, modelID string) error
	DeleteUserChatModel(ctx context.Context, userID, modelID string) error

	CreateChatSession(ctx context.Context, session domain.ChatSession) error
	GetChatSession(ctx context.Context, userID, sessionID string) (domain.ChatSession, error)
	UpdateChatSession(ctx context.Context, session domain.ChatSession) error
	ListChatSessions(ctx context.Context, userID string) ([]domain.ChatSession, error)
	DeleteChatSession(ctx context.Context, userID, sessionID string) error

	SaveChatMessage(ctx context.Context, message domain.ChatMessage) error
	ListChatMessages(ctx context.Context, userID, sessionID string) ([]domain.ChatMessage, error)
	SaveChatMessageSources(ctx context.Context, messageID string, sources []domain.ChatMessageSource) error
	ListChatMessageSources(ctx context.Context, messageID string) ([]domain.ChatMessageSource, error)

	CreateSkill(ctx context.Context, skill domain.Skill) error
	CreateSkillDefinitionRevision(ctx context.Context, definition domain.SkillDefinition, revision domain.SkillRevision) error
	CreateSkillInstallation(ctx context.Context, installation domain.SkillInstallation) error
	UpdateSkill(ctx context.Context, skill domain.Skill) error
	GetSkill(ctx context.Context, userID, skillID string) (domain.Skill, error)
	ListSkills(ctx context.Context, userID string) ([]domain.Skill, error)
	GetSkillInstallationRecord(ctx context.Context, userID, installationID string) (domain.SkillInstallationRecord, error)
	GetDefaultSkillInstallationRecord(ctx context.Context, userID, definitionID string) (domain.SkillInstallationRecord, error)
	ListSkillInstallations(ctx context.Context, userID string) ([]domain.SkillInstallationRecord, error)
	UpdateSkillInstallation(ctx context.Context, installation domain.SkillInstallation) error
	ListSkillDefinitions(ctx context.Context, userID string) ([]domain.SkillDefinition, error)
	GetSkillDefinitionDetails(ctx context.Context, userID, definitionID string) (domain.SkillDefinitionDetails, error)
	ListSkillRevisions(ctx context.Context, userID, definitionID string) ([]domain.SkillRevision, error)
	DeleteSkill(ctx context.Context, userID, skillID string) error
	CreateSkillRuntimeSnapshot(ctx context.Context, snapshot domain.SkillRuntimeSnapshot) error
	CreateSkillArtifact(ctx context.Context, artifact domain.SkillArtifact) error
	GetSkillArtifact(ctx context.Context, userID, artifactID string) (domain.SkillArtifact, error)
	ReplaceSkillArtifactFiles(ctx context.Context, artifactID string, files []domain.SkillArtifactFile) error
	ListSkillArtifactFiles(ctx context.Context, userID, artifactID string) ([]domain.SkillArtifactFile, error)
	CreateSkillImportJob(ctx context.Context, job domain.SkillImportJob) error
	UpdateSkillImportJob(ctx context.Context, job domain.SkillImportJob) error
	GetSkillImportJob(ctx context.Context, userID, jobID string) (domain.SkillImportJob, error)
	ListSkillImportJobs(ctx context.Context, userID string) ([]domain.SkillImportJob, error)

	CreateRun(ctx context.Context, run domain.Run) error
	GetRun(ctx context.Context, userID, runID string) (domain.Run, error)
	GetRunByID(ctx context.Context, runID string) (domain.Run, error)
	UpdateRun(ctx context.Context, run domain.Run) error
	ListRuns(ctx context.Context, userID string) ([]domain.Run, error)

	UpsertRunStep(ctx context.Context, step domain.RunStep) error
	ListRunSteps(ctx context.Context, runID string) ([]domain.RunStep, error)

	SaveArtifact(ctx context.Context, artifact domain.RunArtifact) error
	ListArtifacts(ctx context.Context, runID string) ([]domain.RunArtifact, error)

	SaveSources(ctx context.Context, runID string, sources []domain.RunSource) error
	ListSources(ctx context.Context, runID string) ([]domain.RunSource, error)

	CreateKnowledgeConnection(ctx context.Context, connection domain.KnowledgeConnection) error
	UpdateKnowledgeConnection(ctx context.Context, connection domain.KnowledgeConnection) error
	GetKnowledgeConnection(ctx context.Context, userID, connectionID string) (domain.KnowledgeConnection, error)
	GetKnowledgeConnectionByID(ctx context.Context, connectionID string) (domain.KnowledgeConnection, error)
	ListKnowledgeConnections(ctx context.Context, userID string) ([]domain.KnowledgeConnection, error)
	DeleteKnowledgeConnection(ctx context.Context, userID, connectionID string) error

	CreateKnowledgeSyncJob(ctx context.Context, job domain.KnowledgeSyncJob) error
	UpdateKnowledgeSyncJob(ctx context.Context, job domain.KnowledgeSyncJob) error
	GetKnowledgeSyncJob(ctx context.Context, jobID string) (domain.KnowledgeSyncJob, error)
	ListKnowledgeSyncJobs(ctx context.Context, userID, connectionID string) ([]domain.KnowledgeSyncJob, error)

	ListKnowledgeMetadata(ctx context.Context, connectionID string) ([]domain.KnowledgeDocumentMeta, error)
	ReplaceKnowledgeMetadata(ctx context.Context, connectionID string, docs []domain.KnowledgeDocumentMeta) error
	ListKnowledgeSourceDocuments(ctx context.Context, connectionID string) ([]domain.KnowledgeSourceDocument, error)
	ReplaceKnowledgeSourceDocuments(ctx context.Context, connectionID string, docs []domain.KnowledgeSourceDocument) error
	GetKnowledgeCorpus(ctx context.Context, connectionID string) ([]domain.KnowledgeDocument, []domain.KnowledgeChunk, error)
	ReplaceKnowledgeCorpus(ctx context.Context, connectionID string, docs []domain.KnowledgeDocument, chunks []domain.KnowledgeChunk) error
	SearchKnowledge(ctx context.Context, userID string, connectionIDs []string, query string, limit int) ([]domain.KnowledgeHit, error)

	CreateMirrorTask(ctx context.Context, task domain.MirrorTask) error
	UpdateMirrorTask(ctx context.Context, task domain.MirrorTask) error
	GetMirrorTask(ctx context.Context, taskID string) (domain.MirrorTask, error)
	ListDueMirrorTasks(ctx context.Context, dueBefore time.Time, limit int) ([]domain.MirrorTask, error)
}
