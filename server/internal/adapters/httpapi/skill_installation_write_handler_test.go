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
)

func TestUpdateSkillInstallationCanSwitchRevision(t *testing.T) {
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

	var payload skillInstallationRecordDTO
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

	var payload skillInstallationRecordDTO
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

	req := httptest.NewRequest(http.MethodPost, "/v1/skill-installations", strings.NewReader(`{"definitionId":"def-1","currentRevisionId":"rev-1"}`))
	req = req.WithContext(context.WithValue(req.Context(), userKey, user))
	rec := httptest.NewRecorder()

	handler.createSkillInstallation(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload skillInstallationRecordDTO
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

	var details skillDefinitionDetailsDTO
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

func TestCreateSkillInstallationRejectsInvalidFieldCombinationWithCode(t *testing.T) {
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
	handler := newTestHandler(mem)
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
	injected := &errorInjectingStore{
		fullTestStore:                mem,
		getSkillDefinitionDetailsErr: errors.New("boom"),
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

func TestUpdateSkillInstallationReturnsRevisionRequiredCode(t *testing.T) {
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
	injected := &errorInjectingStore{
		fullTestStore:                 mem,
		getSkillInstallationRecordErr: errors.New("boom"),
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
