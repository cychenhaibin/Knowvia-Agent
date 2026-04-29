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

func TestCreateYuqueConnectionReturnsInvalidRequestBodyCode(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{knowledgeService: newKnowledgeServiceForTest(mem, nil, nil)}
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
	handler := &Handler{knowledgeService: newKnowledgeServiceForTest(mem, nil, nil)}
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
	handler := &Handler{knowledgeService: newKnowledgeServiceForTest(mem, nil, nil)}
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
		knowledgeService: newKnowledgeServiceForTest(mem, nil, &errorMirrorClient{deleteKnowledgeErr: errors.New("mirror boom")}),
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
