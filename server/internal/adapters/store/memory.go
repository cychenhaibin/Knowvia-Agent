package store

import (
	"sync"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

type MemoryStore struct {
	mu sync.RWMutex

	memoryAuthState
	memoryChatState
	memorySkillState
	memoryRunState
	memoryKnowledgeState
	memoryMirrorState
}

type memoryAuthState struct {
	usersByID          map[string]domain.User
	usersByUsername    map[string]string
	authIdentities     map[string]domain.AuthIdentity
	authIdentityLookup map[string]string
	sessions           map[string]domain.Session
	sessionsByToken    map[string]string
	fluxACredentials   map[string]domain.FluxACredential
}

type memoryChatState struct {
	chatModels     map[string]domain.UserChatModel
	chatSessions   map[string]domain.ChatSession
	chatMessages   map[string][]domain.ChatMessage
	messageSources map[string][]domain.ChatMessageSource
}

type memorySkillState struct {
	skillDefinitions   map[string]domain.SkillDefinition
	skillRevisions     map[string]domain.SkillRevision
	skillInstallations map[string]domain.SkillInstallation
	skillSnapshots     map[string]domain.SkillRuntimeSnapshot
	skillArtifacts     map[string]domain.SkillArtifact
	skillArtifactFiles map[string][]domain.SkillArtifactFile
	skillImportJobs    map[string]domain.SkillImportJob
}

type memoryRunState struct {
	runs       map[string]domain.Run
	runSteps   map[string][]domain.RunStep
	artifacts  map[string][]domain.RunArtifact
	runSources map[string][]domain.RunSource
}

type memoryKnowledgeState struct {
	connections     map[string]domain.KnowledgeConnection
	syncJobs        map[string]domain.KnowledgeSyncJob
	metadata        map[string][]domain.KnowledgeDocumentMeta
	sourceDocuments map[string][]domain.KnowledgeSourceDocument
	documents       map[string][]domain.KnowledgeDocument
	chunks          map[string][]domain.KnowledgeChunk
}

type memoryMirrorState struct {
	mirrorTasks map[string]domain.MirrorTask
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		memoryAuthState: memoryAuthState{
			usersByID:          map[string]domain.User{},
			usersByUsername:    map[string]string{},
			authIdentities:     map[string]domain.AuthIdentity{},
			authIdentityLookup: map[string]string{},
			sessions:           map[string]domain.Session{},
			sessionsByToken:    map[string]string{},
			fluxACredentials:   map[string]domain.FluxACredential{},
		},
		memoryChatState: memoryChatState{
			chatModels:     map[string]domain.UserChatModel{},
			chatSessions:   map[string]domain.ChatSession{},
			chatMessages:   map[string][]domain.ChatMessage{},
			messageSources: map[string][]domain.ChatMessageSource{},
		},
		memorySkillState: memorySkillState{
			skillDefinitions:   map[string]domain.SkillDefinition{},
			skillRevisions:     map[string]domain.SkillRevision{},
			skillInstallations: map[string]domain.SkillInstallation{},
			skillSnapshots:     map[string]domain.SkillRuntimeSnapshot{},
			skillArtifacts:     map[string]domain.SkillArtifact{},
			skillArtifactFiles: map[string][]domain.SkillArtifactFile{},
			skillImportJobs:    map[string]domain.SkillImportJob{},
		},
		memoryRunState: memoryRunState{
			runs:       map[string]domain.Run{},
			runSteps:   map[string][]domain.RunStep{},
			artifacts:  map[string][]domain.RunArtifact{},
			runSources: map[string][]domain.RunSource{},
		},
		memoryKnowledgeState: memoryKnowledgeState{
			connections:     map[string]domain.KnowledgeConnection{},
			syncJobs:        map[string]domain.KnowledgeSyncJob{},
			metadata:        map[string][]domain.KnowledgeDocumentMeta{},
			sourceDocuments: map[string][]domain.KnowledgeSourceDocument{},
			documents:       map[string][]domain.KnowledgeDocument{},
			chunks:          map[string][]domain.KnowledgeChunk{},
		},
		memoryMirrorState: memoryMirrorState{
			mirrorTasks: map[string]domain.MirrorTask{},
		},
	}
}
