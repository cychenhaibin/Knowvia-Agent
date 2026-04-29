package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/store"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skillimport"
)

func TestImportSkillFromUploadCreatesJobArtifactAndInstallation(t *testing.T) {
	mem := store.NewMemoryStore()
	parser := skillimport.NewService(nil)
	handler := &Handler{
		skillService:  newSkillServiceForTest(mem, nil),
		skillImporter: newSkillImportServiceForTest(mem, parser, nil),
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
	var payload skillImportJobDetailsDTO
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
	parser := skillimport.NewService(nil)
	handler := &Handler{
		skillService:  newSkillServiceForTest(mem, nil),
		skillImporter: newSkillImportServiceForTest(mem, parser, nil),
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
