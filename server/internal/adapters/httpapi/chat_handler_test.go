package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/store"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

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
		chatService: newChatServiceForTest(mem, nil),
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
	injected := &errorInjectingStore{
		fullTestStore:        mem,
		createChatSessionErr: errors.New("boom"),
	}
	handler := &Handler{
		chatService: newChatServiceForTest(injected, nil),
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
	recordingStore := &snapshotRecordingStore{fullTestStore: mem}
	handler := &Handler{
		chatService: newChatServiceForTest(recordingStore, nil),
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
		chatService: newChatServiceForTest(mem, &errorForwardClient{err: errors.New("python knowledge stream request failed: 502 Bad Gateway")}),
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
		chatService: newChatServiceForTest(mem, &errorForwardClient{err: errors.New("python forward client is not configured")}),
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

func TestCreateChatSessionReturnsInvalidRequestBodyCode(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := newTestHandler(mem)
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
		chatService: newChatServiceForTest(mem, nil),
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
	handler := newTestHandler(mem)
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

func TestChatStreamReturnsDisabledSkillInstallationCode(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{
		chatService: newChatServiceForTest(mem, nil),
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
