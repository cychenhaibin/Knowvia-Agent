package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/store"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/mirror"
)

func TestCreateSkillRejectsInstallationOnlyFieldsWithCode(t *testing.T) {
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
	handler := newTestHandler(mem)
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
	handler := &Handler{}
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
		fullTestStore:  mem,
		createSkillErr: store.ErrConflict,
	}
	handler := newTestHandler(injected)
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
	mirrorService := mirror.NewService(mem, mirrorClient)
	handler := &Handler{
		skillService: newSkillServiceForTest(mem, mirrorService),
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

func TestUpdateSkillReturnsNoUpdatableFieldsCode(t *testing.T) {
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
	mirrorService := mirror.NewService(mem, mirrorClient)
	handler := &Handler{
		skillService: newSkillServiceForTest(mem, mirrorService),
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
	handler := newTestHandler(mem)
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
	handler := newTestHandler(mem)
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
	injected := &errorInjectingStore{
		fullTestStore: mem,
		getSkillErr:   errors.New("boom"),
	}
	handler := newTestHandler(injected)
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
