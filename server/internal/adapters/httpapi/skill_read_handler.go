package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	skillsvc "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skill"
)

func (h *Handler) listSkills(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	skills, err := h.skillService.ListSkills(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, skillListDTO{Items: mapSkills(skills)})
}

func (h *Handler) listSkillInstallations(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	items, err := h.skillService.ListInstallations(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, skillInstallationListDTO{Items: mapSkillInstallationRecords(items)})
}

func (h *Handler) getSkillInstallation(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	item, err := h.skillService.GetInstallation(r.Context(), user.ID, chi.URLParam(r, "installationID"))
	if err != nil {
		if errors.Is(err, skillsvc.ErrInstallationNotFound) {
			writeSkillInstallationNotFound(w)
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, mapSkillInstallationRecord(item))
}

func (h *Handler) listSkillDefinitions(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	items, err := h.skillService.ListDefinitions(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, skillDefinitionListDTO{Items: mapSkillDefinitions(items)})
}

func (h *Handler) getSkillDefinition(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	item, err := h.skillService.GetDefinition(r.Context(), user.ID, chi.URLParam(r, "definitionID"))
	if err != nil {
		if errors.Is(err, skillsvc.ErrDefinitionNotFound) {
			writeSkillDefinitionNotFound(w)
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, mapSkillDefinitionDetails(item))
}

func (h *Handler) listSkillRevisions(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	items, err := h.skillService.ListRevisions(r.Context(), user.ID, chi.URLParam(r, "definitionID"))
	if err != nil {
		if errors.Is(err, skillsvc.ErrDefinitionNotFound) {
			writeSkillDefinitionNotFound(w)
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, skillRevisionListDTO{Items: mapSkillRevisions(items)})
}

func (h *Handler) listSkillImportJobs(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	items, err := h.skillService.ListImportJobs(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, skillImportJobListDTO{Items: mapSkillImportJobs(items)})
}

func (h *Handler) getSkillImportJob(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	job, artifact, err := h.skillService.GetImportJobDetails(r.Context(), user.ID, chi.URLParam(r, "jobID"))
	if err != nil {
		if errors.Is(err, skillsvc.ErrImportJobNotFound) {
			writeNotFound(w, "skill import job not found", "skill_import_job")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	details := skillImportJobDetailsDTO{Job: mapSkillImportJob(job)}
	if artifact != nil {
		mapped := mapSkillArtifact(*artifact)
		details.Artifact = &mapped
	}
	writeJSON(w, http.StatusOK, details)
}

func (h *Handler) listSkillArtifactFiles(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	items, err := h.skillService.ListArtifactFiles(r.Context(), user.ID, chi.URLParam(r, "artifactID"))
	if err != nil {
		if errors.Is(err, skillsvc.ErrArtifactNotFound) {
			writeNotFound(w, "skill artifact not found", "skill_artifact")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, skillArtifactFileListDTO{Items: mapSkillArtifactFiles(items)})
}

func (h *Handler) getSkillArtifactFileContent(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	filePath := r.URL.Query().Get("path")
	if strings.TrimSpace(filePath) == "" {
		writeValidationError(w, "path is required", []string{"path"})
		return
	}

	item, content, err := h.skillService.LoadArtifactFile(r.Context(), user.ID, chi.URLParam(r, "artifactID"), filePath)
	if err != nil {
		switch {
		case errors.Is(err, skillsvc.ErrArtifactNotFound):
			writeNotFound(w, "skill artifact not found", "skill_artifact")
		case errors.Is(err, skillsvc.ErrArtifactFilePathRequired):
			writeValidationError(w, "path is required", []string{"path"})
		case errors.Is(err, skillsvc.ErrArtifactFileNotFound):
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
