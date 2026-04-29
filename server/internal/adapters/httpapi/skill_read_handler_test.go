package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/store"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skillimport"
)

func TestGetSkillDefinitionReturnsDetails(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodGet, "/v1/skill-definitions/def-1", nil)
	req = req.WithContext(withUserAndParam(req.Context(), user, "definitionID", "def-1"))
	rec := httptest.NewRecorder()

	handler.getSkillDefinition(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload skillDefinitionDetailsDTO
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

func TestGetSkillDefinitionReturnsNotFound(t *testing.T) {
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

	var payload skillInstallationListDTO
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

	req := httptest.NewRequest(http.MethodGet, "/v1/skill-installations/install-1", nil)
	req = req.WithContext(withUserAndParam(req.Context(), user, "installationID", "install-1"))
	rec := httptest.NewRecorder()

	handler.getSkillInstallation(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload skillInstallationRecordDTO
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
	handler := newTestHandler(mem)
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

func TestSkillArtifactHandlersListFilesAndReadContent(t *testing.T) {
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

func TestDeleteDefaultInstallationPromotesAnotherDefault(t *testing.T) {
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

	createReq := httptest.NewRequest(http.MethodPost, "/v1/skill-installations", strings.NewReader(`{"definitionId":"def-1","name":"staging"}`))
	createReq = createReq.WithContext(context.WithValue(createReq.Context(), userKey, user))
	createRec := httptest.NewRecorder()
	handler.createSkillInstallation(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", createRec.Code, createRec.Body.String())
	}

	var created skillInstallationRecordDTO
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

	var payload skillInstallationRecordDTO
	if err := json.Unmarshal(getRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if !payload.Installation.IsDefault {
		t.Fatalf("expected remaining installation to be promoted to default")
	}
}
