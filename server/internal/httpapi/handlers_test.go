package httpapi

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/chat"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/mirror"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/provider"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/run"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skillimport"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/store"
)

func TestGetSkillDefinitionReturnsDetails(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{store: mem}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	now := time.Now().UTC()
	skill := domain.Skill{
		ID:            "install-1",
		UserID:        user.ID,
		DefinitionID:  "def-1",
		RevisionID:    "rev-1",
		Version:       1,
		Slug:          "research-skill",
		Kind:          domain.SkillKindAgentWorkflow,
		Title:         "Research Skill",
		Description:   "initial",
		Prompt:        "do research",
		Mode:          "answer",
		PlannerPolicy: map[string]string{"preferred_mode": "hybrid"},
		ToolAllowlist: []string{"knowledge.search", "report.write"},
		Source:        domain.SkillSourceManual,
		Enabled:       true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := mem.CreateSkill(context.Background(), skill); err != nil {
		t.Fatalf("create skill: %v", err)
	}

	skill.RevisionID = "rev-2"
	skill.Version = 2
	skill.Title = "Research Skill v2"
	skill.Description = "updated"
	skill.UpdatedAt = now.Add(time.Minute)
	if err := mem.UpdateSkill(context.Background(), skill); err != nil {
		t.Fatalf("update skill: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/skill-definitions/def-1", nil)
	req = req.WithContext(withUserAndParam(req.Context(), user, "definitionID", "def-1"))
	rec := httptest.NewRecorder()

	handler.getSkillDefinition(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload domain.SkillDefinitionDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Definition.ID != "def-1" {
		t.Fatalf("expected definition def-1, got %s", payload.Definition.ID)
	}
	if len(payload.Revisions) != 2 {
		t.Fatalf("expected 2 revisions, got %d", len(payload.Revisions))
	}
	if payload.Revisions[0].Version != 2 {
		t.Fatalf("expected latest revision first, got version %d", payload.Revisions[0].Version)
	}
	if len(payload.Installations) != 1 {
		t.Fatalf("expected 1 installation, got %d", len(payload.Installations))
	}
	if payload.Installations[0].CurrentRevisionID != "rev-2" {
		t.Fatalf("expected installation to point to latest revision, got %s", payload.Installations[0].CurrentRevisionID)
	}
	if !payload.Installations[0].IsDefault {
		t.Fatalf("expected initial installation to be default")
	}
}

func TestRequireAuthReturnsUnauthorizedCodeOnMissingBearerToken(t *testing.T) {
	handler := &Handler{}
	called := false
	wrapped := handler.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	rec := httptest.NewRecorder()

	wrapped(rec, req)

	if called {
		t.Fatalf("expected wrapped handler not to be called")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
	payload := decodeAPIError(t, rec.Body.Bytes())
	if payload.Code != errorCodeUnauthorized {
		t.Fatalf("expected code %s, got %s", errorCodeUnauthorized, payload.Code)
	}
}

func TestLoginReturnsInvalidRequestBodyCode(t *testing.T) {
	handler := &Handler{}
	req := httptest.NewRequest(http.MethodPost, "/v1/login", strings.NewReader("{"))
	rec := httptest.NewRecorder()

	handler.login(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	payload := decodeAPIError(t, rec.Body.Bytes())
	if payload.Code != errorCodeInvalidRequestBody {
		t.Fatalf("expected code %s, got %s", errorCodeInvalidRequestBody, payload.Code)
	}
}

func TestCreateRunReturnsInvalidRequestBodyCode(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{store: mem}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/runs", strings.NewReader("{"))
	req = req.WithContext(context.WithValue(req.Context(), userKey, user))
	rec := httptest.NewRecorder()

	handler.createRun(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	payload := decodeAPIError(t, rec.Body.Bytes())
	if payload.Code != errorCodeInvalidRequestBody {
		t.Fatalf("expected code %s, got %s", errorCodeInvalidRequestBody, payload.Code)
	}
}

func TestCreateRunReturnsValidationErrorForMissingGoal(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{store: mem}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/runs", strings.NewReader(`{"goal":"   "}`))
	req = req.WithContext(context.WithValue(req.Context(), userKey, user))
	rec := httptest.NewRecorder()

	handler.createRun(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	payload := decodeAPIError(t, rec.Body.Bytes())
	if payload.Code != errorCodeValidationFailed {
		t.Fatalf("expected code %s, got %s", errorCodeValidationFailed, payload.Code)
	}
	if payload.Field != "goal" {
		t.Fatalf("expected field goal, got %s", payload.Field)
	}
	fields := detailStringList(t, payload, "fields")
	if !containsString(fields, "goal") {
		t.Fatalf("expected details.fields to contain goal, got %#v", fields)
	}
}

func TestChatStreamReturnsServiceUnavailableCode(t *testing.T) {
	handler := &Handler{}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/chat/stream?message=hello", nil)
	req = req.WithContext(context.WithValue(req.Context(), userKey, user))
	rec := httptest.NewRecorder()

	handler.chatStream(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", rec.Code, rec.Body.String())
	}
	payload := decodeAPIError(t, rec.Body.Bytes())
	if payload.Code != errorCodeServiceUnavailable {
		t.Fatalf("expected code %s, got %s", errorCodeServiceUnavailable, payload.Code)
	}
}

func TestChatStreamReturnsPythonKnowledgeServiceUnavailableCode(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{
		store:       mem,
		chatService: chat.NewService(nil, nil, nil),
	}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/chat/stream?message=hello&use_knowledge=true", nil)
	req = req.WithContext(context.WithValue(req.Context(), userKey, user))
	rec := httptest.NewRecorder()

	handler.chatStream(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, `"code":"`+errorCodeServiceUnavailable+`"`) {
		t.Fatalf("expected SSE body to contain service unavailable code, got %s", body)
	}
	if !strings.Contains(body, `"error":"python knowledge service unavailable"`) {
		t.Fatalf("expected SSE body to contain python knowledge error, got %s", body)
	}
}

func TestChatStreamReturnsInternalErrorCodeOnSessionCreationFailure(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{
		store: &errorInjectingStore{
			Store:                mem,
			createChatSessionErr: errors.New("boom"),
		},
		chatService: chat.NewService(nil, nil, nil),
	}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/chat/stream?message=hello", nil)
	req = req.WithContext(context.WithValue(req.Context(), userKey, user))
	rec := httptest.NewRecorder()

	handler.chatStream(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, `"code":"`+errorCodeInternalError+`"`) {
		t.Fatalf("expected SSE body to contain internal error code, got %s", body)
	}
	if !strings.Contains(body, `"error":"boom"`) {
		t.Fatalf("expected SSE body to contain boom error, got %s", body)
	}
}

func TestChatStreamPersistsSkillSnapshotWhenSkillSelected(t *testing.T) {
	mem := store.NewMemoryStore()
	recordingStore := &snapshotRecordingStore{Store: mem}
	handler := &Handler{
		store:       recordingStore,
		chatService: chat.NewService(nil, nil, nil),
	}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	now := time.Now().UTC()
	skill := domain.Skill{
		ID:            "install-1",
		UserID:        user.ID,
		DefinitionID:  "def-1",
		RevisionID:    "rev-1",
		Version:       1,
		Slug:          "summary-skill",
		Kind:          domain.SkillKindChatProfile,
		Title:         "Summary Skill",
		Description:   "summarize things",
		Prompt:        "summarize in terse bullets",
		Mode:          "summary",
		PlannerPolicy: map[string]string{"preferred_mode": "kb_only"},
		ToolAllowlist: []string{"report.write"},
		Source:        domain.SkillSourceManual,
		Enabled:       true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := mem.CreateSkill(context.Background(), skill); err != nil {
		t.Fatalf("create skill: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/chat/stream?message=hello&skill_id=install-1", nil)
	req = req.WithContext(context.WithValue(req.Context(), userKey, user))
	rec := httptest.NewRecorder()

	handler.chatStream(rec, req)

	if recordingStore.lastSnapshot == nil {
		t.Fatalf("expected chat stream to persist a skill snapshot")
	}
	if recordingStore.lastSnapshot.Scope != domain.SkillRuntimeScopeChat {
		t.Fatalf("expected chat scope, got %s", recordingStore.lastSnapshot.Scope)
	}
	if recordingStore.lastSnapshot.InstallationID != "install-1" {
		t.Fatalf("expected installation install-1, got %s", recordingStore.lastSnapshot.InstallationID)
	}
	if recordingStore.lastSnapshot.DefinitionID != "def-1" {
		t.Fatalf("expected definition def-1, got %s", recordingStore.lastSnapshot.DefinitionID)
	}
	if recordingStore.lastSnapshot.Prompt != "summarize in terse bullets" {
		t.Fatalf("expected prompt to round-trip, got %s", recordingStore.lastSnapshot.Prompt)
	}

	body := rec.Body.String()
	if !strings.Contains(body, `"type":"session"`) {
		t.Fatalf("expected SSE body to contain session event, got %s", body)
	}
	if !strings.Contains(body, `"type":"done"`) {
		t.Fatalf("expected SSE body to contain done event, got %s", body)
	}
}

func TestChatStreamReturnsBadGatewayCodeOnForwardFailure(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{
		store:       mem,
		chatService: chat.NewService(nil, nil, &errorForwardClient{err: errors.New("python knowledge stream request failed: 502 Bad Gateway")}),
	}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/chat/stream?message=hello&use_knowledge=true", nil)
	req = req.WithContext(context.WithValue(req.Context(), userKey, user))
	rec := httptest.NewRecorder()

	handler.chatStream(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, `"code":"`+errorCodeBadGateway+`"`) {
		t.Fatalf("expected SSE body to contain bad gateway code, got %s", body)
	}
	if !strings.Contains(body, `"error":"python knowledge stream request failed: 502 Bad Gateway"`) {
		t.Fatalf("expected SSE body to contain forward failure, got %s", body)
	}
}

func TestChatStreamReturnsServiceUnavailableCodeOnForwardClientUnavailable(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{
		store:       mem,
		chatService: chat.NewService(nil, nil, &errorForwardClient{err: errors.New("python forward client is not configured")}),
	}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/chat/stream?message=hello&use_knowledge=true", nil)
	req = req.WithContext(context.WithValue(req.Context(), userKey, user))
	rec := httptest.NewRecorder()

	handler.chatStream(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, `"code":"`+errorCodeServiceUnavailable+`"`) {
		t.Fatalf("expected SSE body to contain service unavailable code, got %s", body)
	}
	if !strings.Contains(body, `"error":"python forward client is not configured"`) {
		t.Fatalf("expected SSE body to contain provider unavailable error, got %s", body)
	}
}

func TestGetRunReturnsNotFoundCode(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{
		store:      mem,
		runService: run.NewService(mem, nil, run.NewEventBroker(), run.NewPlanner(), nil, nil, nil, nil, nil),
	}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/runs/run-missing", nil)
	req = req.WithContext(withUserAndParam(req.Context(), user, "runID", "run-missing"))
	rec := httptest.NewRecorder()

	handler.getRun(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
	payload := decodeAPIError(t, rec.Body.Bytes())
	if payload.Code != errorCodeNotFound {
		t.Fatalf("expected code %s, got %s", errorCodeNotFound, payload.Code)
	}
	resource := detailString(t, payload, "resource")
	if resource != "run" {
		t.Fatalf("expected details.resource run, got %s", resource)
	}
}

func TestCreateChatSessionReturnsInvalidRequestBodyCode(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{store: mem}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/sessions", strings.NewReader("{"))
	req = req.WithContext(context.WithValue(req.Context(), userKey, user))
	rec := httptest.NewRecorder()

	handler.createChatSession(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	payload := decodeAPIError(t, rec.Body.Bytes())
	if payload.Code != errorCodeInvalidRequestBody {
		t.Fatalf("expected code %s, got %s", errorCodeInvalidRequestBody, payload.Code)
	}
}

func TestChatStreamReturnsValidationErrorForMissingMessage(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{
		store:       mem,
		chatService: chat.NewService(nil, nil, nil),
	}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/chat/stream", nil)
	req = req.WithContext(context.WithValue(req.Context(), userKey, user))
	rec := httptest.NewRecorder()

	handler.chatStream(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	payload := decodeAPIError(t, rec.Body.Bytes())
	if payload.Code != errorCodeValidationFailed {
		t.Fatalf("expected code %s, got %s", errorCodeValidationFailed, payload.Code)
	}
	if payload.Field != "message" {
		t.Fatalf("expected field message, got %s", payload.Field)
	}
}

func TestDeleteChatSessionReturnsNotFoundCode(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{store: mem}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/v1/chat/sessions/session-missing", nil)
	req = req.WithContext(withUserAndParam(req.Context(), user, "sessionID", "session-missing"))
	rec := httptest.NewRecorder()

	handler.deleteChatSession(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
	payload := decodeAPIError(t, rec.Body.Bytes())
	if payload.Code != errorCodeNotFound {
		t.Fatalf("expected code %s, got %s", errorCodeNotFound, payload.Code)
	}
	resource := detailString(t, payload, "resource")
	if resource != "chat_session" {
		t.Fatalf("expected details.resource chat_session, got %s", resource)
	}
}

func TestCreateYuqueConnectionReturnsInvalidRequestBodyCode(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{store: mem}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/knowledge/yuque/connections", strings.NewReader("{"))
	req = req.WithContext(context.WithValue(req.Context(), userKey, user))
	rec := httptest.NewRecorder()

	handler.createYuqueConnection(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	payload := decodeAPIError(t, rec.Body.Bytes())
	if payload.Code != errorCodeInvalidRequestBody {
		t.Fatalf("expected code %s, got %s", errorCodeInvalidRequestBody, payload.Code)
	}
}

func TestCreateYuqueConnectionReturnsValidationErrorForMissingCredentials(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{store: mem}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/knowledge/yuque/connections", strings.NewReader(`{"name":"Yuque"}`))
	req = req.WithContext(context.WithValue(req.Context(), userKey, user))
	rec := httptest.NewRecorder()

	handler.createYuqueConnection(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	payload := decodeAPIError(t, rec.Body.Bytes())
	if payload.Code != errorCodeValidationFailed {
		t.Fatalf("expected code %s, got %s", errorCodeValidationFailed, payload.Code)
	}
	fields := detailStringList(t, payload, "fields")
	if !containsString(fields, "config.token") || !containsString(fields, "config.groupLogin") {
		t.Fatalf("expected details.fields to contain config.token and config.groupLogin, got %#v", fields)
	}
}

func TestDeleteYuqueConnectionReturnsConflictCode(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{store: mem}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	now := time.Now().UTC()
	connection := domain.KnowledgeConnection{
		ID:       "conn-1",
		UserID:   user.ID,
		Provider: domain.ProviderYuque,
		Name:     "Yuque",
		Yuque: &domain.KnowledgeConnectionYuqueConfig{
			Token:      "token",
			GroupLogin: "group",
			CreatedAt:  now,
			UpdatedAt:  now,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := mem.CreateKnowledgeConnection(context.Background(), connection); err != nil {
		t.Fatalf("create connection: %v", err)
	}
	job := domain.KnowledgeSyncJob{
		ID:           "job-1",
		UserID:       user.ID,
		ConnectionID: connection.ID,
		Status:       domain.SyncJobRunning,
		CreatedAt:    now,
	}
	if err := mem.CreateKnowledgeSyncJob(context.Background(), job); err != nil {
		t.Fatalf("create job: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/v1/knowledge/yuque/connections/conn-1", nil)
	req = req.WithContext(withUserAndParam(req.Context(), user, "connectionID", "conn-1"))
	rec := httptest.NewRecorder()

	handler.deleteYuqueConnection(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
	payload := decodeAPIError(t, rec.Body.Bytes())
	if payload.Code != errorCodeConflict {
		t.Fatalf("expected code %s, got %s", errorCodeConflict, payload.Code)
	}
	resource := detailString(t, payload, "resource")
	if resource != "knowledge_connection" {
		t.Fatalf("expected details.resource knowledge_connection, got %s", resource)
	}
}

func TestDeleteYuqueConnectionReturnsBadGatewayCodeOnMirrorFailure(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{
		store:         mem,
		mirrorService: mirror.NewService(mem, &errorMirrorClient{deleteKnowledgeErr: errors.New("mirror boom")}),
	}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	now := time.Now().UTC()
	connection := domain.KnowledgeConnection{
		ID:       "conn-1",
		UserID:   user.ID,
		Provider: domain.ProviderYuque,
		Name:     "Yuque",
		Yuque: &domain.KnowledgeConnectionYuqueConfig{
			Token:      "token",
			GroupLogin: "group",
			CreatedAt:  now,
			UpdatedAt:  now,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := mem.CreateKnowledgeConnection(context.Background(), connection); err != nil {
		t.Fatalf("create connection: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/v1/knowledge/yuque/connections/conn-1", nil)
	req = req.WithContext(withUserAndParam(req.Context(), user, "connectionID", "conn-1"))
	rec := httptest.NewRecorder()

	handler.deleteYuqueConnection(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", rec.Code, rec.Body.String())
	}
	payload := decodeAPIError(t, rec.Body.Bytes())
	if payload.Code != errorCodeBadGateway {
		t.Fatalf("expected code %s, got %s", errorCodeBadGateway, payload.Code)
	}
	if detailString(t, payload, "upstream") != "python_mirror" {
		t.Fatalf("expected details.upstream python_mirror, got %s", detailString(t, payload, "upstream"))
	}
	if cause := detailString(t, payload, "cause"); !strings.Contains(cause, "mirror boom") {
		t.Fatalf("expected details.cause to contain mirror boom, got %s", cause)
	}
}

func TestGetSkillDefinitionReturnsNotFound(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{store: mem}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/skill-definitions/missing", nil)
	req = req.WithContext(withUserAndParam(req.Context(), user, "definitionID", "missing"))
	rec := httptest.NewRecorder()

	handler.getSkillDefinition(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["code"] != errorCodeSkillDefinitionNotFound {
		t.Fatalf("expected code %s, got %s", errorCodeSkillDefinitionNotFound, payload["code"])
	}
}

func TestListSkillInstallationsReturnsStructuredRecords(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{store: mem}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	now := time.Now().UTC()
	skill := domain.Skill{
		ID:            "install-1",
		UserID:        user.ID,
		DefinitionID:  "def-1",
		RevisionID:    "rev-1",
		Version:       1,
		Slug:          "summary-skill",
		Kind:          domain.SkillKindChatProfile,
		Title:         "Summary Skill",
		Description:   "summarize things",
		Prompt:        "summarize",
		Mode:          "summary",
		PlannerPolicy: map[string]string{"preferred_mode": "kb_only"},
		ToolAllowlist: []string{"knowledge.search", "report.write"},
		Source:        domain.SkillSourceManual,
		Enabled:       true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := mem.CreateSkill(context.Background(), skill); err != nil {
		t.Fatalf("create skill: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/skill-installations", nil)
	req = req.WithContext(context.WithValue(req.Context(), userKey, user))
	rec := httptest.NewRecorder()

	handler.listSkillInstallations(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Items []domain.SkillInstallationRecord `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Items) != 1 {
		t.Fatalf("expected 1 installation record, got %d", len(payload.Items))
	}
	item := payload.Items[0]
	if item.Installation.ID != "install-1" {
		t.Fatalf("expected installation install-1, got %s", item.Installation.ID)
	}
	if item.Definition.ID != "def-1" {
		t.Fatalf("expected definition def-1, got %s", item.Definition.ID)
	}
	if item.CurrentRevision.ID != "rev-1" {
		t.Fatalf("expected current revision rev-1, got %s", item.CurrentRevision.ID)
	}
	if item.CurrentRevision.PlannerPolicy["preferred_mode"] != "kb_only" {
		t.Fatalf("expected planner policy to round-trip")
	}
}

func TestGetSkillInstallationReturnsRecord(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{store: mem}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	now := time.Now().UTC()
	skill := domain.Skill{
		ID:           "install-1",
		UserID:       user.ID,
		DefinitionID: "def-1",
		RevisionID:   "rev-1",
		Version:      1,
		Slug:         "summary-skill",
		Kind:         domain.SkillKindChatProfile,
		Title:        "Summary Skill",
		Description:  "summarize things",
		Prompt:       "summarize",
		Mode:         "summary",
		Source:       domain.SkillSourceManual,
		Enabled:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := mem.CreateSkill(context.Background(), skill); err != nil {
		t.Fatalf("create skill: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/skill-installations/install-1", nil)
	req = req.WithContext(withUserAndParam(req.Context(), user, "installationID", "install-1"))
	rec := httptest.NewRecorder()

	handler.getSkillInstallation(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload domain.SkillInstallationRecord
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Installation.ID != "install-1" {
		t.Fatalf("expected installation install-1, got %s", payload.Installation.ID)
	}
	if payload.Definition.Slug != "summary-skill" {
		t.Fatalf("expected definition slug summary-skill, got %s", payload.Definition.Slug)
	}
	if payload.CurrentRevision.ID != "rev-1" {
		t.Fatalf("expected current revision rev-1, got %s", payload.CurrentRevision.ID)
	}
	if payload.Installation.Name != "Summary Skill" {
		t.Fatalf("expected installation name Summary Skill, got %s", payload.Installation.Name)
	}
	if !payload.Installation.IsDefault {
		t.Fatalf("expected initial installation to be default")
	}
}

func TestGetSkillInstallationReturnsNotFoundCode(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{store: mem}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/skill-installations/missing", nil)
	req = req.WithContext(withUserAndParam(req.Context(), user, "installationID", "missing"))
	rec := httptest.NewRecorder()

	handler.getSkillInstallation(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["code"] != errorCodeSkillInstallationNotFound {
		t.Fatalf("expected code %s, got %s", errorCodeSkillInstallationNotFound, payload["code"])
	}
}

func TestUpdateSkillInstallationCanSwitchRevision(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{store: mem}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	now := time.Now().UTC()
	skill := domain.Skill{
		ID:            "install-1",
		UserID:        user.ID,
		DefinitionID:  "def-1",
		RevisionID:    "rev-1",
		Version:       1,
		Slug:          "research-skill",
		Kind:          domain.SkillKindAgentWorkflow,
		Title:         "Research Skill",
		Description:   "initial",
		Prompt:        "do research",
		Mode:          "answer",
		PlannerPolicy: map[string]string{"preferred_mode": "hybrid"},
		ToolAllowlist: []string{"knowledge.search", "report.write"},
		Source:        domain.SkillSourceManual,
		Enabled:       true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := mem.CreateSkill(context.Background(), skill); err != nil {
		t.Fatalf("create skill: %v", err)
	}

	skill.RevisionID = "rev-2"
	skill.Version = 2
	skill.Title = "Research Skill v2"
	skill.Description = "updated"
	skill.UpdatedAt = now.Add(time.Minute)
	if err := mem.UpdateSkill(context.Background(), skill); err != nil {
		t.Fatalf("update skill: %v", err)
	}

	req := httptest.NewRequest(http.MethodPatch, "/v1/skill-installations/install-1", strings.NewReader(`{"currentRevisionId":"rev-1","enabled":false}`))
	req = req.WithContext(withUserAndParam(req.Context(), user, "installationID", "install-1"))
	rec := httptest.NewRecorder()

	handler.updateSkillInstallation(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload domain.SkillInstallationRecord
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Installation.CurrentRevisionID != "rev-1" {
		t.Fatalf("expected installation to point to rev-1, got %s", payload.Installation.CurrentRevisionID)
	}
	if payload.CurrentRevision.ID != "rev-1" {
		t.Fatalf("expected current revision rev-1, got %s", payload.CurrentRevision.ID)
	}
	if payload.Installation.Enabled {
		t.Fatalf("expected installation to be disabled after patch")
	}
}

func TestUpdateSkillInstallationRejectsMixedRevisionSwitchAndContentPatch(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{store: mem}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	now := time.Now().UTC()
	skill := domain.Skill{
		ID:           "install-1",
		UserID:       user.ID,
		DefinitionID: "def-1",
		RevisionID:   "rev-1",
		Version:      1,
		Slug:         "summary-skill",
		Kind:         domain.SkillKindChatProfile,
		Title:        "Summary Skill",
		Description:  "summarize things",
		Prompt:       "summarize",
		Mode:         "summary",
		Source:       domain.SkillSourceManual,
		Enabled:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := mem.CreateSkill(context.Background(), skill); err != nil {
		t.Fatalf("create skill: %v", err)
	}

	req := httptest.NewRequest(http.MethodPatch, "/v1/skill-installations/install-1", strings.NewReader(`{"currentRevisionId":"rev-1","title":"Changed"}`))
	req = req.WithContext(withUserAndParam(req.Context(), user, "installationID", "install-1"))
	rec := httptest.NewRecorder()

	handler.updateSkillInstallation(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	payload := decodeAPIError(t, rec.Body.Bytes())
	if payload.Code != errorCodeSkillInvalidFieldCombination {
		t.Fatalf("expected code %s, got %s", errorCodeSkillInvalidFieldCombination, payload.Code)
	}
	fields := detailStringList(t, payload, "fields")
	if !containsString(fields, "currentRevisionId") || !containsString(fields, "title") {
		t.Fatalf("expected details.fields to contain currentRevisionId and title, got %#v", fields)
	}
}

func TestUpdateSkillInstallationReturnsRevisionNotFoundCode(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{store: mem}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	now := time.Now().UTC()
	skill := domain.Skill{
		ID:           "install-1",
		UserID:       user.ID,
		DefinitionID: "def-1",
		RevisionID:   "rev-1",
		Version:      1,
		Slug:         "summary-skill",
		Kind:         domain.SkillKindChatProfile,
		Title:        "Summary Skill",
		Description:  "summarize things",
		Prompt:       "summarize",
		Mode:         "summary",
		Source:       domain.SkillSourceManual,
		Enabled:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := mem.CreateSkill(context.Background(), skill); err != nil {
		t.Fatalf("create skill: %v", err)
	}

	req := httptest.NewRequest(http.MethodPatch, "/v1/skill-installations/install-1", strings.NewReader(`{"currentRevisionId":"rev-missing"}`))
	req = req.WithContext(withUserAndParam(req.Context(), user, "installationID", "install-1"))
	rec := httptest.NewRecorder()

	handler.updateSkillInstallation(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["code"] != errorCodeSkillRevisionNotFound {
		t.Fatalf("expected code %s, got %s", errorCodeSkillRevisionNotFound, payload["code"])
	}
}

func TestCreateSkillInstallationFromExistingDefinitionUsesLatestRevision(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{store: mem}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	now := time.Now().UTC()
	skill := domain.Skill{
		ID:            "install-1",
		UserID:        user.ID,
		DefinitionID:  "def-1",
		RevisionID:    "rev-1",
		Version:       1,
		Slug:          "research-skill",
		Kind:          domain.SkillKindAgentWorkflow,
		Title:         "Research Skill",
		Description:   "initial",
		Prompt:        "do research",
		Mode:          "answer",
		PlannerPolicy: map[string]string{"preferred_mode": "hybrid"},
		ToolAllowlist: []string{"knowledge.search", "report.write"},
		Source:        domain.SkillSourceManual,
		Enabled:       true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := mem.CreateSkill(context.Background(), skill); err != nil {
		t.Fatalf("create skill: %v", err)
	}

	skill.RevisionID = "rev-2"
	skill.Version = 2
	skill.Title = "Research Skill v2"
	skill.Description = "updated"
	skill.UpdatedAt = now.Add(time.Minute)
	if err := mem.UpdateSkill(context.Background(), skill); err != nil {
		t.Fatalf("update skill: %v", err)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/v1/skill-installations/install-1", nil)
	deleteReq = deleteReq.WithContext(withUserAndParam(deleteReq.Context(), user, "installationID", "install-1"))
	deleteRec := httptest.NewRecorder()
	handler.deleteSkillInstallation(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("expected delete 204, got %d: %s", deleteRec.Code, deleteRec.Body.String())
	}

	defReq := httptest.NewRequest(http.MethodGet, "/v1/skill-definitions/def-1", nil)
	defReq = defReq.WithContext(withUserAndParam(defReq.Context(), user, "definitionID", "def-1"))
	defRec := httptest.NewRecorder()
	handler.getSkillDefinition(defRec, defReq)
	if defRec.Code != http.StatusOK {
		t.Fatalf("expected definition 200 after uninstall, got %d: %s", defRec.Code, defRec.Body.String())
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/skill-installations", strings.NewReader(`{"definitionId":"def-1"}`))
	req = req.WithContext(context.WithValue(req.Context(), userKey, user))
	rec := httptest.NewRecorder()

	handler.createSkillInstallation(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload domain.SkillInstallationRecord
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Definition.ID != "def-1" {
		t.Fatalf("expected definition def-1, got %s", payload.Definition.ID)
	}
	if payload.CurrentRevision.ID != "rev-2" {
		t.Fatalf("expected latest revision rev-2, got %s", payload.CurrentRevision.ID)
	}
	if payload.Installation.ID == "install-1" {
		t.Fatalf("expected a new installation id after reinstall")
	}
	if payload.Installation.Name != "Research Skill v2" {
		t.Fatalf("expected installation name to default to revision title, got %s", payload.Installation.Name)
	}
	if !payload.Installation.IsDefault {
		t.Fatalf("expected reinstall into empty definition to become default")
	}
}

func TestCreateSkillInstallationFromExistingDefinitionAllowsMultipleInstallations(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{store: mem}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	now := time.Now().UTC()
	skill := domain.Skill{
		ID:           "install-1",
		UserID:       user.ID,
		DefinitionID: "def-1",
		RevisionID:   "rev-1",
		Version:      1,
		Slug:         "summary-skill",
		Kind:         domain.SkillKindChatProfile,
		Title:        "Summary Skill",
		Description:  "summarize things",
		Prompt:       "summarize",
		Mode:         "summary",
		Source:       domain.SkillSourceManual,
		Enabled:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := mem.CreateSkill(context.Background(), skill); err != nil {
		t.Fatalf("create skill: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/skill-installations", strings.NewReader(`{"definitionId":"def-1","currentRevisionId":"rev-1"}`))
	req = req.WithContext(context.WithValue(req.Context(), userKey, user))
	rec := httptest.NewRecorder()

	handler.createSkillInstallation(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload domain.SkillInstallationRecord
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Installation.ID == "install-1" {
		t.Fatalf("expected a distinct installation id for the second install")
	}
	if payload.Installation.DefinitionID != "def-1" {
		t.Fatalf("expected definition def-1, got %s", payload.Installation.DefinitionID)
	}
	if payload.Installation.Name != "Summary Skill" {
		t.Fatalf("expected installation name Summary Skill, got %s", payload.Installation.Name)
	}
	if payload.Installation.IsDefault {
		t.Fatalf("expected second installation to start as non-default")
	}

	defReq := httptest.NewRequest(http.MethodGet, "/v1/skill-definitions/def-1", nil)
	defReq = defReq.WithContext(withUserAndParam(defReq.Context(), user, "definitionID", "def-1"))
	defRec := httptest.NewRecorder()
	handler.getSkillDefinition(defRec, defReq)
	if defRec.Code != http.StatusOK {
		t.Fatalf("expected definition 200, got %d: %s", defRec.Code, defRec.Body.String())
	}

	var details domain.SkillDefinitionDetails
	if err := json.Unmarshal(defRec.Body.Bytes(), &details); err != nil {
		t.Fatalf("decode definition response: %v", err)
	}
	if len(details.Installations) != 2 {
		t.Fatalf("expected 2 installations, got %d", len(details.Installations))
	}
	defaultCount := 0
	for _, installation := range details.Installations {
		if installation.IsDefault {
			defaultCount++
		}
	}
	if defaultCount != 1 {
		t.Fatalf("expected exactly 1 default installation, got %d", defaultCount)
	}
}

func TestCreateSkillInstallationReturnsRevisionNotFoundCode(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{store: mem}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	now := time.Now().UTC()
	skill := domain.Skill{
		ID:           "install-1",
		UserID:       user.ID,
		DefinitionID: "def-1",
		RevisionID:   "rev-1",
		Version:      1,
		Slug:         "summary-skill",
		Kind:         domain.SkillKindChatProfile,
		Title:        "Summary Skill",
		Description:  "summarize things",
		Prompt:       "summarize",
		Mode:         "summary",
		Source:       domain.SkillSourceManual,
		Enabled:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := mem.CreateSkill(context.Background(), skill); err != nil {
		t.Fatalf("create skill: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/skill-installations", strings.NewReader(`{"definitionId":"def-1","currentRevisionId":"rev-missing"}`))
	req = req.WithContext(context.WithValue(req.Context(), userKey, user))
	rec := httptest.NewRecorder()

	handler.createSkillInstallation(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["code"] != errorCodeSkillRevisionNotFound {
		t.Fatalf("expected code %s, got %s", errorCodeSkillRevisionNotFound, payload["code"])
	}
}

func TestCreateSkillRejectsInstallationOnlyFieldsWithCode(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{store: mem}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/skills", strings.NewReader(`{"definitionId":"def-1","title":"Summary Skill","prompt":"summarize"}`))
	req = req.WithContext(context.WithValue(req.Context(), userKey, user))
	rec := httptest.NewRecorder()

	handler.createSkill(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	payload := decodeAPIError(t, rec.Body.Bytes())
	if payload.Code != errorCodeSkillInstallationOnlyFields {
		t.Fatalf("expected code %s, got %s", errorCodeSkillInstallationOnlyFields, payload.Code)
	}
	fields := detailStringList(t, payload, "fields")
	if !containsString(fields, "definitionId") || !containsString(fields, "currentRevisionId") {
		t.Fatalf("expected details.fields to contain definitionId and currentRevisionId, got %#v", fields)
	}
}

func TestCreateSkillReturnsTitlePromptRequiredCode(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{store: mem}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/skills", strings.NewReader(`{"title":"Summary Skill"}`))
	req = req.WithContext(context.WithValue(req.Context(), userKey, user))
	rec := httptest.NewRecorder()

	handler.createSkill(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	payload := decodeAPIError(t, rec.Body.Bytes())
	if payload.Code != errorCodeSkillTitlePromptRequired {
		t.Fatalf("expected code %s, got %s", errorCodeSkillTitlePromptRequired, payload.Code)
	}
	fields := detailStringList(t, payload, "fields")
	if !containsString(fields, "title") || !containsString(fields, "prompt") {
		t.Fatalf("expected details.fields to contain title and prompt, got %#v", fields)
	}
}

func TestCreateSkillReturnsInvalidRequestBodyCode(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{store: mem}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/skills", strings.NewReader("{"))
	req = req.WithContext(context.WithValue(req.Context(), userKey, user))
	rec := httptest.NewRecorder()

	handler.createSkill(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["code"] != errorCodeSkillInvalidRequestBody {
		t.Fatalf("expected code %s, got %s", errorCodeSkillInvalidRequestBody, payload["code"])
	}
}

func TestCreateSkillReturnsConflictCode(t *testing.T) {
	mem := store.NewMemoryStore()
	injected := &errorInjectingStore{
		Store:          mem,
		createSkillErr: store.ErrConflict,
	}
	handler := &Handler{store: injected}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/skills", strings.NewReader(`{"title":"Summary Skill","prompt":"summarize"}`))
	req = req.WithContext(context.WithValue(req.Context(), userKey, user))
	rec := httptest.NewRecorder()

	handler.createSkill(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["code"] != errorCodeSkillConflict {
		t.Fatalf("expected code %s, got %s", errorCodeSkillConflict, payload["code"])
	}
}

func TestCreateSkillMirrorsToPython(t *testing.T) {
	mem := store.NewMemoryStore()
	mirrorClient := &errorMirrorClient{}
	handler := &Handler{
		store:         mem,
		mirrorService: mirror.NewService(mem, mirrorClient),
	}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/skills", strings.NewReader(`{"title":"Summary Skill","prompt":"summarize","mode":"summary"}`))
	req = req.WithContext(context.WithValue(req.Context(), userKey, user))
	rec := httptest.NewRecorder()

	handler.createSkill(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(mirrorClient.upsertedSkills) != 1 {
		t.Fatalf("expected mirror to upsert exactly once, got %d", len(mirrorClient.upsertedSkills))
	}
	if mirrorClient.upsertedSkills[0].Title != "Summary Skill" {
		t.Fatalf("expected mirrored skill title Summary Skill, got %s", mirrorClient.upsertedSkills[0].Title)
	}
}

func TestImportSkillFromUploadCreatesJobArtifactAndInstallation(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{
		store:         mem,
		skillImporter: skillimport.NewService(nil),
	}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	archive := buildSkillImportArchive(t, map[string]string{
		"demo-main/skill.json": `{"slug":"zip-demo","kind":"chat_profile","title":"Zip Demo","description":"zip import","mode":"summary","toolAllowlist":["report.write"]}`,
		"demo-main/SKILL.md":   "Reply using the packaged skill instructions.",
	})
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fileWriter, err := writer.CreateFormFile("file", "zip-demo.zip")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fileWriter.Write(archive); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	if err := writer.WriteField("install", "true"); err != nil {
		t.Fatalf("write install field: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/skill-imports/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req = req.WithContext(context.WithValue(req.Context(), userKey, user))
	rec := httptest.NewRecorder()

	handler.importSkillFromUpload(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var payload skillImportJobDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Job.Status != domain.SkillImportCompleted {
		t.Fatalf("expected completed import job, got %s", payload.Job.Status)
	}
	if payload.Artifact == nil {
		t.Fatalf("expected artifact in response")
	}
	if payload.Artifact.FileName != "zip-demo.zip" {
		t.Fatalf("expected artifact file name zip-demo.zip, got %s", payload.Artifact.FileName)
	}
	skills, err := mem.ListSkills(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("list skills: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 installed skill, got %d", len(skills))
	}
	if skills[0].Slug != "zip-demo" {
		t.Fatalf("expected imported skill slug zip-demo, got %s", skills[0].Slug)
	}
	if skills[0].Prompt != "Reply using the packaged skill instructions." {
		t.Fatalf("expected imported prompt from SKILL.md, got %q", skills[0].Prompt)
	}
	files, err := mem.ListSkillArtifactFiles(context.Background(), user.ID, payload.Artifact.ID)
	if err != nil {
		t.Fatalf("list artifact files: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 indexed artifact files, got %d", len(files))
	}
	if files[0].Path != "SKILL.md" || !files[0].IsInstructions {
		t.Fatalf("expected SKILL.md to be indexed as instructions, got %#v", files[0])
	}
	if files[1].Path != "skill.json" || !files[1].IsManifest {
		t.Fatalf("expected skill.json to be indexed as manifest, got %#v", files[1])
	}
}

func TestImportSkillFromUploadReturnsSkillConflictCodeOnDuplicateSlug(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{
		store:         mem,
		skillImporter: skillimport.NewService(nil),
	}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	archive := buildSkillImportArchive(t, map[string]string{
		"demo-main/skill.json": `{"slug":"zip-demo","kind":"chat_profile","title":"Zip Demo","description":"zip import","mode":"summary"}`,
		"demo-main/SKILL.md":   "Reply using the packaged skill instructions.",
	})

	buildRequest := func() *http.Request {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		fileWriter, err := writer.CreateFormFile("file", "zip-demo.zip")
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		if _, err := fileWriter.Write(archive); err != nil {
			t.Fatalf("write archive: %v", err)
		}
		if err := writer.WriteField("install", "true"); err != nil {
			t.Fatalf("write install field: %v", err)
		}
		if err := writer.Close(); err != nil {
			t.Fatalf("close multipart writer: %v", err)
		}

		req := httptest.NewRequest(http.MethodPost, "/v1/skill-imports/upload", &body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		return req.WithContext(context.WithValue(req.Context(), userKey, user))
	}

	firstRec := httptest.NewRecorder()
	handler.importSkillFromUpload(firstRec, buildRequest())
	if firstRec.Code != http.StatusCreated {
		t.Fatalf("expected first import 201, got %d: %s", firstRec.Code, firstRec.Body.String())
	}

	secondRec := httptest.NewRecorder()
	handler.importSkillFromUpload(secondRec, buildRequest())
	if secondRec.Code != http.StatusConflict {
		t.Fatalf("expected second import 409, got %d: %s", secondRec.Code, secondRec.Body.String())
	}

	payload := decodeAPIError(t, secondRec.Body.Bytes())
	if payload.Code != errorCodeSkillConflict {
		t.Fatalf("expected code %s, got %s", errorCodeSkillConflict, payload.Code)
	}
	if payload.Error != "skill already exists" {
		t.Fatalf("expected message skill already exists, got %q", payload.Error)
	}
}

func TestSkillArtifactHandlersListFilesAndReadContent(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{store: mem}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	archive := buildSkillImportArchive(t, map[string]string{
		"demo-main/skill.json":          `{"slug":"zip-demo","title":"Zip Demo","mode":"summary"}`,
		"demo-main/SKILL.md":            "Reply using the packaged skill instructions.",
		"demo-main/resources/guide.txt": "guide body",
	})
	pkg, err := skillimport.NewService(nil).ImportUploadedArchive("zip-demo.zip", "application/zip", "", archive)
	if err != nil {
		t.Fatalf("parse uploaded archive: %v", err)
	}
	artifact := domain.SkillArtifact{
		ID:               "artifact-1",
		UserID:           user.ID,
		DefinitionID:     "def-1",
		RevisionID:       "rev-1",
		Source:           domain.SkillSourceUpload,
		FileName:         "zip-demo.zip",
		MediaType:        "application/zip",
		SHA256:           pkg.SHA256,
		SizeBytes:        pkg.SizeBytes,
		EntryPath:        pkg.EntryPath,
		ManifestPath:     pkg.ManifestPath,
		InstructionsPath: pkg.InstructionsPath,
		ArchiveBytes:     append([]byte(nil), pkg.ArchiveBytes...),
		CreatedAt:        time.Now().UTC(),
	}
	if err := mem.CreateSkillArtifact(context.Background(), artifact); err != nil {
		t.Fatalf("create skill artifact: %v", err)
	}
	files := make([]domain.SkillArtifactFile, 0, len(pkg.Files))
	for i, item := range pkg.Files {
		files = append(files, domain.SkillArtifactFile{
			ID:             "file-" + strconv.Itoa(i+1),
			ArtifactID:     artifact.ID,
			UserID:         user.ID,
			Path:           item.Path,
			MediaType:      item.MediaType,
			SizeBytes:      item.SizeBytes,
			SHA256:         item.SHA256,
			IsManifest:     item.IsManifest,
			IsInstructions: item.IsInstructions,
			CreatedAt:      time.Now().UTC(),
		})
	}
	if err := mem.ReplaceSkillArtifactFiles(context.Background(), artifact.ID, files); err != nil {
		t.Fatalf("replace skill artifact files: %v", err)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/skill-artifacts/artifact-1/files", nil)
	listReq = listReq.WithContext(withUserAndParam(listReq.Context(), user, "artifactID", artifact.ID))
	listRec := httptest.NewRecorder()

	handler.listSkillArtifactFiles(listRec, listReq)

	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", listRec.Code, listRec.Body.String())
	}
	var listPayload struct {
		Items []domain.SkillArtifactFile `json:"items"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listPayload); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listPayload.Items) != 3 {
		t.Fatalf("expected 3 artifact files, got %d", len(listPayload.Items))
	}
	if listPayload.Items[0].Path != "SKILL.md" {
		t.Fatalf("expected sorted files, got first path %s", listPayload.Items[0].Path)
	}

	contentReq := httptest.NewRequest(http.MethodGet, "/v1/skill-artifacts/artifact-1/content?path=resources/guide.txt", nil)
	contentReq = contentReq.WithContext(withUserAndParam(contentReq.Context(), user, "artifactID", artifact.ID))
	contentRec := httptest.NewRecorder()

	handler.getSkillArtifactFileContent(contentRec, contentReq)

	if contentRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", contentRec.Code, contentRec.Body.String())
	}
	if body := contentRec.Body.String(); body != "guide body" {
		t.Fatalf("expected guide body, got %q", body)
	}
	if mediaType := contentRec.Header().Get("Content-Type"); mediaType != "text/plain" {
		t.Fatalf("expected text/plain content type, got %s", mediaType)
	}
	if pathHeader := contentRec.Header().Get("X-Skill-Artifact-Path"); pathHeader != "resources/guide.txt" {
		t.Fatalf("expected artifact path header resources/guide.txt, got %s", pathHeader)
	}
}

func TestCreateSkillInstallationRejectsInvalidFieldCombinationWithCode(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{store: mem}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/skill-installations", strings.NewReader(`{"definitionId":"def-1","title":"Summary Skill","prompt":"summarize"}`))
	req = req.WithContext(context.WithValue(req.Context(), userKey, user))
	rec := httptest.NewRecorder()

	handler.createSkillInstallation(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	payload := decodeAPIError(t, rec.Body.Bytes())
	if payload.Code != errorCodeSkillInvalidFieldCombination {
		t.Fatalf("expected code %s, got %s", errorCodeSkillInvalidFieldCombination, payload.Code)
	}
	fields := detailStringList(t, payload, "fields")
	if !containsString(fields, "definitionId") || !containsString(fields, "title") || !containsString(fields, "prompt") {
		t.Fatalf("expected details.fields to contain definitionId/title/prompt, got %#v", fields)
	}
}

func TestCreateSkillInstallationReturnsDefinitionRequiredCode(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{store: mem}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/skill-installations", strings.NewReader(`{"currentRevisionId":"rev-1"}`))
	req = req.WithContext(context.WithValue(req.Context(), userKey, user))
	rec := httptest.NewRecorder()

	handler.createSkillInstallation(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["code"] != errorCodeSkillDefinitionRequired {
		t.Fatalf("expected code %s, got %s", errorCodeSkillDefinitionRequired, payload["code"])
	}
}

func TestCreateSkillInstallationReturnsInternalServerErrorOnDefinitionLookupFailure(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{store: &errorInjectingStore{
		Store:                        mem,
		getSkillDefinitionDetailsErr: errors.New("boom"),
	}}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/skill-installations", strings.NewReader(`{"definitionId":"def-1"}`))
	req = req.WithContext(context.WithValue(req.Context(), userKey, user))
	rec := httptest.NewRecorder()

	handler.createSkillInstallation(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
	payload := decodeAPIError(t, rec.Body.Bytes())
	if payload.Code != errorCodeInternalError {
		t.Fatalf("expected code %s, got %s", errorCodeInternalError, payload.Code)
	}
	if payload.Error != "boom" {
		t.Fatalf("expected error boom, got %s", payload.Error)
	}
}

func TestUpdateSkillReturnsNoUpdatableFieldsCode(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{store: mem}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	now := time.Now().UTC()
	skill := domain.Skill{
		ID:           "install-1",
		UserID:       user.ID,
		DefinitionID: "def-1",
		RevisionID:   "rev-1",
		Version:      1,
		Slug:         "summary-skill",
		Kind:         domain.SkillKindChatProfile,
		Title:        "Summary Skill",
		Description:  "summarize things",
		Prompt:       "summarize",
		Mode:         "summary",
		Source:       domain.SkillSourceManual,
		Enabled:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := mem.CreateSkill(context.Background(), skill); err != nil {
		t.Fatalf("create skill: %v", err)
	}

	req := httptest.NewRequest(http.MethodPatch, "/v1/skills/install-1", strings.NewReader(`{}`))
	req = req.WithContext(withUserAndParam(req.Context(), user, "skillID", "install-1"))
	rec := httptest.NewRecorder()

	handler.updateSkill(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	payload := decodeAPIError(t, rec.Body.Bytes())
	if payload.Code != errorCodeSkillNoUpdatableFields {
		t.Fatalf("expected code %s, got %s", errorCodeSkillNoUpdatableFields, payload.Code)
	}
	allowedFields := detailStringList(t, payload, "allowedFields")
	if !containsString(allowedFields, "prompt") || !containsString(allowedFields, "isDefault") {
		t.Fatalf("expected details.allowedFields to contain prompt and isDefault, got %#v", allowedFields)
	}
}

func TestUpdateSkillMirrorsRevisionPatchToPython(t *testing.T) {
	mem := store.NewMemoryStore()
	mirrorClient := &errorMirrorClient{}
	handler := &Handler{
		store:         mem,
		mirrorService: mirror.NewService(mem, mirrorClient),
	}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	now := time.Now().UTC()
	skill := domain.Skill{
		ID:           "install-1",
		UserID:       user.ID,
		DefinitionID: "def-1",
		RevisionID:   "rev-1",
		Version:      1,
		Slug:         "summary-skill",
		Kind:         domain.SkillKindChatProfile,
		Title:        "Summary Skill",
		Description:  "summarize things",
		Prompt:       "summarize",
		Mode:         "summary",
		Source:       domain.SkillSourceManual,
		Enabled:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := mem.CreateSkill(context.Background(), skill); err != nil {
		t.Fatalf("create skill: %v", err)
	}

	req := httptest.NewRequest(http.MethodPatch, "/v1/skills/install-1", strings.NewReader(`{"prompt":"answer tersely"}`))
	req = req.WithContext(withUserAndParam(req.Context(), user, "skillID", "install-1"))
	rec := httptest.NewRecorder()

	handler.updateSkill(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(mirrorClient.upsertedSkills) != 1 {
		t.Fatalf("expected mirror to upsert exactly once, got %d", len(mirrorClient.upsertedSkills))
	}
	if mirrorClient.upsertedSkills[0].Prompt != "answer tersely" {
		t.Fatalf("expected mirrored prompt to be updated, got %s", mirrorClient.upsertedSkills[0].Prompt)
	}
}

func TestUpdateSkillReturnsTitlePromptRequiredCode(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{store: mem}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	now := time.Now().UTC()
	skill := domain.Skill{
		ID:           "install-1",
		UserID:       user.ID,
		DefinitionID: "def-1",
		RevisionID:   "rev-1",
		Version:      1,
		Slug:         "summary-skill",
		Kind:         domain.SkillKindChatProfile,
		Title:        "Summary Skill",
		Description:  "summarize things",
		Prompt:       "summarize",
		Mode:         "summary",
		Source:       domain.SkillSourceManual,
		Enabled:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := mem.CreateSkill(context.Background(), skill); err != nil {
		t.Fatalf("create skill: %v", err)
	}

	req := httptest.NewRequest(http.MethodPatch, "/v1/skills/install-1", strings.NewReader(`{"prompt":"   "}`))
	req = req.WithContext(withUserAndParam(req.Context(), user, "skillID", "install-1"))
	rec := httptest.NewRecorder()

	handler.updateSkill(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	payload := decodeAPIError(t, rec.Body.Bytes())
	if payload.Code != errorCodeSkillTitlePromptRequired {
		t.Fatalf("expected code %s, got %s", errorCodeSkillTitlePromptRequired, payload.Code)
	}
	fields := detailStringList(t, payload, "fields")
	if !containsString(fields, "title") || !containsString(fields, "prompt") {
		t.Fatalf("expected details.fields to contain title and prompt, got %#v", fields)
	}
}

func TestUpdateSkillReturnsInvalidRequestBodyCode(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{store: mem}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	req := httptest.NewRequest(http.MethodPatch, "/v1/skills/install-1", strings.NewReader("{"))
	req = req.WithContext(withUserAndParam(req.Context(), user, "skillID", "install-1"))
	rec := httptest.NewRecorder()

	handler.updateSkill(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["code"] != errorCodeSkillInvalidRequestBody {
		t.Fatalf("expected code %s, got %s", errorCodeSkillInvalidRequestBody, payload["code"])
	}
}

func TestUpdateSkillReturnsInternalServerErrorOnSkillLookupFailure(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{store: &errorInjectingStore{
		Store:       mem,
		getSkillErr: errors.New("boom"),
	}}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	req := httptest.NewRequest(http.MethodPatch, "/v1/skills/install-1", strings.NewReader(`{"title":"Updated"}`))
	req = req.WithContext(withUserAndParam(req.Context(), user, "skillID", "install-1"))
	rec := httptest.NewRecorder()

	handler.updateSkill(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
	payload := decodeAPIError(t, rec.Body.Bytes())
	if payload.Code != errorCodeInternalError {
		t.Fatalf("expected code %s, got %s", errorCodeInternalError, payload.Code)
	}
	if payload.Error != "boom" {
		t.Fatalf("expected error boom, got %s", payload.Error)
	}
}

func TestUpdateSkillInstallationReturnsRevisionRequiredCode(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{store: mem}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	now := time.Now().UTC()
	skill := domain.Skill{
		ID:           "install-1",
		UserID:       user.ID,
		DefinitionID: "def-1",
		RevisionID:   "rev-1",
		Version:      1,
		Slug:         "summary-skill",
		Kind:         domain.SkillKindChatProfile,
		Title:        "Summary Skill",
		Description:  "summarize things",
		Prompt:       "summarize",
		Mode:         "summary",
		Source:       domain.SkillSourceManual,
		Enabled:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := mem.CreateSkill(context.Background(), skill); err != nil {
		t.Fatalf("create skill: %v", err)
	}

	req := httptest.NewRequest(http.MethodPatch, "/v1/skill-installations/install-1", strings.NewReader(`{"currentRevisionId":"   "}`))
	req = req.WithContext(withUserAndParam(req.Context(), user, "installationID", "install-1"))
	rec := httptest.NewRecorder()

	handler.updateSkillInstallation(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["code"] != errorCodeSkillRevisionRequired {
		t.Fatalf("expected code %s, got %s", errorCodeSkillRevisionRequired, payload["code"])
	}
}

func TestUpdateSkillInstallationReturnsInternalServerErrorOnInstallationLookupFailure(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{store: &errorInjectingStore{
		Store:                         mem,
		getSkillInstallationRecordErr: errors.New("boom"),
	}}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	req := httptest.NewRequest(http.MethodPatch, "/v1/skill-installations/install-1", strings.NewReader(`{"name":"Renamed"}`))
	req = req.WithContext(withUserAndParam(req.Context(), user, "installationID", "install-1"))
	rec := httptest.NewRecorder()

	handler.updateSkillInstallation(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
	payload := decodeAPIError(t, rec.Body.Bytes())
	if payload.Code != errorCodeInternalError {
		t.Fatalf("expected code %s, got %s", errorCodeInternalError, payload.Code)
	}
	if payload.Error != "boom" {
		t.Fatalf("expected error boom, got %s", payload.Error)
	}
}

func TestDeleteDefaultInstallationPromotesAnotherDefault(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{store: mem}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	now := time.Now().UTC()
	skill := domain.Skill{
		ID:           "install-1",
		UserID:       user.ID,
		DefinitionID: "def-1",
		RevisionID:   "rev-1",
		Version:      1,
		Slug:         "summary-skill",
		Kind:         domain.SkillKindChatProfile,
		Title:        "Summary Skill",
		Description:  "summarize things",
		Prompt:       "summarize",
		Mode:         "summary",
		Source:       domain.SkillSourceManual,
		Enabled:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := mem.CreateSkill(context.Background(), skill); err != nil {
		t.Fatalf("create skill: %v", err)
	}

	createReq := httptest.NewRequest(http.MethodPost, "/v1/skill-installations", strings.NewReader(`{"definitionId":"def-1","name":"staging"}`))
	createReq = createReq.WithContext(context.WithValue(createReq.Context(), userKey, user))
	createRec := httptest.NewRecorder()
	handler.createSkillInstallation(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", createRec.Code, createRec.Body.String())
	}

	var created domain.SkillInstallationRecord
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/v1/skill-installations/install-1", nil)
	deleteReq = deleteReq.WithContext(withUserAndParam(deleteReq.Context(), user, "installationID", "install-1"))
	deleteRec := httptest.NewRecorder()
	handler.deleteSkillInstallation(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("expected delete 204, got %d: %s", deleteRec.Code, deleteRec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/v1/skill-installations/"+created.Installation.ID, nil)
	getReq = getReq.WithContext(withUserAndParam(getReq.Context(), user, "installationID", created.Installation.ID))
	getRec := httptest.NewRecorder()
	handler.getSkillInstallation(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected get 200, got %d: %s", getRec.Code, getRec.Body.String())
	}

	var payload domain.SkillInstallationRecord
	if err := json.Unmarshal(getRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if !payload.Installation.IsDefault {
		t.Fatalf("expected remaining installation to be promoted to default")
	}
}

func TestSyncSkillMirrorHelperCallsMirrorService(t *testing.T) {
	mem := store.NewMemoryStore()
	mirrorClient := &errorMirrorClient{}
	handler := &Handler{
		store:         mem,
		mirrorService: mirror.NewService(mem, mirrorClient),
	}

	err := handler.syncSkillMirror(context.Background(), domain.Skill{
		ID:     "install-1",
		UserID: "user-1",
		Title:  "Summary Skill",
		Prompt: "summarize",
	})
	if err != nil {
		t.Fatalf("sync mirror: %v", err)
	}
	if len(mirrorClient.upsertedSkills) != 1 {
		t.Fatalf("expected helper to mirror exactly once, got %d", len(mirrorClient.upsertedSkills))
	}
}

func TestResolveChatSkillSelectionByDefinitionUsesDefaultInstallation(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{store: mem}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	now := time.Now().UTC()
	skill := domain.Skill{
		ID:           "install-1",
		UserID:       user.ID,
		DefinitionID: "def-1",
		RevisionID:   "rev-1",
		Version:      1,
		Slug:         "summary-skill",
		Kind:         domain.SkillKindChatProfile,
		Title:        "Summary Skill",
		Description:  "summarize things",
		Prompt:       "summarize",
		Mode:         "summary",
		Source:       domain.SkillSourceManual,
		Enabled:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := mem.CreateSkill(context.Background(), skill); err != nil {
		t.Fatalf("create skill: %v", err)
	}

	secondInstallation := domain.SkillInstallation{
		ID:                "install-2",
		UserID:            user.ID,
		DefinitionID:      "def-1",
		CurrentRevisionID: "rev-1",
		Name:              "staging",
		IsDefault:         false,
		Enabled:           true,
		CreatedAt:         now.Add(time.Minute),
		UpdatedAt:         now.Add(time.Minute),
	}
	if err := mem.CreateSkillInstallation(context.Background(), secondInstallation); err != nil {
		t.Fatalf("create second installation: %v", err)
	}

	resolvedID, resolvedSkill, err := handler.resolveChatSkillSelection(context.Background(), user.ID, "", "def-1")
	if err != nil {
		t.Fatalf("resolve selection: %v", err)
	}
	if resolvedID != "install-1" {
		t.Fatalf("expected default installation install-1, got %s", resolvedID)
	}
	if resolvedSkill.ID != "install-1" {
		t.Fatalf("expected resolved skill install-1, got %s", resolvedSkill.ID)
	}
	if resolvedSkill.DefinitionID != "def-1" {
		t.Fatalf("expected definition def-1, got %s", resolvedSkill.DefinitionID)
	}
}

func TestCreateRunReturnsSkillDefinitionNoEnabledInstallationCode(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{
		store:      mem,
		runService: run.NewService(mem, nil, run.NewEventBroker(), run.NewPlanner(), nil, nil, nil, nil, nil),
	}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	now := time.Now().UTC()
	skill := domain.Skill{
		ID:           "install-1",
		UserID:       user.ID,
		DefinitionID: "def-1",
		RevisionID:   "rev-1",
		Version:      1,
		Slug:         "summary-skill",
		Kind:         domain.SkillKindChatProfile,
		Title:        "Summary Skill",
		Description:  "summarize things",
		Prompt:       "summarize",
		Mode:         "summary",
		Source:       domain.SkillSourceManual,
		Enabled:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := mem.CreateSkill(context.Background(), skill); err != nil {
		t.Fatalf("create skill: %v", err)
	}

	record, err := mem.GetSkillInstallationRecord(context.Background(), user.ID, "install-1")
	if err != nil {
		t.Fatalf("get installation: %v", err)
	}
	record.Installation.Enabled = false
	record.Installation.UpdatedAt = now.Add(time.Minute)
	if err := mem.UpdateSkillInstallation(context.Background(), record.Installation); err != nil {
		t.Fatalf("disable installation: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/runs", strings.NewReader(`{"goal":"Summarize internal docs","skill_definition_id":"def-1"}`))
	req = req.WithContext(context.WithValue(req.Context(), userKey, user))
	rec := httptest.NewRecorder()

	handler.createRun(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["code"] != errorCodeNoEnabledSkillInstallation {
		t.Fatalf("expected code %s, got %s", errorCodeNoEnabledSkillInstallation, payload["code"])
	}
}

func TestChatStreamReturnsDisabledSkillInstallationCode(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{
		store:       mem,
		chatService: chat.NewService(nil, nil, nil),
	}
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	now := time.Now().UTC()
	skill := domain.Skill{
		ID:           "install-1",
		UserID:       user.ID,
		DefinitionID: "def-1",
		RevisionID:   "rev-1",
		Version:      1,
		Slug:         "summary-skill",
		Kind:         domain.SkillKindChatProfile,
		Title:        "Summary Skill",
		Description:  "summarize things",
		Prompt:       "summarize",
		Mode:         "summary",
		Source:       domain.SkillSourceManual,
		Enabled:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := mem.CreateSkill(context.Background(), skill); err != nil {
		t.Fatalf("create skill: %v", err)
	}

	record, err := mem.GetSkillInstallationRecord(context.Background(), user.ID, "install-1")
	if err != nil {
		t.Fatalf("get installation: %v", err)
	}
	record.Installation.Enabled = false
	record.Installation.UpdatedAt = now.Add(time.Minute)
	if err := mem.UpdateSkillInstallation(context.Background(), record.Installation); err != nil {
		t.Fatalf("disable installation: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/chat/stream?message=hello&skill_id=install-1", nil)
	req = req.WithContext(context.WithValue(req.Context(), userKey, user))
	rec := httptest.NewRecorder()

	handler.chatStream(rec, req)

	if !strings.Contains(rec.Body.String(), `"code":"`+errorCodeSkillInstallationDisabled+`"`) {
		t.Fatalf("expected SSE body to contain disabled code, got %s", rec.Body.String())
	}
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
	store.Store
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
	store.Store
	lastSnapshot *domain.SkillRuntimeSnapshot
}

func (s *errorInjectingStore) CreateSkill(ctx context.Context, skill domain.Skill) error {
	if s.createSkillErr != nil {
		return s.createSkillErr
	}
	return s.Store.CreateSkill(ctx, skill)
}

func (s *errorInjectingStore) CreateChatSession(ctx context.Context, session domain.ChatSession) error {
	if s.createChatSessionErr != nil {
		return s.createChatSessionErr
	}
	return s.Store.CreateChatSession(ctx, session)
}

func (s *errorInjectingStore) GetSkill(ctx context.Context, userID, skillID string) (domain.Skill, error) {
	if s.getSkillErr != nil {
		return domain.Skill{}, s.getSkillErr
	}
	return s.Store.GetSkill(ctx, userID, skillID)
}

func (s *errorInjectingStore) GetSkillDefinitionDetails(ctx context.Context, userID, definitionID string) (domain.SkillDefinitionDetails, error) {
	if s.getSkillDefinitionDetailsErr != nil {
		return domain.SkillDefinitionDetails{}, s.getSkillDefinitionDetailsErr
	}
	return s.Store.GetSkillDefinitionDetails(ctx, userID, definitionID)
}

func (s *errorInjectingStore) GetSkillInstallationRecord(ctx context.Context, userID, installationID string) (domain.SkillInstallationRecord, error) {
	if s.getSkillInstallationRecordErr != nil {
		return domain.SkillInstallationRecord{}, s.getSkillInstallationRecordErr
	}
	return s.Store.GetSkillInstallationRecord(ctx, userID, installationID)
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
	return s.Store.CreateSkillRuntimeSnapshot(ctx, snapshot)
}
