package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	skillsvc "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skill"
)

func writeCreateSkillError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return true
	}
	var mirrorErr *skillsvc.MirrorSyncError
	switch {
	case writeSkillValidationError(w, err):
	case errors.Is(err, skillsvc.ErrSkillConflict):
		writeSkillConflict(w)
	case errors.Is(err, skillsvc.ErrTitlePromptRequired):
		writeSkillTitlePromptRequired(w)
	case errors.As(err, &mirrorErr):
		writeMirrorError(w, "skill was created locally but python mirror failed", mirrorErr.Err)
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
	return false
}

func writeCreateInstallationError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return true
	}
	var mirrorErr *skillsvc.MirrorSyncError
	switch {
	case writeSkillValidationError(w, err):
	case errors.Is(err, skillsvc.ErrDefinitionNotFound):
		writeSkillDefinitionNotFound(w)
	case errors.Is(err, skillsvc.ErrDefinitionRequired):
		writeSkillDefinitionRequired(w)
	case errors.Is(err, skillsvc.ErrRevisionNotFound):
		writeSkillRevisionNotFound(w, http.StatusBadRequest)
	case errors.Is(err, skillsvc.ErrInstallationConflict):
		writeSkillInstallationConflict(w)
	case errors.Is(err, skillsvc.ErrSkillConflict):
		writeSkillConflict(w)
	case errors.Is(err, skillsvc.ErrTitlePromptRequired):
		writeSkillTitlePromptRequired(w)
	case errors.As(err, &mirrorErr):
		writeMirrorError(w, "skill was created locally but python mirror failed", mirrorErr.Err)
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
	return false
}

func writeUpdateSkillError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return true
	}
	var mirrorErr *skillsvc.MirrorSyncError
	switch {
	case writeSkillValidationError(w, err):
	case errors.Is(err, skillsvc.ErrInstallationNotFound):
		writeSkillInstallationNotFound(w)
	case errors.Is(err, skillsvc.ErrRevisionNotFound):
		writeSkillRevisionNotFound(w, http.StatusBadRequest)
	case errors.Is(err, skillsvc.ErrRevisionRequired):
		writeSkillRevisionRequired(w)
	case errors.Is(err, skillsvc.ErrTitlePromptRequired):
		writeSkillTitlePromptRequired(w)
	case errors.As(err, &mirrorErr):
		writeMirrorError(w, "skill was updated locally but python mirror failed", mirrorErr.Err)
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
	return false
}

func writeUpdateInstallationError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return true
	}
	var mirrorErr *skillsvc.MirrorSyncError
	switch {
	case writeSkillValidationError(w, err):
	case errors.Is(err, skillsvc.ErrInstallationNotFound):
		writeSkillInstallationNotFound(w)
	case errors.Is(err, skillsvc.ErrRevisionNotFound):
		writeSkillRevisionNotFound(w, http.StatusBadRequest)
	case errors.Is(err, skillsvc.ErrRevisionRequired):
		writeSkillRevisionRequired(w)
	case errors.Is(err, skillsvc.ErrTitlePromptRequired):
		writeSkillTitlePromptRequired(w)
	case errors.As(err, &mirrorErr):
		writeMirrorError(w, "skill was updated locally but python mirror failed", mirrorErr.Err)
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
	return false
}

func (h *Handler) createSkill(w http.ResponseWriter, r *http.Request) {
	req, err := decodeCreateSkillRequest(r)
	if err != nil {
		writeSkillInvalidRequestBody(w)
		return
	}
	user := currentUser(r.Context())
	skill, err := h.skillService.CreateStandaloneSkill(r.Context(), user.ID, createSkillInputFromRequest(req))
	if !writeCreateSkillError(w, err) {
		return
	}
	writeJSON(w, http.StatusCreated, mapSkill(skill))
}

func (h *Handler) createSkillInstallation(w http.ResponseWriter, r *http.Request) {
	req, err := decodeCreateSkillRequest(r)
	if err != nil {
		writeSkillInvalidRequestBody(w)
		return
	}
	user := currentUser(r.Context())
	record, err := h.skillService.CreateInstallation(r.Context(), user.ID, createSkillInputFromRequest(req))
	if !writeCreateInstallationError(w, err) {
		return
	}
	writeJSON(w, http.StatusCreated, mapSkillInstallationRecord(record))
}

func (h *Handler) updateSkill(w http.ResponseWriter, r *http.Request) {
	req, err := decodeUpdateSkillRequest(r)
	if err != nil {
		writeSkillInvalidRequestBody(w)
		return
	}
	user := currentUser(r.Context())
	skill, err := h.skillService.UpdateStandaloneSkill(r.Context(), user.ID, chi.URLParam(r, "skillID"), updateSkillInputFromRequest(req))
	if !writeUpdateSkillError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, mapSkill(skill))
}

func (h *Handler) updateSkillInstallation(w http.ResponseWriter, r *http.Request) {
	req, err := decodeUpdateSkillRequest(r)
	if err != nil {
		writeSkillInvalidRequestBody(w)
		return
	}
	user := currentUser(r.Context())
	record, err := h.skillService.UpdateInstallationFromInput(r.Context(), user.ID, chi.URLParam(r, "installationID"), updateSkillInputFromRequest(req))
	if !writeUpdateInstallationError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, mapSkillInstallationRecord(record))
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
	return h.skillService.DeleteSkill(ctx, userID, installationID)
}
