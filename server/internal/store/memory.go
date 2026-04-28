package store

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skillruntime"
)

type MemoryStore struct {
	mu sync.RWMutex

	usersByID          map[string]domain.User
	usersByUsername    map[string]string
	authIdentities     map[string]domain.AuthIdentity
	authIdentityLookup map[string]string
	sessions           map[string]domain.Session
	sessionsByToken    map[string]string
	chatModels         map[string]domain.UserChatModel
	chatSessions       map[string]domain.ChatSession
	chatMessages       map[string][]domain.ChatMessage
	messageSources     map[string][]domain.ChatMessageSource
	skillDefinitions   map[string]domain.SkillDefinition
	skillRevisions     map[string]domain.SkillRevision
	skillInstallations map[string]domain.SkillInstallation
	skillSnapshots     map[string]domain.SkillRuntimeSnapshot
	skillArtifacts     map[string]domain.SkillArtifact
	skillArtifactFiles map[string][]domain.SkillArtifactFile
	skillImportJobs    map[string]domain.SkillImportJob

	runs       map[string]domain.Run
	runSteps   map[string][]domain.RunStep
	artifacts  map[string][]domain.RunArtifact
	runSources map[string][]domain.RunSource

	connections     map[string]domain.KnowledgeConnection
	syncJobs        map[string]domain.KnowledgeSyncJob
	metadata        map[string][]domain.KnowledgeDocumentMeta
	sourceDocuments map[string][]domain.KnowledgeSourceDocument
	documents       map[string][]domain.KnowledgeDocument
	chunks          map[string][]domain.KnowledgeChunk
	mirrorTasks     map[string]domain.MirrorTask
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		usersByID:          map[string]domain.User{},
		usersByUsername:    map[string]string{},
		authIdentities:     map[string]domain.AuthIdentity{},
		authIdentityLookup: map[string]string{},
		sessions:           map[string]domain.Session{},
		sessionsByToken:    map[string]string{},
		chatModels:         map[string]domain.UserChatModel{},
		chatSessions:       map[string]domain.ChatSession{},
		chatMessages:       map[string][]domain.ChatMessage{},
		messageSources:     map[string][]domain.ChatMessageSource{},
		skillDefinitions:   map[string]domain.SkillDefinition{},
		skillRevisions:     map[string]domain.SkillRevision{},
		skillInstallations: map[string]domain.SkillInstallation{},
		skillSnapshots:     map[string]domain.SkillRuntimeSnapshot{},
		skillArtifacts:     map[string]domain.SkillArtifact{},
		skillArtifactFiles: map[string][]domain.SkillArtifactFile{},
		skillImportJobs:    map[string]domain.SkillImportJob{},
		runs:               map[string]domain.Run{},
		runSteps:           map[string][]domain.RunStep{},
		artifacts:          map[string][]domain.RunArtifact{},
		runSources:         map[string][]domain.RunSource{},
		connections:        map[string]domain.KnowledgeConnection{},
		syncJobs:           map[string]domain.KnowledgeSyncJob{},
		metadata:           map[string][]domain.KnowledgeDocumentMeta{},
		sourceDocuments:    map[string][]domain.KnowledgeSourceDocument{},
		documents:          map[string][]domain.KnowledgeDocument{},
		chunks:             map[string][]domain.KnowledgeChunk{},
		mirrorTasks:        map[string]domain.MirrorTask{},
	}
}

func authIdentityKey(provider domain.AuthProvider, subject string) string {
	return string(provider) + ":" + strings.TrimSpace(subject)
}

func (s *MemoryStore) UpsertUser(_ context.Context, user domain.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.usersByID[user.ID]; ok {
		if existing.Username != "" {
			delete(s.usersByUsername, existing.Username)
		}
	}
	s.usersByID[user.ID] = user
	s.usersByUsername[user.Username] = user.ID
	return nil
}

func (s *MemoryStore) GetUserByUsername(_ context.Context, username string) (domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	userID, ok := s.usersByUsername[username]
	if !ok {
		return domain.User{}, ErrNotFound
	}
	return s.usersByID[userID], nil
}

func (s *MemoryStore) GetUserByID(_ context.Context, userID string) (domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.usersByID[userID]
	if !ok {
		return domain.User{}, ErrNotFound
	}
	return user, nil
}

func (s *MemoryStore) GetUserByAuthIdentity(_ context.Context, provider domain.AuthProvider, subject string) (domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	identityID, ok := s.authIdentityLookup[authIdentityKey(provider, subject)]
	if !ok {
		return domain.User{}, ErrNotFound
	}
	identity, ok := s.authIdentities[identityID]
	if !ok {
		return domain.User{}, ErrNotFound
	}
	user, ok := s.usersByID[identity.UserID]
	if !ok {
		return domain.User{}, ErrNotFound
	}
	return user, nil
}

func (s *MemoryStore) UpsertAuthIdentity(_ context.Context, identity domain.AuthIdentity) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := authIdentityKey(identity.Provider, identity.ProviderSubject)
	if existingID, ok := s.authIdentityLookup[key]; ok && existingID != identity.ID {
		return ErrConflict
	}
	if existing, ok := s.authIdentities[identity.ID]; ok {
		delete(s.authIdentityLookup, authIdentityKey(existing.Provider, existing.ProviderSubject))
	}
	s.authIdentities[identity.ID] = identity
	s.authIdentityLookup[key] = identity.ID
	return nil
}

func (s *MemoryStore) CreateSession(_ context.Context, session domain.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[session.ID] = session
	s.sessionsByToken[session.RefreshToken] = session.ID
	return nil
}

func (s *MemoryStore) GetSessionByRefreshToken(_ context.Context, refreshToken string) (domain.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sessionID, ok := s.sessionsByToken[refreshToken]
	if !ok {
		return domain.Session{}, ErrNotFound
	}
	return s.sessions[sessionID], nil
}

func (s *MemoryStore) RevokeSession(_ context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[sessionID]
	if !ok {
		return ErrNotFound
	}
	now := time.Now().UTC()
	session.RevokedAt = &now
	s.sessions[sessionID] = session
	return nil
}

func (s *MemoryStore) EnsureUserChatModelDefaults(_ context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	s.ensureDefaultChatModelsLocked(userID, now)
	return nil
}

func (s *MemoryStore) ListUserChatModels(_ context.Context, userID string) ([]domain.UserChatModel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureDefaultChatModelsLocked(userID, time.Now().UTC())
	items := make([]domain.UserChatModel, 0)
	for _, model := range s.chatModels {
		if model.UserID == userID {
			items = append(items, model)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Purpose != items[j].Purpose {
			return items[i].Purpose < items[j].Purpose
		}
		if items[i].IsSelected != items[j].IsSelected {
			return items[i].IsSelected
		}
		if !items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].CreatedAt.Before(items[j].CreatedAt)
		}
		return items[i].Name < items[j].Name
	})
	return append([]domain.UserChatModel(nil), items...), nil
}

func (s *MemoryStore) GetUserChatModel(_ context.Context, userID, modelID string) (domain.UserChatModel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	model, ok := s.chatModels[modelID]
	if !ok || model.UserID != userID {
		return domain.UserChatModel{}, ErrNotFound
	}
	return model, nil
}

func (s *MemoryStore) GetSelectedUserChatModel(_ context.Context, userID string, purpose domain.ChatModelPurpose) (domain.UserChatModel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureDefaultChatModelsLocked(userID, time.Now().UTC())
	for _, model := range s.chatModels {
		if model.UserID == userID && model.Purpose == purpose && model.IsSelected {
			return model, nil
		}
	}
	return domain.UserChatModel{}, ErrNotFound
}

func (s *MemoryStore) CreateUserChatModel(_ context.Context, model domain.UserChatModel) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureDefaultChatModelsLocked(model.UserID, time.Now().UTC())
	for _, existing := range s.chatModels {
		if existing.UserID == model.UserID &&
			existing.Purpose == model.Purpose &&
			strings.EqualFold(existing.Name, model.Name) {
			return ErrConflict
		}
	}
	if !s.hasSelectedChatModelLocked(model.UserID, model.Purpose) {
		model.IsSelected = true
	}
	s.chatModels[model.ID] = model
	return nil
}

func (s *MemoryStore) UpdateUserChatModel(_ context.Context, model domain.UserChatModel) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.chatModels[model.ID]
	if !ok || existing.UserID != model.UserID {
		return ErrNotFound
	}
	for _, item := range s.chatModels {
		if item.ID == model.ID {
			continue
		}
		if item.UserID == model.UserID &&
			item.Purpose == model.Purpose &&
			strings.EqualFold(item.Name, model.Name) {
			return ErrConflict
		}
	}
	s.chatModels[model.ID] = model
	return nil
}

func (s *MemoryStore) SelectUserChatModel(_ context.Context, userID, modelID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	model, ok := s.chatModels[modelID]
	if !ok || model.UserID != userID {
		return ErrNotFound
	}
	now := time.Now().UTC()
	for id, existing := range s.chatModels {
		if existing.UserID != userID || existing.Purpose != model.Purpose {
			continue
		}
		existing.IsSelected = id == modelID
		existing.UpdatedAt = now
		s.chatModels[id] = existing
	}
	return nil
}

func (s *MemoryStore) DeleteUserChatModel(_ context.Context, userID, modelID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	model, ok := s.chatModels[modelID]
	if !ok || model.UserID != userID {
		return ErrNotFound
	}
	if model.Origin == domain.ChatModelOriginDefault {
		return ErrConflict
	}
	remaining := make([]domain.UserChatModel, 0)
	for id, existing := range s.chatModels {
		if id == modelID {
			continue
		}
		if existing.UserID == userID && existing.Purpose == model.Purpose {
			remaining = append(remaining, existing)
		}
	}
	if len(remaining) == 0 {
		return ErrConflict
	}
	delete(s.chatModels, modelID)
	if model.IsSelected {
		sort.Slice(remaining, func(i, j int) bool {
			if !remaining[i].CreatedAt.Equal(remaining[j].CreatedAt) {
				return remaining[i].CreatedAt.Before(remaining[j].CreatedAt)
			}
			return remaining[i].Name < remaining[j].Name
		})
		fallback := remaining[0]
		fallback.IsSelected = true
		fallback.UpdatedAt = time.Now().UTC()
		s.chatModels[fallback.ID] = fallback
	}
	return nil
}

func (s *MemoryStore) CreateChatSession(_ context.Context, session domain.ChatSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.chatSessions[session.ID] = session
	return nil
}

func (s *MemoryStore) GetChatSession(_ context.Context, userID, sessionID string) (domain.ChatSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.chatSessions[sessionID]
	if !ok || session.UserID != userID {
		return domain.ChatSession{}, ErrNotFound
	}
	return session, nil
}

func (s *MemoryStore) UpdateChatSession(_ context.Context, session domain.ChatSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.chatSessions[session.ID]; !ok {
		return ErrNotFound
	}
	s.chatSessions[session.ID] = session
	return nil
}

func (s *MemoryStore) ListChatSessions(_ context.Context, userID string) ([]domain.ChatSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sessions := []domain.ChatSession{}
	for _, session := range s.chatSessions {
		if session.UserID == userID {
			sessions = append(sessions, session)
		}
	}
	sort.Slice(sessions, func(i, j int) bool {
		if sessions[i].Pinned != sessions[j].Pinned {
			return sessions[i].Pinned
		}
		left := sessions[i].UpdatedAt
		if sessions[i].LastMessageAt != nil {
			left = *sessions[i].LastMessageAt
		}
		right := sessions[j].UpdatedAt
		if sessions[j].LastMessageAt != nil {
			right = *sessions[j].LastMessageAt
		}
		return left.After(right)
	})
	return sessions, nil
}

func (s *MemoryStore) DeleteChatSession(_ context.Context, userID, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.chatSessions[sessionID]
	if !ok || session.UserID != userID {
		return ErrNotFound
	}
	messages := append([]domain.ChatMessage(nil), s.chatMessages[sessionID]...)
	delete(s.chatSessions, sessionID)
	delete(s.chatMessages, sessionID)
	for _, message := range messages {
		delete(s.messageSources, message.ID)
	}
	return nil
}

func (s *MemoryStore) SaveChatMessage(_ context.Context, message domain.ChatMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	messages := s.chatMessages[message.SessionID]
	for i := range messages {
		if messages[i].ID == message.ID {
			messages[i] = message
			s.chatMessages[message.SessionID] = messages
			return nil
		}
	}
	s.chatMessages[message.SessionID] = append(messages, message)
	return nil
}

func (s *MemoryStore) ListChatMessages(_ context.Context, userID, sessionID string) ([]domain.ChatMessage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.chatSessions[sessionID]
	if !ok || session.UserID != userID {
		return nil, ErrNotFound
	}
	messages := append([]domain.ChatMessage(nil), s.chatMessages[sessionID]...)
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].CreatedAt.Before(messages[j].CreatedAt)
	})
	return messages, nil
}

func (s *MemoryStore) SaveChatMessageSources(_ context.Context, messageID string, sources []domain.ChatMessageSource) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messageSources[messageID] = append([]domain.ChatMessageSource(nil), sources...)
	return nil
}

func (s *MemoryStore) ListChatMessageSources(_ context.Context, messageID string) ([]domain.ChatMessageSource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sources := append([]domain.ChatMessageSource(nil), s.messageSources[messageID]...)
	sort.Slice(sources, func(i, j int) bool {
		return sources[i].CreatedAt.Before(sources[j].CreatedAt)
	})
	return sources, nil
}

func (s *MemoryStore) CreateSkill(_ context.Context, skill domain.Skill) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	manifestJSON, err := skillruntime.EncodeRevisionManifest(skill)
	if err != nil {
		return err
	}
	s.skillDefinitions[skill.DefinitionID] = domain.SkillDefinition{
		ID:        skill.DefinitionID,
		UserID:    skill.UserID,
		Slug:      skill.Slug,
		Kind:      skill.Kind,
		Source:    skill.Source,
		RepoURL:   skill.RepoURL,
		CreatedAt: skill.CreatedAt,
		UpdatedAt: skill.UpdatedAt,
	}
	s.skillRevisions[skill.RevisionID] = domain.SkillRevision{
		ID:            skill.RevisionID,
		DefinitionID:  skill.DefinitionID,
		Version:       skill.Version,
		Title:         skill.Title,
		Description:   skill.Description,
		Prompt:        skill.Prompt,
		Mode:          skill.Mode,
		PlannerPolicy: cloneStringMap(skill.PlannerPolicy),
		ToolAllowlist: append([]string(nil), skill.ToolAllowlist...),
		ManifestJSON:  manifestJSON,
		CreatedAt:     skill.UpdatedAt,
	}
	s.skillInstallations[skill.ID] = domain.SkillInstallation{
		ID:                skill.ID,
		UserID:            skill.UserID,
		DefinitionID:      skill.DefinitionID,
		CurrentRevisionID: skill.RevisionID,
		Name:              skill.Title,
		IsDefault:         true,
		Enabled:           skill.Enabled,
		CreatedAt:         skill.CreatedAt,
		UpdatedAt:         skill.UpdatedAt,
	}
	return nil
}

func (s *MemoryStore) CreateSkillDefinitionRevision(_ context.Context, definition domain.SkillDefinition, revision domain.SkillRevision) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.skillDefinitions {
		if existing.UserID == definition.UserID && existing.Slug == definition.Slug {
			return ErrConflict
		}
	}
	if strings.TrimSpace(revision.ManifestJSON) == "" {
		revision.ManifestJSON = "{}"
	}
	s.skillDefinitions[definition.ID] = definition
	s.skillRevisions[revision.ID] = domain.SkillRevision{
		ID:            revision.ID,
		DefinitionID:  revision.DefinitionID,
		Version:       revision.Version,
		Title:         revision.Title,
		Description:   revision.Description,
		Prompt:        revision.Prompt,
		Mode:          revision.Mode,
		PlannerPolicy: cloneStringMap(revision.PlannerPolicy),
		ToolAllowlist: append([]string(nil), revision.ToolAllowlist...),
		ManifestJSON:  revision.ManifestJSON,
		CreatedAt:     revision.CreatedAt,
	}
	return nil
}

func (s *MemoryStore) CreateSkillInstallation(_ context.Context, installation domain.SkillInstallation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	definition, ok := s.skillDefinitions[installation.DefinitionID]
	if !ok || definition.UserID != installation.UserID {
		return ErrNotFound
	}
	revision, ok := s.skillRevisions[installation.CurrentRevisionID]
	if !ok || revision.DefinitionID != installation.DefinitionID {
		return ErrNotFound
	}
	if strings.TrimSpace(installation.Name) == "" {
		installation.Name = revision.Title
	}
	if !installation.IsDefault && !s.definitionHasInstallationsForUser(installation.UserID, installation.DefinitionID) {
		installation.IsDefault = true
	}
	if installation.IsDefault {
		s.clearDefaultSkillInstallations(installation.UserID, installation.DefinitionID, installation.ID)
	}
	s.skillInstallations[installation.ID] = installation
	return nil
}

func (s *MemoryStore) UpdateSkill(_ context.Context, skill domain.Skill) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	installation, ok := s.skillInstallations[skill.ID]
	if !ok {
		return ErrNotFound
	}
	definition, ok := s.skillDefinitions[installation.DefinitionID]
	if !ok {
		return ErrNotFound
	}
	definition.Slug = skill.Slug
	definition.Kind = skill.Kind
	definition.Source = skill.Source
	definition.RepoURL = skill.RepoURL
	definition.UpdatedAt = skill.UpdatedAt
	s.skillDefinitions[definition.ID] = definition
	manifestJSON, err := skillruntime.EncodeRevisionManifest(skill)
	if err != nil {
		return err
	}
	s.skillRevisions[skill.RevisionID] = domain.SkillRevision{
		ID:            skill.RevisionID,
		DefinitionID:  definition.ID,
		Version:       skill.Version,
		Title:         skill.Title,
		Description:   skill.Description,
		Prompt:        skill.Prompt,
		Mode:          skill.Mode,
		PlannerPolicy: cloneStringMap(skill.PlannerPolicy),
		ToolAllowlist: append([]string(nil), skill.ToolAllowlist...),
		ManifestJSON:  manifestJSON,
		CreatedAt:     skill.UpdatedAt,
	}
	installation.CurrentRevisionID = skill.RevisionID
	installation.Enabled = skill.Enabled
	installation.UpdatedAt = skill.UpdatedAt
	s.skillInstallations[installation.ID] = installation
	return nil
}

func (s *MemoryStore) GetSkill(_ context.Context, userID, skillID string) (domain.Skill, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	skill, err := s.materializeSkill(skillID)
	if err != nil || skill.UserID != userID {
		return domain.Skill{}, ErrNotFound
	}
	return skill, nil
}

func (s *MemoryStore) ListSkills(_ context.Context, userID string) ([]domain.Skill, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	skills := []domain.Skill{}
	for installationID, installation := range s.skillInstallations {
		if installation.UserID != userID {
			continue
		}
		skill, err := s.materializeSkill(installationID)
		if err == nil {
			skills = append(skills, skill)
		}
	}
	sort.Slice(skills, func(i, j int) bool {
		return skills[i].UpdatedAt.After(skills[j].UpdatedAt)
	})
	return skills, nil
}

func (s *MemoryStore) GetSkillInstallationRecord(_ context.Context, userID, installationID string) (domain.SkillInstallationRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, err := s.materializeSkillInstallationRecord(installationID)
	if err != nil || record.Installation.UserID != userID {
		return domain.SkillInstallationRecord{}, ErrNotFound
	}
	return record, nil
}

func (s *MemoryStore) GetDefaultSkillInstallationRecord(_ context.Context, userID, definitionID string) (domain.SkillInstallationRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var selectedID string
	var selected domain.SkillInstallation
	for installationID, installation := range s.skillInstallations {
		if installation.UserID != userID || installation.DefinitionID != definitionID {
			continue
		}
		if selectedID == "" || installation.IsDefault {
			selectedID = installationID
			selected = installation
			if installation.IsDefault {
				break
			}
		}
	}
	if selectedID == "" {
		return domain.SkillInstallationRecord{}, ErrNotFound
	}
	record, err := s.materializeSkillInstallationRecord(selected.ID)
	if err != nil {
		return domain.SkillInstallationRecord{}, err
	}
	return record, nil
}

func (s *MemoryStore) ListSkillInstallations(_ context.Context, userID string) ([]domain.SkillInstallationRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]domain.SkillInstallationRecord, 0, len(s.skillInstallations))
	for installationID, installation := range s.skillInstallations {
		if installation.UserID != userID {
			continue
		}
		record, err := s.materializeSkillInstallationRecord(installationID)
		if err == nil {
			items = append(items, record)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Installation.UpdatedAt.After(items[j].Installation.UpdatedAt)
	})
	return items, nil
}

func (s *MemoryStore) UpdateSkillInstallation(_ context.Context, installation domain.SkillInstallation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.skillInstallations[installation.ID]
	if !ok || current.UserID != installation.UserID {
		return ErrNotFound
	}
	revision, ok := s.skillRevisions[installation.CurrentRevisionID]
	if !ok || revision.DefinitionID != current.DefinitionID {
		return ErrNotFound
	}
	if strings.TrimSpace(installation.Name) == "" {
		installation.Name = revision.Title
	}
	if installation.IsDefault {
		s.clearDefaultSkillInstallations(installation.UserID, installation.DefinitionID, installation.ID)
	}
	if current.IsDefault && !installation.IsDefault {
		if s.hasSiblingSkillInstallation(current.UserID, current.DefinitionID, current.ID) {
			s.promoteDefaultSkillInstallation(current.UserID, current.DefinitionID, current.ID)
		} else {
			installation.IsDefault = true
		}
	}
	current.CurrentRevisionID = installation.CurrentRevisionID
	current.Name = installation.Name
	current.IsDefault = installation.IsDefault
	current.Enabled = installation.Enabled
	current.UpdatedAt = installation.UpdatedAt
	s.skillInstallations[current.ID] = current
	return nil
}

func (s *MemoryStore) ListSkillDefinitions(_ context.Context, userID string) ([]domain.SkillDefinition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]domain.SkillDefinition, 0, len(s.skillDefinitions))
	for _, definition := range s.skillDefinitions {
		if definition.UserID != userID {
			continue
		}
		items = append(items, definition)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	return items, nil
}

func (s *MemoryStore) GetSkillDefinitionDetails(_ context.Context, userID, definitionID string) (domain.SkillDefinitionDetails, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	definition, ok := s.skillDefinitions[definitionID]
	if !ok || definition.UserID != userID {
		return domain.SkillDefinitionDetails{}, ErrNotFound
	}

	revisions := make([]domain.SkillRevision, 0)
	for _, revision := range s.skillRevisions {
		if revision.DefinitionID != definitionID {
			continue
		}
		item := revision
		if err := skillruntime.ApplyRevisionManifestToRevision(&item, revision.ManifestJSON); err != nil {
			return domain.SkillDefinitionDetails{}, err
		}
		revisions = append(revisions, item)
	}
	sort.Slice(revisions, func(i, j int) bool {
		if revisions[i].Version == revisions[j].Version {
			return revisions[i].CreatedAt.After(revisions[j].CreatedAt)
		}
		return revisions[i].Version > revisions[j].Version
	})

	installations := make([]domain.SkillInstallation, 0)
	for _, installation := range s.skillInstallations {
		if installation.UserID != userID || installation.DefinitionID != definitionID {
			continue
		}
		installations = append(installations, installation)
	}
	sort.Slice(installations, func(i, j int) bool {
		return installations[i].UpdatedAt.After(installations[j].UpdatedAt)
	})

	return domain.SkillDefinitionDetails{
		Definition:    definition,
		Revisions:     revisions,
		Installations: installations,
	}, nil
}

func (s *MemoryStore) ListSkillRevisions(_ context.Context, userID, definitionID string) ([]domain.SkillRevision, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	definition, ok := s.skillDefinitions[definitionID]
	if !ok || definition.UserID != userID {
		return nil, ErrNotFound
	}
	items := make([]domain.SkillRevision, 0)
	for _, revision := range s.skillRevisions {
		if revision.DefinitionID != definitionID {
			continue
		}
		item := revision
		if err := skillruntime.ApplyRevisionManifestToRevision(&item, revision.ManifestJSON); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Version == items[j].Version {
			return items[i].CreatedAt.After(items[j].CreatedAt)
		}
		return items[i].Version > items[j].Version
	})
	return items, nil
}

func (s *MemoryStore) DeleteSkill(_ context.Context, userID, skillID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	installation, ok := s.skillInstallations[skillID]
	if !ok || installation.UserID != userID {
		return ErrNotFound
	}
	delete(s.skillInstallations, skillID)
	if installation.IsDefault {
		s.promoteDefaultSkillInstallation(userID, installation.DefinitionID, "")
	}
	return nil
}

func (s *MemoryStore) CreateSkillRuntimeSnapshot(_ context.Context, snapshot domain.SkillRuntimeSnapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.skillSnapshots[snapshot.ID] = snapshot
	return nil
}

func (s *MemoryStore) CreateSkillArtifact(_ context.Context, artifact domain.SkillArtifact) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.skillArtifacts[artifact.ID] = cloneSkillArtifact(artifact)
	return nil
}

func (s *MemoryStore) GetSkillArtifact(_ context.Context, userID, artifactID string) (domain.SkillArtifact, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	artifact, ok := s.skillArtifacts[artifactID]
	if !ok || artifact.UserID != userID {
		return domain.SkillArtifact{}, ErrNotFound
	}
	return cloneSkillArtifact(artifact), nil
}

func (s *MemoryStore) ReplaceSkillArtifactFiles(_ context.Context, artifactID string, files []domain.SkillArtifactFile) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	artifact, ok := s.skillArtifacts[artifactID]
	if !ok {
		return ErrNotFound
	}
	items := make([]domain.SkillArtifactFile, 0, len(files))
	for _, file := range files {
		file.ArtifactID = artifactID
		file.UserID = artifact.UserID
		items = append(items, file)
	}
	s.skillArtifactFiles[artifactID] = items
	return nil
}

func (s *MemoryStore) ListSkillArtifactFiles(_ context.Context, userID, artifactID string) ([]domain.SkillArtifactFile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	artifact, ok := s.skillArtifacts[artifactID]
	if !ok || artifact.UserID != userID {
		return nil, ErrNotFound
	}
	items := append([]domain.SkillArtifactFile(nil), s.skillArtifactFiles[artifactID]...)
	sort.Slice(items, func(i, j int) bool {
		return items[i].Path < items[j].Path
	})
	return items, nil
}

func (s *MemoryStore) CreateSkillImportJob(_ context.Context, job domain.SkillImportJob) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.skillImportJobs[job.ID] = job
	return nil
}

func (s *MemoryStore) UpdateSkillImportJob(_ context.Context, job domain.SkillImportJob) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.skillImportJobs[job.ID]; !ok {
		return ErrNotFound
	}
	s.skillImportJobs[job.ID] = job
	return nil
}

func (s *MemoryStore) GetSkillImportJob(_ context.Context, userID, jobID string) (domain.SkillImportJob, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.skillImportJobs[jobID]
	if !ok || job.UserID != userID {
		return domain.SkillImportJob{}, ErrNotFound
	}
	return job, nil
}

func (s *MemoryStore) ListSkillImportJobs(_ context.Context, userID string) ([]domain.SkillImportJob, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]domain.SkillImportJob, 0, len(s.skillImportJobs))
	for _, job := range s.skillImportJobs {
		if job.UserID == userID {
			items = append(items, job)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	return items, nil
}

func (s *MemoryStore) materializeSkill(installationID string) (domain.Skill, error) {
	installation, ok := s.skillInstallations[installationID]
	if !ok {
		return domain.Skill{}, ErrNotFound
	}
	definition, ok := s.skillDefinitions[installation.DefinitionID]
	if !ok {
		return domain.Skill{}, ErrNotFound
	}
	revision, ok := s.skillRevisions[installation.CurrentRevisionID]
	if !ok {
		return domain.Skill{}, ErrNotFound
	}
	skill := domain.Skill{
		ID:           installation.ID,
		UserID:       installation.UserID,
		DefinitionID: definition.ID,
		RevisionID:   revision.ID,
		Version:      revision.Version,
		Slug:         definition.Slug,
		Kind:         definition.Kind,
		Title:        revision.Title,
		Description:  revision.Description,
		Prompt:       revision.Prompt,
		Mode:         revision.Mode,
		Source:       definition.Source,
		Enabled:      installation.Enabled,
		RepoURL:      definition.RepoURL,
		CreatedAt:    installation.CreatedAt,
		UpdatedAt:    installation.UpdatedAt,
	}
	if err := skillruntime.ApplyRevisionManifest(&skill, revision.ManifestJSON); err != nil {
		return domain.Skill{}, err
	}
	return skill, nil
}

func (s *MemoryStore) materializeSkillInstallationRecord(installationID string) (domain.SkillInstallationRecord, error) {
	installation, ok := s.skillInstallations[installationID]
	if !ok {
		return domain.SkillInstallationRecord{}, ErrNotFound
	}
	definition, ok := s.skillDefinitions[installation.DefinitionID]
	if !ok {
		return domain.SkillInstallationRecord{}, ErrNotFound
	}
	revision, ok := s.skillRevisions[installation.CurrentRevisionID]
	if !ok {
		return domain.SkillInstallationRecord{}, ErrNotFound
	}
	currentRevision := revision
	if err := skillruntime.ApplyRevisionManifestToRevision(&currentRevision, revision.ManifestJSON); err != nil {
		return domain.SkillInstallationRecord{}, err
	}
	return domain.SkillInstallationRecord{
		Installation:    installation,
		Definition:      definition,
		CurrentRevision: currentRevision,
	}, nil
}

func (s *MemoryStore) definitionHasInstallations(definitionID string) bool {
	for _, installation := range s.skillInstallations {
		if installation.DefinitionID == definitionID {
			return true
		}
	}
	return false
}

func (s *MemoryStore) definitionHasInstallationsForUser(userID, definitionID string) bool {
	for _, installation := range s.skillInstallations {
		if installation.UserID == userID && installation.DefinitionID == definitionID {
			return true
		}
	}
	return false
}

func (s *MemoryStore) clearDefaultSkillInstallations(userID, definitionID, exceptID string) {
	for id, installation := range s.skillInstallations {
		if installation.UserID != userID || installation.DefinitionID != definitionID || id == exceptID {
			continue
		}
		installation.IsDefault = false
		s.skillInstallations[id] = installation
	}
}

func (s *MemoryStore) promoteDefaultSkillInstallation(userID, definitionID, excludedID string) {
	var selectedID string
	var selected domain.SkillInstallation
	for id, installation := range s.skillInstallations {
		if installation.UserID != userID || installation.DefinitionID != definitionID || id == excludedID {
			continue
		}
		if selectedID == "" || installation.UpdatedAt.After(selected.UpdatedAt) {
			selectedID = id
			selected = installation
		}
	}
	if selectedID == "" {
		return
	}
	selected.IsDefault = true
	s.skillInstallations[selectedID] = selected
}

func (s *MemoryStore) hasSiblingSkillInstallation(userID, definitionID, excludedID string) bool {
	for id, installation := range s.skillInstallations {
		if id == excludedID {
			continue
		}
		if installation.UserID == userID && installation.DefinitionID == definitionID {
			return true
		}
	}
	return false
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func (s *MemoryStore) CreateRun(_ context.Context, run domain.Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[run.ID] = run
	return nil
}

func (s *MemoryStore) GetRun(_ context.Context, userID, runID string) (domain.Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	run, ok := s.runs[runID]
	if !ok || run.UserID != userID {
		return domain.Run{}, ErrNotFound
	}
	return run, nil
}

func (s *MemoryStore) GetRunByID(_ context.Context, runID string) (domain.Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	run, ok := s.runs[runID]
	if !ok {
		return domain.Run{}, ErrNotFound
	}
	return run, nil
}

func (s *MemoryStore) UpdateRun(_ context.Context, run domain.Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[run.ID] = run
	return nil
}

func (s *MemoryStore) ListRuns(_ context.Context, userID string) ([]domain.Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	runs := make([]domain.Run, 0, len(s.runs))
	for _, run := range s.runs {
		if run.UserID == userID {
			runs = append(runs, run)
		}
	}
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].UpdatedAt.After(runs[j].UpdatedAt)
	})
	return runs, nil
}

func (s *MemoryStore) UpsertRunStep(_ context.Context, step domain.RunStep) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	steps := s.runSteps[step.RunID]
	for i := range steps {
		if steps[i].ID == step.ID {
			steps[i] = step
			s.runSteps[step.RunID] = steps
			return nil
		}
	}
	s.runSteps[step.RunID] = append(steps, step)
	return nil
}

func (s *MemoryStore) ListRunSteps(_ context.Context, runID string) ([]domain.RunStep, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	steps := append([]domain.RunStep(nil), s.runSteps[runID]...)
	sort.Slice(steps, func(i, j int) bool {
		return steps[i].CreatedAt.Before(steps[j].CreatedAt)
	})
	return steps, nil
}

func (s *MemoryStore) SaveArtifact(_ context.Context, artifact domain.RunArtifact) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.artifacts[artifact.RunID] = append(s.artifacts[artifact.RunID], artifact)
	return nil
}

func (s *MemoryStore) ListArtifacts(_ context.Context, runID string) ([]domain.RunArtifact, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	artifacts := append([]domain.RunArtifact(nil), s.artifacts[runID]...)
	sort.Slice(artifacts, func(i, j int) bool {
		return artifacts[i].CreatedAt.Before(artifacts[j].CreatedAt)
	})
	return artifacts, nil
}

func (s *MemoryStore) SaveSources(_ context.Context, runID string, sources []domain.RunSource) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runSources[runID] = append([]domain.RunSource(nil), sources...)
	return nil
}

func (s *MemoryStore) ListSources(_ context.Context, runID string) ([]domain.RunSource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sources := append([]domain.RunSource(nil), s.runSources[runID]...)
	sort.Slice(sources, func(i, j int) bool {
		return sources[i].CreatedAt.Before(sources[j].CreatedAt)
	})
	return sources, nil
}

func (s *MemoryStore) CreateKnowledgeConnection(_ context.Context, connection domain.KnowledgeConnection) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.connections[connection.ID] = connection
	return nil
}

func (s *MemoryStore) UpdateKnowledgeConnection(_ context.Context, connection domain.KnowledgeConnection) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.connections[connection.ID] = connection
	return nil
}

func (s *MemoryStore) GetKnowledgeConnection(_ context.Context, userID, connectionID string) (domain.KnowledgeConnection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	connection, ok := s.connections[connectionID]
	if !ok || connection.UserID != userID {
		return domain.KnowledgeConnection{}, ErrNotFound
	}
	return connection, nil
}

func (s *MemoryStore) GetKnowledgeConnectionByID(_ context.Context, connectionID string) (domain.KnowledgeConnection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	connection, ok := s.connections[connectionID]
	if !ok {
		return domain.KnowledgeConnection{}, ErrNotFound
	}
	return connection, nil
}

func (s *MemoryStore) ListKnowledgeConnections(_ context.Context, userID string) ([]domain.KnowledgeConnection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	connections := []domain.KnowledgeConnection{}
	for _, connection := range s.connections {
		if connection.UserID == userID {
			connections = append(connections, connection)
		}
	}
	sort.Slice(connections, func(i, j int) bool {
		return connections[i].UpdatedAt.After(connections[j].UpdatedAt)
	})
	return connections, nil
}

func (s *MemoryStore) DeleteKnowledgeConnection(_ context.Context, userID, connectionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	connection, ok := s.connections[connectionID]
	if !ok || connection.UserID != userID {
		return ErrNotFound
	}

	delete(s.connections, connectionID)
	delete(s.metadata, connectionID)
	delete(s.sourceDocuments, connectionID)
	delete(s.documents, connectionID)
	delete(s.chunks, connectionID)
	for jobID, job := range s.syncJobs {
		if job.ConnectionID == connectionID && job.UserID == userID {
			delete(s.syncJobs, jobID)
		}
	}
	return nil
}

func (s *MemoryStore) CreateKnowledgeSyncJob(_ context.Context, job domain.KnowledgeSyncJob) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.syncJobs[job.ID] = job
	return nil
}

func (s *MemoryStore) UpdateKnowledgeSyncJob(_ context.Context, job domain.KnowledgeSyncJob) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.syncJobs[job.ID] = job
	return nil
}

func (s *MemoryStore) GetKnowledgeSyncJob(_ context.Context, jobID string) (domain.KnowledgeSyncJob, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.syncJobs[jobID]
	if !ok {
		return domain.KnowledgeSyncJob{}, ErrNotFound
	}
	return job, nil
}

func (s *MemoryStore) ListKnowledgeSyncJobs(_ context.Context, userID, connectionID string) ([]domain.KnowledgeSyncJob, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	jobs := []domain.KnowledgeSyncJob{}
	for _, job := range s.syncJobs {
		if job.UserID == userID && job.ConnectionID == connectionID {
			jobs = append(jobs, job)
		}
	}
	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].CreatedAt.After(jobs[j].CreatedAt)
	})
	return jobs, nil
}

func (s *MemoryStore) ListKnowledgeMetadata(_ context.Context, connectionID string) ([]domain.KnowledgeDocumentMeta, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]domain.KnowledgeDocumentMeta(nil), s.metadata[connectionID]...), nil
}

func (s *MemoryStore) ReplaceKnowledgeMetadata(_ context.Context, connectionID string, docs []domain.KnowledgeDocumentMeta) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metadata[connectionID] = append([]domain.KnowledgeDocumentMeta(nil), docs...)
	return nil
}

func (s *MemoryStore) ListKnowledgeSourceDocuments(_ context.Context, connectionID string) ([]domain.KnowledgeSourceDocument, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]domain.KnowledgeSourceDocument(nil), s.sourceDocuments[connectionID]...), nil
}

func (s *MemoryStore) ReplaceKnowledgeSourceDocuments(_ context.Context, connectionID string, docs []domain.KnowledgeSourceDocument) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sourceDocuments[connectionID] = append([]domain.KnowledgeSourceDocument(nil), docs...)
	return nil
}

func (s *MemoryStore) GetKnowledgeCorpus(_ context.Context, connectionID string) ([]domain.KnowledgeDocument, []domain.KnowledgeChunk, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	docs := append([]domain.KnowledgeDocument(nil), s.documents[connectionID]...)
	chunks := append([]domain.KnowledgeChunk(nil), s.chunks[connectionID]...)
	return docs, chunks, nil
}

func (s *MemoryStore) ReplaceKnowledgeCorpus(_ context.Context, connectionID string, docs []domain.KnowledgeDocument, chunks []domain.KnowledgeChunk) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.documents[connectionID] = append([]domain.KnowledgeDocument(nil), docs...)
	s.chunks[connectionID] = append([]domain.KnowledgeChunk(nil), chunks...)
	return nil
}

func (s *MemoryStore) SearchKnowledge(_ context.Context, userID string, connectionIDs []string, query string, limit int) ([]domain.KnowledgeHit, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	allowed := map[string]struct{}{}
	if len(connectionIDs) == 0 {
		for id, connection := range s.connections {
			if connection.UserID == userID {
				allowed[id] = struct{}{}
			}
		}
	} else {
		for _, id := range connectionIDs {
			if connection, ok := s.connections[id]; ok && connection.UserID == userID {
				allowed[id] = struct{}{}
			}
		}
	}

	tokens := tokenize(query)
	loweredQuery := strings.ToLower(strings.TrimSpace(query))
	hits := []domain.KnowledgeHit{}
	for connectionID := range allowed {
		docIndex := map[string]domain.KnowledgeDocument{}
		for _, doc := range s.documents[connectionID] {
			docIndex[doc.ID] = doc
		}
		for _, chunk := range s.chunks[connectionID] {
			doc := docIndex[chunk.DocumentID]
			connection := s.connections[connectionID]
			score := boostedScore(doc.Title, doc.Repo, chunk.Content, tokens)
			if score == 0 &&
				!strings.Contains(strings.ToLower(chunk.Content), loweredQuery) &&
				!strings.Contains(strings.ToLower(doc.Title), loweredQuery) &&
				!strings.Contains(strings.ToLower(doc.Repo), loweredQuery) {
				continue
			}
			if score == 0 {
				score = 1
			}
			hits = append(hits, domain.KnowledgeHit{
				ConnectionID: connectionID,
				DocumentID:   chunk.DocumentID,
				ChunkID:      chunk.ID,
				Provider:     string(connection.Provider),
				Title:        doc.Title,
				Repo:         doc.Repo,
				URL:          doc.SourceURL,
				Snippet:      snippet(chunk.Content, tokens, 220),
				Score:        score,
			})
		}
	}

	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Score == hits[j].Score {
			return hits[i].Title < hits[j].Title
		}
		return hits[i].Score > hits[j].Score
	})

	if limit > 0 && len(hits) > limit {
		hits = hits[:limit]
	}
	return hits, nil
}

func (s *MemoryStore) CreateMirrorTask(_ context.Context, task domain.MirrorTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mirrorTasks[task.ID] = task
	return nil
}

func (s *MemoryStore) UpdateMirrorTask(_ context.Context, task domain.MirrorTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.mirrorTasks[task.ID]; !ok {
		return ErrNotFound
	}
	s.mirrorTasks[task.ID] = task
	return nil
}

func (s *MemoryStore) GetMirrorTask(_ context.Context, taskID string) (domain.MirrorTask, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	task, ok := s.mirrorTasks[taskID]
	if !ok {
		return domain.MirrorTask{}, ErrNotFound
	}
	return task, nil
}

func (s *MemoryStore) ListDueMirrorTasks(_ context.Context, dueBefore time.Time, limit int) ([]domain.MirrorTask, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tasks := []domain.MirrorTask{}
	for _, task := range s.mirrorTasks {
		if task.Status != domain.MirrorTaskPending {
			continue
		}
		if task.NextRetryAt.After(dueBefore) {
			continue
		}
		tasks = append(tasks, task)
	}
	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].NextRetryAt.Equal(tasks[j].NextRetryAt) {
			return tasks[i].CreatedAt.Before(tasks[j].CreatedAt)
		}
		return tasks[i].NextRetryAt.Before(tasks[j].NextRetryAt)
	})
	if limit > 0 && len(tasks) > limit {
		tasks = tasks[:limit]
	}
	return tasks, nil
}

func cloneSkillArtifact(artifact domain.SkillArtifact) domain.SkillArtifact {
	artifact.ArchiveBytes = append([]byte(nil), artifact.ArchiveBytes...)
	return artifact
}

func (s *MemoryStore) ensureDefaultChatModelsLocked(userID string, now time.Time) {
	if !s.hasAnyChatModelForPurposeLocked(userID, domain.ChatModelPurposeGeneral) {
		name, runtime := domain.DefaultChatModelConfigForPurpose(domain.ChatModelPurposeGeneral)
		model := domain.UserChatModel{
			ID:          userID + ":general:default",
			UserID:      userID,
			Purpose:     domain.ChatModelPurposeGeneral,
			Origin:      domain.ChatModelOriginDefault,
			Name:        name,
			BaseURL:     runtime.BaseURL,
			APIKey:      runtime.APIKey,
			ModelName:   runtime.ModelName,
			Temperature: runtime.Temperature,
			IsSelected:  true,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		s.chatModels[model.ID] = model
	}
	if !s.hasAnyChatModelForPurposeLocked(userID, domain.ChatModelPurposeKnowledge) {
		name, runtime := domain.DefaultChatModelConfigForPurpose(domain.ChatModelPurposeKnowledge)
		model := domain.UserChatModel{
			ID:          userID + ":knowledge:default",
			UserID:      userID,
			Purpose:     domain.ChatModelPurposeKnowledge,
			Origin:      domain.ChatModelOriginDefault,
			Name:        name,
			BaseURL:     runtime.BaseURL,
			APIKey:      runtime.APIKey,
			ModelName:   runtime.ModelName,
			Temperature: runtime.Temperature,
			IsSelected:  true,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		s.chatModels[model.ID] = model
	}
	if !s.hasSelectedChatModelLocked(userID, domain.ChatModelPurposeGeneral) {
		s.selectFirstChatModelLocked(userID, domain.ChatModelPurposeGeneral, now)
	}
	if !s.hasSelectedChatModelLocked(userID, domain.ChatModelPurposeKnowledge) {
		s.selectFirstChatModelLocked(userID, domain.ChatModelPurposeKnowledge, now)
	}
}

func (s *MemoryStore) hasAnyChatModelForPurposeLocked(userID string, purpose domain.ChatModelPurpose) bool {
	for _, model := range s.chatModels {
		if model.UserID == userID && model.Purpose == purpose {
			return true
		}
	}
	return false
}

func (s *MemoryStore) hasSelectedChatModelLocked(userID string, purpose domain.ChatModelPurpose) bool {
	for _, model := range s.chatModels {
		if model.UserID == userID && model.Purpose == purpose && model.IsSelected {
			return true
		}
	}
	return false
}

func (s *MemoryStore) selectFirstChatModelLocked(userID string, purpose domain.ChatModelPurpose, now time.Time) {
	candidates := make([]domain.UserChatModel, 0)
	for _, model := range s.chatModels {
		if model.UserID == userID && model.Purpose == purpose {
			candidates = append(candidates, model)
		}
	}
	if len(candidates) == 0 {
		return
	}
	sort.Slice(candidates, func(i, j int) bool {
		if !candidates[i].CreatedAt.Equal(candidates[j].CreatedAt) {
			return candidates[i].CreatedAt.Before(candidates[j].CreatedAt)
		}
		return candidates[i].Name < candidates[j].Name
	})
	targetID := candidates[0].ID
	for id, model := range s.chatModels {
		if model.UserID != userID || model.Purpose != purpose {
			continue
		}
		model.IsSelected = id == targetID
		model.UpdatedAt = now
		s.chatModels[id] = model
	}
}
