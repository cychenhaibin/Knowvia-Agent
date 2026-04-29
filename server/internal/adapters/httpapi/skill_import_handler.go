package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	skillsvc "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skill"
)

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
	result, err := h.skillImporter.ImportFromGitHub(r.Context(), user.ID, skillsvc.ImportGitHubInput{
		RepoURL:          req.RepoURL,
		Ref:              req.Ref,
		Path:             req.Path,
		Install:          req.Install,
		InstallationName: req.InstallationName,
		IsDefault:        req.IsDefault,
		Enabled:          req.Enabled,
	})
	if err != nil {
		writeSkillImportError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, mapSkillImportResult(result))
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

	result, err := h.skillImporter.ImportFromUpload(r.Context(), user.ID, skillsvc.ImportUploadInput{
		FileName:         header.Filename,
		MediaType:        header.Header.Get("Content-Type"),
		Path:             r.FormValue("path"),
		ArchiveBytes:     raw,
		Install:          install,
		InstallationName: r.FormValue("installationName"),
		IsDefault:        isDefault,
		Enabled:          enabled,
	})
	if err != nil {
		writeSkillImportError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, mapSkillImportResult(result))
}

func writeSkillImportError(w http.ResponseWriter, err error) {
	var githubArchiveErr *skillsvc.GitHubArchiveDownloadError
	switch {
	case errors.Is(err, skillsvc.ErrSkillConflict):
		writeSkillConflict(w)
	case errors.Is(err, skillsvc.ErrUnsupportedRepoURL):
		writeValidationError(w, err.Error(), []string{"repoUrl"})
	case errors.Is(err, skillsvc.ErrArchiveTooLarge):
		writeValidationError(w, err.Error(), []string{"file"})
	case errors.Is(err, skillsvc.ErrInvalidArchive):
		writeValidationError(w, err.Error(), []string{"file"})
	case errors.Is(err, skillsvc.ErrSkillPackageNotFound):
		writeValidationError(w, err.Error(), []string{"file", "path"})
	case errors.Is(err, skillsvc.ErrInvalidManifest):
		writeValidationError(w, err.Error(), []string{"file"})
	case errors.Is(err, skillsvc.ErrIncompleteSkillPrompt):
		writeValidationError(w, err.Error(), []string{"file"})
	case errors.As(err, &githubArchiveErr):
		writeBadGateway(w, err.Error(), errorDetailResource("github_archive"))
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}
