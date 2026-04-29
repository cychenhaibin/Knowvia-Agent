package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/store"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/run"
)

func TestCreateRunReturnsInvalidRequestBodyCode(t *testing.T) {
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
	handler := newTestHandler(mem)
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

func TestGetRunReturnsNotFoundCode(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{
		runService: run.NewService(run.ServiceDeps{
			Runs:      mem,
			Steps:     mem,
			Artifacts: mem,
			Sources:   mem,
			Skills:    mem,
			Selection: mem,
		}, nil, run.NewEventBroker(), run.NewPlanner(), nil, nil, nil, nil, nil),
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

func TestCreateRunReturnsSkillDefinitionNoEnabledInstallationCode(t *testing.T) {
	mem := store.NewMemoryStore()
	handler := &Handler{
		runService: run.NewService(run.ServiceDeps{
			Runs:      mem,
			Steps:     mem,
			Artifacts: mem,
			Sources:   mem,
			Skills:    mem,
			Selection: mem,
		}, nil, run.NewEventBroker(), run.NewPlanner(), nil, nil, nil, nil, nil),
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
