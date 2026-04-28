package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/chat"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skillartifact"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skillimport"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skillresolver"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skillruntime"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/store"
)

type createSkillRequest struct {
	DefinitionID      string            `json:"definitionId"`
	CurrentRevisionID string            `json:"currentRevisionId"`
	Name              string            `json:"name"`
	IsDefault         *bool             `json:"isDefault"`
	Slug              string            `json:"slug"`
	Kind              string            `json:"kind"`
	Title             string            `json:"title"`
	Description       string            `json:"description"`
	Prompt            string            `json:"prompt"`
	Mode              string            `json:"mode"`
	Source            string            `json:"source"`
	Enabled           *bool             `json:"enabled"`
	RepoURL           string            `json:"repoUrl"`
	PlannerPolicy     map[string]string `json:"plannerPolicy"`
	ToolAllowlist     []string          `json:"toolAllowlist"`
}

type updateSkillRequest struct {
	CurrentRevisionID *string            `json:"currentRevisionId"`
	Name              *string            `json:"name"`
	IsDefault         *bool              `json:"isDefault"`
	Slug              *string            `json:"slug"`
	Kind              *string            `json:"kind"`
	Title             *string            `json:"title"`
	Description       *string            `json:"description"`
	Prompt            *string            `json:"prompt"`
	Mode              *string            `json:"mode"`
	Source            *string            `json:"source"`
	Enabled           *bool              `json:"enabled"`
	RepoURL           *string            `json:"repoUrl"`
	PlannerPolicy     *map[string]string `json:"plannerPolicy"`
	ToolAllowlist     *[]string          `json:"toolAllowlist"`
}

type importSkillGitHubRequest struct {
	RepoURL          string `json:"repoUrl"`
	Ref              string `json:"ref"`
	Path             string `json:"path"`
	Install          *bool  `json:"install"`
	InstallationName string `json:"installationName"`
	IsDefault        *bool  `json:"isDefault"`
	Enabled          *bool  `json:"enabled"`
}

type skillImportJobDetails struct {
	Job      domain.SkillImportJob `json:"job"`
	Artifact *domain.SkillArtifact `json:"artifact,omitempty"`
}

func (h *Handler) createSkill(w http.ResponseWriter, r *http.Request) {
	req, err := decodeCreateSkillRequest(r)
	if err != nil {
		writeSkillInvalidRequestBody(w)
		return
	}
	if isExistingDefinitionInstallationRequest(req) {
		writeSkillInstallationOnlyFields(w, "definitionId and currentRevisionId are only supported for skill installations", []string{"definitionId", "currentRevisionId"})
		return
	}
	if req.Name != "" || req.IsDefault != nil {
		writeSkillInstallationOnlyFields(w, "name and isDefault are only supported for skill installations", []string{"name", "isDefault"})
		return
	}
	skill, err := h.createSkillModel(r, req)
	if err != nil {
		if errors.Is(err, errSkillTitlePromptRequired) {
			writeSkillTitlePromptRequired(w)
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.store.CreateSkill(r.Context(), skill); err != nil {
		if errors.Is(err, store.ErrConflict) {
			writeSkillConflict(w)
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	storedSkill, err := h.store.GetSkill(r.Context(), skill.UserID, skill.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.syncSkillMirror(r.Context(), storedSkill); err != nil {
		writeMirrorError(w, "skill was created locally but python mirror failed", err)
		return
	}
	writeJSON(w, http.StatusCreated, skill)
}

func (h *Handler) createSkillInstallation(w http.ResponseWriter, r *http.Request) {
	req, err := decodeCreateSkillRequest(r)
	if err != nil {
		writeSkillInvalidRequestBody(w)
		return
	}
	if isExistingDefinitionInstallationRequest(req) {
		if hasNewSkillCreateFields(req) {
			writeSkillInvalidFieldCombination(w, "definitionId/currentRevisionId cannot be combined with new skill fields", presentCreateSkillRequestFields(req))
			return
		}
		installation, err := h.createSkillInstallationModel(r, req)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeSkillDefinitionNotFound(w)
				return
			}
			if errors.Is(err, errSkillDefinitionRequired) {
				writeSkillDefinitionRequired(w)
				return
			}
			if errors.Is(err, errSkillRevisionNotFound) {
				writeSkillRevisionNotFound(w, http.StatusBadRequest)
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if err := h.store.CreateSkillInstallation(r.Context(), installation); err != nil {
			switch {
			case errors.Is(err, store.ErrNotFound):
				writeSkillDefinitionNotFound(w)
			case errors.Is(err, store.ErrConflict):
				writeSkillInstallationConflict(w)
			default:
				writeError(w, http.StatusInternalServerError, err.Error())
			}
			return
		}
		record, err := h.store.GetSkillInstallationRecord(r.Context(), installation.UserID, installation.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		storedSkill, err := h.store.GetSkill(r.Context(), installation.UserID, installation.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if err := h.syncSkillMirror(r.Context(), storedSkill); err != nil {
			writeMirrorError(w, "skill was created locally but python mirror failed", err)
			return
		}
		writeJSON(w, http.StatusCreated, record)
		return
	}

	skill, err := h.createSkillModel(r, req)
	if err != nil {
		if errors.Is(err, errSkillTitlePromptRequired) {
			writeSkillTitlePromptRequired(w)
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.store.CreateSkill(r.Context(), skill); err != nil {
		if errors.Is(err, store.ErrConflict) {
			writeSkillConflict(w)
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	record, err := h.store.GetSkillInstallationRecord(r.Context(), skill.UserID, skill.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if req.Name != "" || req.IsDefault != nil {
		record.Installation = applyCreateInstallationMetadata(record.Installation, req)
		record.Installation.UpdatedAt = time.Now().UTC()
		if err := h.store.UpdateSkillInstallation(r.Context(), record.Installation); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		record, err = h.store.GetSkillInstallationRecord(r.Context(), skill.UserID, skill.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	storedSkill, err := h.store.GetSkill(r.Context(), skill.UserID, skill.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.syncSkillMirror(r.Context(), storedSkill); err != nil {
		writeMirrorError(w, "skill was created locally but python mirror failed", err)
		return
	}
	writeJSON(w, http.StatusCreated, record)
}

func decodeCreateSkillRequest(r *http.Request) (createSkillRequest, error) {
	var req createSkillRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return createSkillRequest{}, errors.New("invalid request body")
	}
	return req, nil
}

func isExistingDefinitionInstallationRequest(req createSkillRequest) bool {
	return strings.TrimSpace(req.DefinitionID) != "" || strings.TrimSpace(req.CurrentRevisionID) != ""
}

func hasNewSkillCreateFields(req createSkillRequest) bool {
	return strings.TrimSpace(req.Slug) != "" ||
		strings.TrimSpace(req.Kind) != "" ||
		strings.TrimSpace(req.Title) != "" ||
		strings.TrimSpace(req.Description) != "" ||
		strings.TrimSpace(req.Prompt) != "" ||
		strings.TrimSpace(req.Mode) != "" ||
		strings.TrimSpace(req.Source) != "" ||
		strings.TrimSpace(req.RepoURL) != "" ||
		req.PlannerPolicy != nil ||
		len(req.ToolAllowlist) > 0
}

func presentCreateSkillRequestFields(req createSkillRequest) []string {
	fields := make([]string, 0, 13)
	if strings.TrimSpace(req.DefinitionID) != "" {
		fields = append(fields, "definitionId")
	}
	if strings.TrimSpace(req.CurrentRevisionID) != "" {
		fields = append(fields, "currentRevisionId")
	}
	if strings.TrimSpace(req.Name) != "" {
		fields = append(fields, "name")
	}
	if req.IsDefault != nil {
		fields = append(fields, "isDefault")
	}
	if strings.TrimSpace(req.Slug) != "" {
		fields = append(fields, "slug")
	}
	if strings.TrimSpace(req.Kind) != "" {
		fields = append(fields, "kind")
	}
	if strings.TrimSpace(req.Title) != "" {
		fields = append(fields, "title")
	}
	if strings.TrimSpace(req.Description) != "" {
		fields = append(fields, "description")
	}
	if strings.TrimSpace(req.Prompt) != "" {
		fields = append(fields, "prompt")
	}
	if strings.TrimSpace(req.Mode) != "" {
		fields = append(fields, "mode")
	}
	if strings.TrimSpace(req.Source) != "" {
		fields = append(fields, "source")
	}
	if req.Enabled != nil {
		fields = append(fields, "enabled")
	}
	if strings.TrimSpace(req.RepoURL) != "" {
		fields = append(fields, "repoUrl")
	}
	if req.PlannerPolicy != nil {
		fields = append(fields, "plannerPolicy")
	}
	if len(req.ToolAllowlist) > 0 {
		fields = append(fields, "toolAllowlist")
	}
	return fields
}

func applyCreateInstallationMetadata(installation domain.SkillInstallation, req createSkillRequest) domain.SkillInstallation {
	if strings.TrimSpace(req.Name) != "" {
		installation.Name = strings.TrimSpace(req.Name)
	}
	if req.IsDefault != nil {
		installation.IsDefault = installation.IsDefault || *req.IsDefault
	}
	return installation
}

func (h *Handler) createSkillModel(r *http.Request, req createSkillRequest) (domain.Skill, error) {
	user := currentUser(r.Context())
	if strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Prompt) == "" {
		return domain.Skill{}, errSkillTitlePromptRequired
	}
	now := time.Now().UTC()
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	return domain.Skill{
		ID:            uuid.NewString(),
		UserID:        user.ID,
		DefinitionID:  uuid.NewString(),
		RevisionID:    uuid.NewString(),
		Version:       1,
		Slug:          fallbackSlug(strings.TrimSpace(req.Slug), strings.TrimSpace(req.Title)),
		Kind:          normalizeSkillKind(req.Kind),
		Title:         strings.TrimSpace(req.Title),
		Description:   strings.TrimSpace(req.Description),
		Prompt:        strings.TrimSpace(req.Prompt),
		Mode:          string(chat.NormalizeSkill(req.Mode)),
		PlannerPolicy: normalizeStringMap(req.PlannerPolicy),
		ToolAllowlist: normalizeStringSlice(req.ToolAllowlist),
		Source:        normalizeSkillSource(req.Source),
		Enabled:       enabled,
		RepoURL:       strings.TrimSpace(req.RepoURL),
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

func (h *Handler) createSkillInstallationModel(r *http.Request, req createSkillRequest) (domain.SkillInstallation, error) {
	user := currentUser(r.Context())
	definitionID := strings.TrimSpace(req.DefinitionID)
	if definitionID == "" {
		return domain.SkillInstallation{}, errSkillDefinitionRequired
	}

	details, err := h.store.GetSkillDefinitionDetails(r.Context(), user.ID, definitionID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return domain.SkillInstallation{}, store.ErrNotFound
		}
		return domain.SkillInstallation{}, err
	}
	if len(details.Revisions) == 0 {
		return domain.SkillInstallation{}, store.ErrNotFound
	}

	revisionID := strings.TrimSpace(req.CurrentRevisionID)
	selectedRevision := details.Revisions[0]
	if revisionID == "" {
		revisionID = selectedRevision.ID
	} else {
		matched := false
		for _, revision := range details.Revisions {
			if revision.ID == revisionID {
				selectedRevision = revision
				matched = true
				break
			}
		}
		if !matched {
			return domain.SkillInstallation{}, errSkillRevisionNotFound
		}
	}

	now := time.Now().UTC()
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = selectedRevision.Title
	}
	isDefault := len(details.Installations) == 0
	if req.IsDefault != nil {
		isDefault = isDefault || *req.IsDefault
	}
	return domain.SkillInstallation{
		ID:                uuid.NewString(),
		UserID:            user.ID,
		DefinitionID:      definitionID,
		CurrentRevisionID: revisionID,
		Name:              name,
		IsDefault:         isDefault,
		Enabled:           enabled,
		CreatedAt:         now,
		UpdatedAt:         now,
	}, nil
}

func (h *Handler) listSkills(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	skills, err := h.store.ListSkills(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": skills})
}

func (h *Handler) listSkillInstallations(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	items, err := h.store.ListSkillInstallations(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) getSkillInstallation(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	item, err := h.store.GetSkillInstallationRecord(r.Context(), user.ID, chi.URLParam(r, "installationID"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeSkillInstallationNotFound(w)
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) listSkillDefinitions(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	items, err := h.store.ListSkillDefinitions(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) getSkillDefinition(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	item, err := h.store.GetSkillDefinitionDetails(r.Context(), user.ID, chi.URLParam(r, "definitionID"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeSkillDefinitionNotFound(w)
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) listSkillRevisions(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	items, err := h.store.ListSkillRevisions(r.Context(), user.ID, chi.URLParam(r, "definitionID"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeSkillDefinitionNotFound(w)
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) listSkillImportJobs(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	items, err := h.store.ListSkillImportJobs(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) getSkillImportJob(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	job, err := h.store.GetSkillImportJob(r.Context(), user.ID, chi.URLParam(r, "jobID"))
	if err != nil {
		writeNotFound(w, "skill import job not found", "skill_import_job")
		return
	}
	details := skillImportJobDetails{Job: job}
	if strings.TrimSpace(job.ArtifactID) != "" {
		artifact, err := h.store.GetSkillArtifact(r.Context(), user.ID, job.ArtifactID)
		if err == nil {
			details.Artifact = &artifact
		}
	}
	writeJSON(w, http.StatusOK, details)
}

func (h *Handler) listSkillArtifactFiles(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	items, err := h.store.ListSkillArtifactFiles(r.Context(), user.ID, chi.URLParam(r, "artifactID"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeNotFound(w, "skill artifact not found", "skill_artifact")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) getSkillArtifactFileContent(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	filePath := r.URL.Query().Get("path")
	if strings.TrimSpace(filePath) == "" {
		writeValidationError(w, "path is required", []string{"path"})
		return
	}

	item, content, err := skillartifact.LoadFile(r.Context(), h.store, user.ID, chi.URLParam(r, "artifactID"), filePath)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeNotFound(w, "skill artifact not found", "skill_artifact")
		case errors.Is(err, skillartifact.ErrArtifactFilePathRequired):
			writeValidationError(w, "path is required", []string{"path"})
		case errors.Is(err, skillartifact.ErrArtifactFileNotFound):
			writeNotFound(w, "skill artifact file not found", "skill_artifact_file")
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	mediaType := strings.TrimSpace(item.MediaType)
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", mediaType)
	w.Header().Set("X-Skill-Artifact-Path", item.Path)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func (h *Handler) importSkillFromGitHub(w http.ResponseWriter, r *http.Request) {
	if h.skillImporter == nil {
		writeServiceUnavailable(w, "skill importer is unavailable")
		return
	}
	var req importSkillGitHubRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeInvalidRequestBody(w)
		return
	}
	if strings.TrimSpace(req.RepoURL) == "" {
		writeValidationError(w, "repoUrl is required", []string{"repoUrl"})
		return
	}
	user := currentUser(r.Context())
	job := newSkillImportJob(user.ID, domain.SkillSourceGithub, req)
	if err := h.store.CreateSkillImportJob(r.Context(), job); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	parsed, err := h.skillImporter.ImportGitHubArchive(r.Context(), skillimport.GitHubImportRequest{
		RepoURL: req.RepoURL,
		Ref:     req.Ref,
		Path:    req.Path,
	})
	if err != nil {
		h.failSkillImportJob(r.Context(), &job, err)
		writeSkillImportError(w, err)
		return
	}
	result, err := h.materializeImportedSkill(r.Context(), user.ID, job, parsed, req.Install, req.InstallationName, req.IsDefault, req.Enabled)
	if err != nil {
		writeSkillImportError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) importSkillFromUpload(w http.ResponseWriter, r *http.Request) {
	if h.skillImporter == nil {
		writeServiceUnavailable(w, "skill importer is unavailable")
		return
	}
	user := currentUser(r.Context())
	if err := r.ParseMultipartForm(12 << 20); err != nil {
		writeValidationError(w, "invalid upload form", []string{"file"})
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeValidationError(w, "file is required", []string{"file"})
		return
	}
	defer file.Close()

	raw, err := io.ReadAll(io.LimitReader(file, 10<<20+1))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(raw) == 0 {
		writeValidationError(w, "file is empty", []string{"file"})
		return
	}

	install, err := parseBoolWithDefault(r.FormValue("install"), true)
	if err != nil {
		writeValidationError(w, "install must be true or false", []string{"install"})
		return
	}
	isDefault, err := parseOptionalBool(r.FormValue("isDefault"))
	if err != nil {
		writeValidationError(w, "isDefault must be true or false", []string{"isDefault"})
		return
	}
	enabled, err := parseOptionalBool(r.FormValue("enabled"))
	if err != nil {
		writeValidationError(w, "enabled must be true or false", []string{"enabled"})
		return
	}

	requestPayload := map[string]any{
		"fileName":         header.Filename,
		"path":             r.FormValue("path"),
		"install":          install,
		"installationName": r.FormValue("installationName"),
	}
	if isDefault != nil {
		requestPayload["isDefault"] = *isDefault
	}
	if enabled != nil {
		requestPayload["enabled"] = *enabled
	}

	job := newSkillImportJob(user.ID, domain.SkillSourceUpload, requestPayload)
	if err := h.store.CreateSkillImportJob(r.Context(), job); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	parsed, err := h.skillImporter.ImportUploadedArchive(header.Filename, header.Header.Get("Content-Type"), r.FormValue("path"), raw)
	if err != nil {
		h.failSkillImportJob(r.Context(), &job, err)
		writeSkillImportError(w, err)
		return
	}
	result, err := h.materializeImportedSkill(r.Context(), user.ID, job, parsed, &install, r.FormValue("installationName"), isDefault, enabled)
	if err != nil {
		writeSkillImportError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) updateSkill(w http.ResponseWriter, r *http.Request) {
	req, err := decodeUpdateSkillRequest(r)
	if err != nil {
		writeSkillInvalidRequestBody(w)
		return
	}
	if req.CurrentRevisionID != nil {
		writeSkillInstallationOnlyFields(w, "currentRevisionId is only supported for skill installations", []string{"currentRevisionId"})
		return
	}
	user := currentUser(r.Context())
	skillID := chi.URLParam(r, "skillID")
	if hasSkillRevisionPatch(req) {
		skill, err := h.updateSkillModel(r.Context(), user.ID, skillID, req)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeSkillInstallationNotFound(w)
				return
			}
			if errors.Is(err, errSkillTitlePromptRequired) {
				writeSkillTitlePromptRequired(w)
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if err := h.store.UpdateSkill(r.Context(), skill); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeSkillInstallationNotFound(w)
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if err := h.syncSkillMirror(r.Context(), skill); err != nil {
			writeMirrorError(w, "skill was updated locally but python mirror failed", err)
			return
		}
		writeJSON(w, http.StatusOK, skill)
		return
	}
	if !hasSkillInstallationPatch(req) {
		writeSkillNoUpdatableFields(w, supportedSkillUpdateFields())
		return
	}
	if _, err := h.updateSkillInstallationRecordModel(r.Context(), user.ID, skillID, req); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeSkillInstallationNotFound(w)
			return
		}
		if errors.Is(err, errSkillRevisionNotFound) {
			writeSkillRevisionNotFound(w, http.StatusBadRequest)
			return
		}
		if errors.Is(err, errSkillNoUpdatableFields) {
			writeSkillNoUpdatableFields(w, supportedSkillUpdateFields())
			return
		}
		if errors.Is(err, errSkillRevisionRequired) {
			writeSkillRevisionRequired(w)
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	skill, err := h.store.GetSkill(r.Context(), user.ID, skillID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeSkillInstallationNotFound(w)
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.syncSkillMirror(r.Context(), skill); err != nil {
		writeMirrorError(w, "skill was updated locally but python mirror failed", err)
		return
	}
	writeJSON(w, http.StatusOK, skill)
}

func (h *Handler) updateSkillInstallation(w http.ResponseWriter, r *http.Request) {
	req, err := decodeUpdateSkillRequest(r)
	if err != nil {
		writeSkillInvalidRequestBody(w)
		return
	}
	user := currentUser(r.Context())
	installationID := chi.URLParam(r, "installationID")
	if req.CurrentRevisionID != nil {
		if hasSkillRevisionPatch(req) {
			writeSkillInvalidFieldCombination(w, "currentRevisionId cannot be combined with skill definition or revision fields", presentUpdateSkillRequestFields(req))
			return
		}
		record, err := h.updateSkillInstallationRecordModel(r.Context(), user.ID, installationID, req)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeSkillInstallationNotFound(w)
				return
			}
			if errors.Is(err, errSkillRevisionNotFound) {
				writeSkillRevisionNotFound(w, http.StatusBadRequest)
				return
			}
			if errors.Is(err, errSkillRevisionRequired) {
				writeSkillRevisionRequired(w)
				return
			}
			if errors.Is(err, errSkillNoUpdatableFields) {
				writeSkillNoUpdatableFields(w, supportedSkillInstallationUpdateFields())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		storedSkill, err := h.store.GetSkill(r.Context(), user.ID, installationID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeSkillInstallationNotFound(w)
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if err := h.syncSkillMirror(r.Context(), storedSkill); err != nil {
			writeMirrorError(w, "skill was updated locally but python mirror failed", err)
			return
		}
		writeJSON(w, http.StatusOK, record)
		return
	}
	if hasSkillRevisionPatch(req) {
		skill, err := h.updateSkillModel(r.Context(), user.ID, installationID, req)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeSkillInstallationNotFound(w)
				return
			}
			if errors.Is(err, errSkillTitlePromptRequired) {
				writeSkillTitlePromptRequired(w)
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if err := h.store.UpdateSkill(r.Context(), skill); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeSkillInstallationNotFound(w)
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		record, err := h.store.GetSkillInstallationRecord(r.Context(), user.ID, skill.ID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeSkillInstallationNotFound(w)
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if err := h.syncSkillMirror(r.Context(), skill); err != nil {
			writeMirrorError(w, "skill was updated locally but python mirror failed", err)
			return
		}
		writeJSON(w, http.StatusOK, record)
		return
	}
	if !hasSkillInstallationPatch(req) {
		writeSkillNoUpdatableFields(w, supportedSkillInstallationUpdateFields())
		return
	}
	record, err := h.updateSkillInstallationRecordModel(r.Context(), user.ID, installationID, req)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeSkillInstallationNotFound(w)
			return
		}
		if errors.Is(err, errSkillRevisionNotFound) {
			writeSkillRevisionNotFound(w, http.StatusBadRequest)
			return
		}
		if errors.Is(err, errSkillRevisionRequired) {
			writeSkillRevisionRequired(w)
			return
		}
		if errors.Is(err, errSkillNoUpdatableFields) {
			writeSkillNoUpdatableFields(w, supportedSkillInstallationUpdateFields())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	storedSkill, err := h.store.GetSkill(r.Context(), user.ID, installationID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeSkillInstallationNotFound(w)
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.syncSkillMirror(r.Context(), storedSkill); err != nil {
		writeMirrorError(w, "skill was updated locally but python mirror failed", err)
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func decodeUpdateSkillRequest(r *http.Request) (updateSkillRequest, error) {
	var req updateSkillRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return updateSkillRequest{}, errors.New("invalid request body")
	}
	return req, nil
}

func hasSkillRevisionPatch(req updateSkillRequest) bool {
	return req.Slug != nil ||
		req.Kind != nil ||
		req.Title != nil ||
		req.Description != nil ||
		req.Prompt != nil ||
		req.Mode != nil ||
		req.Source != nil ||
		req.RepoURL != nil ||
		req.PlannerPolicy != nil ||
		req.ToolAllowlist != nil
}

func hasSkillInstallationPatch(req updateSkillRequest) bool {
	return req.Enabled != nil || req.Name != nil || req.IsDefault != nil
}

func presentUpdateSkillRequestFields(req updateSkillRequest) []string {
	fields := make([]string, 0, 12)
	if req.CurrentRevisionID != nil {
		fields = append(fields, "currentRevisionId")
	}
	if req.Name != nil {
		fields = append(fields, "name")
	}
	if req.IsDefault != nil {
		fields = append(fields, "isDefault")
	}
	if req.Slug != nil {
		fields = append(fields, "slug")
	}
	if req.Kind != nil {
		fields = append(fields, "kind")
	}
	if req.Title != nil {
		fields = append(fields, "title")
	}
	if req.Description != nil {
		fields = append(fields, "description")
	}
	if req.Prompt != nil {
		fields = append(fields, "prompt")
	}
	if req.Mode != nil {
		fields = append(fields, "mode")
	}
	if req.Source != nil {
		fields = append(fields, "source")
	}
	if req.Enabled != nil {
		fields = append(fields, "enabled")
	}
	if req.RepoURL != nil {
		fields = append(fields, "repoUrl")
	}
	if req.PlannerPolicy != nil {
		fields = append(fields, "plannerPolicy")
	}
	if req.ToolAllowlist != nil {
		fields = append(fields, "toolAllowlist")
	}
	return fields
}

func supportedSkillUpdateFields() []string {
	return []string{
		"slug",
		"kind",
		"title",
		"description",
		"prompt",
		"mode",
		"source",
		"enabled",
		"repoUrl",
		"plannerPolicy",
		"toolAllowlist",
		"name",
		"isDefault",
	}
}

func supportedSkillInstallationUpdateFields() []string {
	return []string{
		"currentRevisionId",
		"slug",
		"kind",
		"title",
		"description",
		"prompt",
		"mode",
		"source",
		"enabled",
		"repoUrl",
		"plannerPolicy",
		"toolAllowlist",
		"name",
		"isDefault",
	}
}

func (h *Handler) updateSkillModel(ctx context.Context, userID, installationID string, req updateSkillRequest) (domain.Skill, error) {
	skill, err := h.store.GetSkill(ctx, userID, installationID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return domain.Skill{}, store.ErrNotFound
		}
		return domain.Skill{}, err
	}
	if req.Slug != nil {
		skill.Slug = fallbackSlug(strings.TrimSpace(*req.Slug), skill.Title)
	}
	if req.Kind != nil {
		skill.Kind = normalizeSkillKind(*req.Kind)
	}
	if req.Title != nil {
		skill.Title = strings.TrimSpace(*req.Title)
	}
	if req.Description != nil {
		skill.Description = strings.TrimSpace(*req.Description)
	}
	if req.Prompt != nil {
		skill.Prompt = strings.TrimSpace(*req.Prompt)
	}
	if req.Mode != nil {
		skill.Mode = string(chat.NormalizeSkill(*req.Mode))
	}
	if req.Source != nil {
		skill.Source = normalizeSkillSource(*req.Source)
	}
	if req.Enabled != nil {
		skill.Enabled = *req.Enabled
	}
	if req.RepoURL != nil {
		skill.RepoURL = strings.TrimSpace(*req.RepoURL)
	}
	if req.PlannerPolicy != nil {
		skill.PlannerPolicy = normalizeStringMap(*req.PlannerPolicy)
	}
	if req.ToolAllowlist != nil {
		skill.ToolAllowlist = normalizeStringSlice(*req.ToolAllowlist)
	}
	if !hasSkillRevisionPatch(req) && !hasSkillInstallationPatch(req) {
		return domain.Skill{}, errSkillNoUpdatableFields
	}
	skill.Version++
	skill.RevisionID = uuid.NewString()
	skill.UpdatedAt = time.Now().UTC()

	if strings.TrimSpace(skill.Title) == "" || strings.TrimSpace(skill.Prompt) == "" {
		return domain.Skill{}, errSkillTitlePromptRequired
	}
	skill.Slug = fallbackSlug(skill.Slug, skill.Title)
	return skill, nil
}

func (h *Handler) updateSkillInstallationRecordModel(ctx context.Context, userID, installationID string, req updateSkillRequest) (domain.SkillInstallationRecord, error) {
	record, err := h.store.GetSkillInstallationRecord(ctx, userID, installationID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return domain.SkillInstallationRecord{}, store.ErrNotFound
		}
		return domain.SkillInstallationRecord{}, err
	}
	if req.CurrentRevisionID == nil && !hasSkillInstallationPatch(req) {
		return domain.SkillInstallationRecord{}, errSkillNoUpdatableFields
	}

	installation := record.Installation
	if req.CurrentRevisionID != nil {
		revisionID := strings.TrimSpace(*req.CurrentRevisionID)
		if revisionID == "" {
			return domain.SkillInstallationRecord{}, errSkillRevisionRequired
		}
		installation.CurrentRevisionID = revisionID
	}
	if req.Enabled != nil {
		installation.Enabled = *req.Enabled
	}
	if req.Name != nil {
		installation.Name = strings.TrimSpace(*req.Name)
	}
	if req.IsDefault != nil {
		installation.IsDefault = *req.IsDefault
	}
	installation.UpdatedAt = time.Now().UTC()

	if err := h.store.UpdateSkillInstallation(ctx, installation); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			if req.CurrentRevisionID != nil {
				return domain.SkillInstallationRecord{}, errSkillRevisionNotFound
			}
			return domain.SkillInstallationRecord{}, store.ErrNotFound
		}
		return domain.SkillInstallationRecord{}, err
	}
	return h.store.GetSkillInstallationRecord(ctx, userID, installationID)
}

func (h *Handler) deleteSkill(w http.ResponseWriter, r *http.Request) {
	if err := h.deleteSkillByID(r.Context(), currentUser(r.Context()).ID, chi.URLParam(r, "skillID")); err != nil {
		writeSkillInstallationNotFound(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) deleteSkillInstallation(w http.ResponseWriter, r *http.Request) {
	if err := h.deleteSkillByID(r.Context(), currentUser(r.Context()).ID, chi.URLParam(r, "installationID")); err != nil {
		writeSkillInstallationNotFound(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) deleteSkillByID(ctx context.Context, userID, installationID string) error {
	if err := h.store.DeleteSkill(ctx, userID, installationID); err != nil {
		return err
	}
	return h.deleteSkillMirror(ctx, userID, installationID)
}

func normalizeSkillSource(raw string) domain.SkillSource {
	switch domain.SkillSource(strings.ToLower(strings.TrimSpace(raw))) {
	case domain.SkillSourceGithub:
		return domain.SkillSourceGithub
	case domain.SkillSourceUpload:
		return domain.SkillSourceUpload
	default:
		return domain.SkillSourceManual
	}
}

func (h *Handler) syncSkillMirror(ctx context.Context, skill domain.Skill) error {
	if h.mirrorService == nil {
		return nil
	}
	return h.mirrorService.SyncSkill(ctx, skill)
}

func (h *Handler) deleteSkillMirror(ctx context.Context, userID, installationID string) error {
	if h.mirrorService == nil {
		return nil
	}
	return h.mirrorService.DeleteSkill(ctx, userID, installationID)
}

func (h *Handler) resolveChatSkillSelection(ctx context.Context, userID, installationID, definitionID string) (string, domain.Skill, error) {
	record, err := skillresolver.ResolveInstallationRecord(ctx, h.store, userID, installationID, definitionID)
	if err != nil {
		return "", domain.Skill{}, err
	}
	storedSkill, err := h.store.GetSkill(ctx, userID, record.Installation.ID)
	if err != nil {
		return "", domain.Skill{}, err
	}
	return record.Installation.ID, storedSkill, nil
}

func normalizeSkillKind(raw string) domain.SkillKind {
	switch domain.SkillKind(strings.ToLower(strings.TrimSpace(raw))) {
	case domain.SkillKindAgentWorkflow:
		return domain.SkillKindAgentWorkflow
	default:
		return domain.SkillKindChatProfile
	}
}

func normalizeStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeStringSlice(input []string) []string {
	if len(input) == 0 {
		return nil
	}
	out := make([]string, 0, len(input))
	seen := map[string]struct{}{}
	for _, item := range input {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func fallbackSlug(raw, title string) string {
	if strings.TrimSpace(raw) != "" {
		return strings.TrimSpace(raw)
	}
	candidate := strings.ToLower(strings.TrimSpace(title))
	var builder strings.Builder
	lastDash := false
	for _, r := range candidate {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || (r >= 0x4e00 && r <= 0x9fa5) {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteRune('-')
			lastDash = true
		}
	}
	slug := strings.Trim(builder.String(), "-")
	if slug == "" {
		return uuid.NewString()
	}
	return slug
}

func newSkillImportJob(userID string, source domain.SkillSource, request any) domain.SkillImportJob {
	now := time.Now().UTC()
	requestJSON := "{}"
	if request != nil {
		if raw, err := json.Marshal(request); err == nil {
			requestJSON = string(raw)
		}
	}
	return domain.SkillImportJob{
		ID:          uuid.NewString(),
		UserID:      userID,
		Source:      source,
		Status:      domain.SkillImportPending,
		RequestJSON: requestJSON,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func (h *Handler) failSkillImportJob(ctx context.Context, job *domain.SkillImportJob, cause error) {
	if job == nil {
		return
	}
	now := time.Now().UTC()
	job.Status = domain.SkillImportFailed
	job.ErrorMessage = cause.Error()
	job.UpdatedAt = now
	job.CompletedAt = &now
	_ = h.store.UpdateSkillImportJob(ctx, *job)
}

func (h *Handler) materializeImportedSkill(
	ctx context.Context,
	userID string,
	job domain.SkillImportJob,
	parsed skillimport.ParsedPackage,
	install *bool,
	installationName string,
	isDefault *bool,
	enabled *bool,
) (skillImportJobDetails, error) {
	now := time.Now().UTC()
	definitionID := uuid.NewString()
	revisionID := uuid.NewString()

	skillModel := domain.Skill{
		ID:            "",
		UserID:        userID,
		DefinitionID:  definitionID,
		RevisionID:    revisionID,
		Version:       1,
		Slug:          parsed.Slug,
		Kind:          parsed.Kind,
		Title:         parsed.Title,
		Description:   parsed.Description,
		Prompt:        parsed.Prompt,
		Mode:          parsed.Mode,
		PlannerPolicy: clonePlannerPolicy(parsed.PlannerPolicy),
		ToolAllowlist: append([]string(nil), parsed.ToolAllowlist...),
		Source:        parsed.Source,
		RepoURL:       parsed.RepoURL,
		Enabled:       enabled == nil || *enabled,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	manifestJSON, err := skillruntime.EncodeRevisionManifest(skillModel)
	if err != nil {
		h.failSkillImportJob(ctx, &job, err)
		return skillImportJobDetails{}, err
	}

	definition := domain.SkillDefinition{
		ID:        definitionID,
		UserID:    userID,
		Slug:      parsed.Slug,
		Kind:      parsed.Kind,
		Source:    parsed.Source,
		RepoURL:   parsed.RepoURL,
		CreatedAt: now,
		UpdatedAt: now,
	}
	revision := domain.SkillRevision{
		ID:            revisionID,
		DefinitionID:  definitionID,
		Version:       1,
		Title:         parsed.Title,
		Description:   parsed.Description,
		Prompt:        parsed.Prompt,
		Mode:          parsed.Mode,
		PlannerPolicy: clonePlannerPolicy(parsed.PlannerPolicy),
		ToolAllowlist: append([]string(nil), parsed.ToolAllowlist...),
		ManifestJSON:  manifestJSON,
		CreatedAt:     now,
	}
	if err := h.store.CreateSkillDefinitionRevision(ctx, definition, revision); err != nil {
		h.failSkillImportJob(ctx, &job, err)
		return skillImportJobDetails{}, err
	}

	artifact := domain.SkillArtifact{
		ID:               uuid.NewString(),
		UserID:           userID,
		DefinitionID:     definitionID,
		RevisionID:       revisionID,
		Source:           parsed.Source,
		FileName:         parsed.FileName,
		MediaType:        parsed.MediaType,
		SourceURL:        parsed.SourceURL,
		SHA256:           parsed.SHA256,
		SizeBytes:        parsed.SizeBytes,
		EntryPath:        parsed.EntryPath,
		ManifestPath:     parsed.ManifestPath,
		InstructionsPath: parsed.InstructionsPath,
		ArchiveBytes:     append([]byte(nil), parsed.ArchiveBytes...),
		CreatedAt:        now,
	}
	if err := h.store.CreateSkillArtifact(ctx, artifact); err != nil {
		h.failSkillImportJob(ctx, &job, err)
		return skillImportJobDetails{}, err
	}
	files := make([]domain.SkillArtifactFile, 0, len(parsed.Files))
	for _, item := range parsed.Files {
		files = append(files, domain.SkillArtifactFile{
			ID:             uuid.NewString(),
			ArtifactID:     artifact.ID,
			UserID:         userID,
			Path:           item.Path,
			MediaType:      item.MediaType,
			SizeBytes:      item.SizeBytes,
			SHA256:         item.SHA256,
			IsManifest:     item.IsManifest,
			IsInstructions: item.IsInstructions,
			CreatedAt:      now,
		})
	}
	if err := h.store.ReplaceSkillArtifactFiles(ctx, artifact.ID, files); err != nil {
		h.failSkillImportJob(ctx, &job, err)
		return skillImportJobDetails{}, err
	}

	job.ArtifactID = artifact.ID
	job.DefinitionID = definitionID
	job.RevisionID = revisionID

	shouldInstall := true
	if install != nil {
		shouldInstall = *install
	}
	if shouldInstall {
		installation := domain.SkillInstallation{
			ID:                uuid.NewString(),
			UserID:            userID,
			DefinitionID:      definitionID,
			CurrentRevisionID: revisionID,
			Name:              strings.TrimSpace(installationName),
			IsDefault:         isDefault != nil && *isDefault,
			Enabled:           enabled == nil || *enabled,
			CreatedAt:         now,
			UpdatedAt:         now,
		}
		if err := h.store.CreateSkillInstallation(ctx, installation); err != nil {
			h.failSkillImportJob(ctx, &job, err)
			return skillImportJobDetails{}, err
		}
		job.InstallationID = installation.ID
		storedSkill, err := h.store.GetSkill(ctx, userID, installation.ID)
		if err != nil {
			h.failSkillImportJob(ctx, &job, err)
			return skillImportJobDetails{}, err
		}
		if err := h.syncSkillMirror(ctx, storedSkill); err != nil {
			h.failSkillImportJob(ctx, &job, err)
			return skillImportJobDetails{}, err
		}
	}

	completedAt := time.Now().UTC()
	job.Status = domain.SkillImportCompleted
	job.ErrorMessage = ""
	job.UpdatedAt = completedAt
	job.CompletedAt = &completedAt
	if err := h.store.UpdateSkillImportJob(ctx, job); err != nil {
		return skillImportJobDetails{}, err
	}
	return skillImportJobDetails{
		Job:      job,
		Artifact: &artifact,
	}, nil
}

func writeSkillImportError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrConflict):
		writeSkillConflict(w)
	case errors.Is(err, skillimport.ErrUnsupportedRepoURL):
		writeValidationError(w, err.Error(), []string{"repoUrl"})
	case errors.Is(err, skillimport.ErrArchiveTooLarge):
		writeValidationError(w, err.Error(), []string{"file"})
	case errors.Is(err, skillimport.ErrInvalidArchive):
		writeValidationError(w, err.Error(), []string{"file"})
	case errors.Is(err, skillimport.ErrSkillPackageNotFound):
		writeValidationError(w, err.Error(), []string{"file", "path"})
	case errors.Is(err, skillimport.ErrInvalidManifest):
		writeValidationError(w, err.Error(), []string{"file"})
	case errors.Is(err, skillimport.ErrIncompleteSkillPrompt):
		writeValidationError(w, err.Error(), []string{"file"})
	case strings.Contains(err.Error(), "github archive download failed"):
		writeBadGateway(w, err.Error(), errorDetailResource("github_archive"))
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func parseBoolWithDefault(raw string, fallback bool) (bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, nil
	}
	return strconv.ParseBool(raw)
}

func parseOptionalBool(raw string) (*bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func clonePlannerPolicy(source map[string]string) map[string]string {
	if len(source) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}
