package httpapi

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/provider"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/chat"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/knowledge"
	skillsvc "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skill"
)

func newTestHandler(st fullTestStore) *Handler {
	return &Handler{
		chatService:      newChatServiceForTest(st, nil),
		knowledgeService: newKnowledgeServiceForTest(st, nil, nil),
		skillService:     newSkillServiceForTest(st, nil),
	}
}

type fullTestStore interface {
	chat.ChatModelStore
	chat.SessionStore
	chat.ConversationStore
	chat.PrepareStreamStore
	knowledge.KnowledgeConnectionStore
	knowledge.KnowledgeSyncJobStore
	knowledge.KnowledgeMetadataStore
	knowledge.KnowledgeCorpusStore
	skillsvc.SkillMutationStore
	skillsvc.SkillInstallationStore
	skillsvc.SkillRecordStore
	skillsvc.SkillDefinitionStore
	skillsvc.SkillImportJobStore
	skillsvc.SkillSelectionStore
	skillsvc.ImportDefinitionStore
	skillsvc.ImportArtifactStore
	skillsvc.ImportJobLifecycleStore
}

func newKnowledgeServiceForTest(
	st interface {
		knowledge.KnowledgeConnectionStore
		knowledge.KnowledgeSyncJobStore
		knowledge.KnowledgeMetadataStore
		knowledge.KnowledgeCorpusStore
	},
	syncClient provider.KnowledgeSyncClient,
	mirrorClient provider.MirrorClient,
) *knowledge.Service {
	return knowledge.NewService(knowledge.ServiceDeps{
		Connections: st,
		SyncJobs:    st,
		Metadata:    st,
		Corpus:      st,
	}, syncClient, mirrorClient)
}

func newChatServiceForTest(
	st interface {
		chat.ChatModelStore
		chat.SessionStore
		chat.ConversationStore
		chat.PrepareStreamStore
	},
	forward provider.ForwardChatClient,
) *chat.Service {
	return chat.NewService(chat.ServiceDeps{
		Models:        st,
		Sessions:      st,
		Conversations: st,
		Prepare:       st,
	}, nil, nil, forward)
}

func newSkillServiceForTest(
	st interface {
		skillsvc.SkillMutationStore
		skillsvc.SkillInstallationStore
		skillsvc.SkillRecordStore
		skillsvc.SkillDefinitionStore
		skillsvc.SkillImportJobStore
		skillsvc.SkillSelectionStore
		skillsvc.ImportArtifactStore
	},
	mirror skillsvc.MirrorSyncer,
) *skillsvc.Service {
	return skillsvc.NewService(skillsvc.ServiceDeps{
		Mutations:     st,
		Installations: st,
		Records:       st,
		Definitions:   st,
		ImportJobs:    st,
		Artifacts:     st,
		Selection:     st,
	}, mirror)
}

func newSkillImportServiceForTest(
	st interface {
		skillsvc.ImportDefinitionStore
		skillsvc.SkillInstallationStore
		skillsvc.ImportArtifactStore
		skillsvc.ImportJobLifecycleStore
		skillsvc.SkillRecordStore
	},
	parser skillsvc.ImportParser,
	mirror skillsvc.MirrorSyncer,
) *skillsvc.ImportService {
	return skillsvc.NewImportService(skillsvc.ImportDeps{
		Definitions:   st,
		Installations: st,
		Artifacts:     st,
		Jobs:          st,
		Records:       st,
	}, parser, mirror)
}

func withUserAndParam(ctx context.Context, user domain.User, key, value string) context.Context {
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add(key, value)
	ctx = context.WithValue(ctx, chi.RouteCtxKey, routeCtx)
	return context.WithValue(ctx, userKey, user)
}

func decodeAPIError(t *testing.T, body []byte) apiErrorPayload {
	t.Helper()
	var payload apiErrorPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return payload
}

func buildSkillImportArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for name, content := range files {
		file, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", name, err)
		}
		if _, err := file.Write([]byte(content)); err != nil {
			t.Fatalf("write zip entry %s: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	return buf.Bytes()
}

func detailStringList(t *testing.T, payload apiErrorPayload, key string) []string {
	t.Helper()
	if payload.Details == nil {
		t.Fatalf("expected details to contain %s, got nil", key)
	}
	raw, ok := payload.Details[key]
	if !ok {
		t.Fatalf("expected details to contain %s, got %#v", key, payload.Details)
	}
	itemsAny, ok := raw.([]any)
	if !ok {
		t.Fatalf("expected details[%s] to be []any, got %T", key, raw)
	}
	items := make([]string, 0, len(itemsAny))
	for _, item := range itemsAny {
		text, ok := item.(string)
		if !ok {
			t.Fatalf("expected details[%s] items to be string, got %T", key, item)
		}
		items = append(items, text)
	}
	return items
}

func detailString(t *testing.T, payload apiErrorPayload, key string) string {
	t.Helper()
	if payload.Details == nil {
		t.Fatalf("expected details to contain %s, got nil", key)
	}
	raw, ok := payload.Details[key]
	if !ok {
		t.Fatalf("expected details to contain %s, got %#v", key, payload.Details)
	}
	text, ok := raw.(string)
	if !ok {
		t.Fatalf("expected details[%s] to be string, got %T", key, raw)
	}
	return text
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

type errorInjectingStore struct {
	fullTestStore
	createSkillErr                error
	createChatSessionErr          error
	getSkillErr                   error
	getSkillDefinitionDetailsErr  error
	getSkillInstallationRecordErr error
}

type errorMirrorClient struct {
	deleteKnowledgeErr error
	upsertSkillErr     error
	deleteSkillErr     error
	upsertedSkills     []domain.Skill
	deletedSkillIDs    []string
}

type errorForwardClient struct {
	err error
}

type snapshotRecordingStore struct {
	fullTestStore
	lastSnapshot *domain.SkillRuntimeSnapshot
}

func (s *errorInjectingStore) CreateSkill(ctx context.Context, skill domain.Skill) error {
	if s.createSkillErr != nil {
		return s.createSkillErr
	}
	return s.fullTestStore.CreateSkill(ctx, skill)
}

func (s *errorInjectingStore) CreateChatSession(ctx context.Context, session domain.ChatSession) error {
	if s.createChatSessionErr != nil {
		return s.createChatSessionErr
	}
	return s.fullTestStore.CreateChatSession(ctx, session)
}

func (s *errorInjectingStore) GetSkill(ctx context.Context, userID, skillID string) (domain.Skill, error) {
	if s.getSkillErr != nil {
		return domain.Skill{}, s.getSkillErr
	}
	return s.fullTestStore.GetSkill(ctx, userID, skillID)
}

func (s *errorInjectingStore) GetSkillDefinitionDetails(ctx context.Context, userID, definitionID string) (domain.SkillDefinitionDetails, error) {
	if s.getSkillDefinitionDetailsErr != nil {
		return domain.SkillDefinitionDetails{}, s.getSkillDefinitionDetailsErr
	}
	return s.fullTestStore.GetSkillDefinitionDetails(ctx, userID, definitionID)
}

func (s *errorInjectingStore) GetSkillInstallationRecord(ctx context.Context, userID, installationID string) (domain.SkillInstallationRecord, error) {
	if s.getSkillInstallationRecordErr != nil {
		return domain.SkillInstallationRecord{}, s.getSkillInstallationRecordErr
	}
	return s.fullTestStore.GetSkillInstallationRecord(ctx, userID, installationID)
}

func (c *errorMirrorClient) UpsertSkill(_ context.Context, skill domain.Skill) error {
	if c.upsertSkillErr != nil {
		return c.upsertSkillErr
	}
	c.upsertedSkills = append(c.upsertedSkills, skill)
	return nil
}

func (c *errorMirrorClient) DeleteSkill(_ context.Context, _ string, skillID string) error {
	if c.deleteSkillErr != nil {
		return c.deleteSkillErr
	}
	c.deletedSkillIDs = append(c.deletedSkillIDs, skillID)
	return nil
}

func (c *errorMirrorClient) UpsertKnowledge(context.Context, provider.MirroredKnowledgeUpsertRequest) (provider.MirroredKnowledgeUpsertResult, error) {
	return provider.MirroredKnowledgeUpsertResult{}, nil
}

func (c *errorMirrorClient) DeleteKnowledge(context.Context, string, string) error {
	return c.deleteKnowledgeErr
}

func (c *errorForwardClient) StreamKnowledgeChat(
	context.Context,
	provider.ForwardedChatRequest,
	func([]provider.ForwardedSource) error,
	func(string) error,
) (provider.ForwardedChatResult, error) {
	return provider.ForwardedChatResult{}, c.err
}

func (s *snapshotRecordingStore) CreateSkillRuntimeSnapshot(ctx context.Context, snapshot domain.SkillRuntimeSnapshot) error {
	copy := snapshot
	s.lastSnapshot = &copy
	return s.fullTestStore.CreateSkillRuntimeSnapshot(ctx, snapshot)
}
