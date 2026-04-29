package httpapi

import (
	"errors"
	"net/http"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/chat"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/run"
	skillsvc "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skill"
)

var (
	errSkillRevisionNotFound    = errors.New("skill revision not found")
	errSkillNoUpdatableFields   = errors.New("no updatable fields provided")
	errSkillRevisionRequired    = errors.New("currentRevisionId is required")
	errSkillDefinitionRequired  = errors.New("definitionId is required")
	errSkillTitlePromptRequired = errors.New("title and prompt are required")
)

const (
	errorCodeSkillDefinitionNotFound      = "skill_definition_not_found"
	errorCodeSkillDefinitionRequired      = "skill_definition_required"
	errorCodeSkillInvalidRequestBody      = "skill_invalid_request_body"
	errorCodeSkillConflict                = "skill_conflict"
	errorCodeSkillInstallationNotFound    = "skill_installation_not_found"
	errorCodeSkillRevisionNotFound        = "skill_revision_not_found"
	errorCodeSkillInstallationDisabled    = "skill_installation_disabled"
	errorCodeNoEnabledSkillInstallation   = "skill_definition_no_enabled_installation"
	errorCodeSkillDefinitionMismatch      = "skill_definition_mismatch"
	errorCodeSkillInstallationConflict    = "skill_installation_conflict"
	errorCodeSkillInstallationOnlyFields  = "skill_installation_only_fields"
	errorCodeSkillInvalidFieldCombination = "skill_invalid_field_combination"
	errorCodeSkillNoUpdatableFields       = "skill_no_updatable_fields"
	errorCodeSkillRevisionRequired        = "skill_revision_required"
	errorCodeSkillTitlePromptRequired     = "skill_title_prompt_required"
)

func writeSkillDefinitionNotFound(w http.ResponseWriter) {
	writeErrorCode(w, http.StatusNotFound, errorCodeSkillDefinitionNotFound, "skill definition not found")
}

func writeSkillDefinitionRequired(w http.ResponseWriter) {
	writeAPIError(w, http.StatusBadRequest, errorCodeSkillDefinitionRequired, "definitionId is required", "definitionId", nil)
}

func writeSkillInvalidRequestBody(w http.ResponseWriter) {
	writeErrorCode(w, http.StatusBadRequest, errorCodeSkillInvalidRequestBody, "invalid request body")
}

func writeSkillConflict(w http.ResponseWriter) {
	writeErrorCode(w, http.StatusConflict, errorCodeSkillConflict, "skill already exists")
}

func writeSkillInstallationNotFound(w http.ResponseWriter) {
	writeErrorCode(w, http.StatusNotFound, errorCodeSkillInstallationNotFound, "skill installation not found")
}

func writeSkillRevisionNotFound(w http.ResponseWriter, status int) {
	writeErrorCode(w, status, errorCodeSkillRevisionNotFound, "skill revision not found")
}

func writeSkillInstallationConflict(w http.ResponseWriter) {
	writeErrorCode(w, http.StatusConflict, errorCodeSkillInstallationConflict, "skill installation already exists")
}

func writeSkillInstallationOnlyFields(w http.ResponseWriter, message string, fields []string) {
	writeAPIError(w, http.StatusBadRequest, errorCodeSkillInstallationOnlyFields, message, "", errorDetailsForFields(fields))
}

func writeSkillInvalidFieldCombination(w http.ResponseWriter, message string, fields []string) {
	writeAPIError(w, http.StatusBadRequest, errorCodeSkillInvalidFieldCombination, message, "", errorDetailsForFields(fields))
}

func writeSkillNoUpdatableFields(w http.ResponseWriter, allowedFields []string) {
	writeAPIError(w, http.StatusBadRequest, errorCodeSkillNoUpdatableFields, "no updatable fields provided", "", errorDetailsWithKey("allowedFields", allowedFields))
}

func writeSkillRevisionRequired(w http.ResponseWriter) {
	writeAPIError(w, http.StatusBadRequest, errorCodeSkillRevisionRequired, "currentRevisionId is required", "currentRevisionId", nil)
}

func writeSkillTitlePromptRequired(w http.ResponseWriter) {
	writeAPIError(w, http.StatusBadRequest, errorCodeSkillTitlePromptRequired, "title and prompt are required", "", errorDetailsForFields([]string{"title", "prompt"}))
}

func writeSkillValidationError(w http.ResponseWriter, err error) bool {
	var validationErr *skillsvc.ValidationError
	if !errors.As(err, &validationErr) {
		return false
	}
	switch validationErr.Code {
	case skillsvc.ValidationInstallationOnlyFields:
		writeSkillInstallationOnlyFields(w, validationErr.Message, validationErr.Fields)
	case skillsvc.ValidationInvalidFieldCombination:
		writeSkillInvalidFieldCombination(w, validationErr.Message, validationErr.Fields)
	case skillsvc.ValidationNoUpdatableFields:
		writeSkillNoUpdatableFields(w, validationErr.AllowedFields)
	default:
		return false
	}
	return true
}

func writeSkillSelectionError(w http.ResponseWriter, err error) bool {
	status, code, message, ok := skillSelectionErrorResponse(err)
	if !ok {
		return false
	}
	writeErrorCode(w, status, code, message)
	return true
}

func streamSkillSelectionError(w http.ResponseWriter, flusher http.Flusher, err error) bool {
	_, code, message, ok := skillSelectionErrorResponse(err)
	if !ok {
		return false
	}
	streamError(w, flusher, code, message)
	return true
}

func skillSelectionErrorResponse(err error) (int, string, string, bool) {
	switch {
	case errors.Is(err, skillsvc.ErrDefinitionNotFound),
		errors.Is(err, run.ErrSkillDefinitionNotFound),
		errors.Is(err, chat.ErrSkillDefinitionNotFound):
		return http.StatusNotFound, errorCodeSkillDefinitionNotFound, "skill definition not found", true
	case errors.Is(err, skillsvc.ErrInstallationNotFound),
		errors.Is(err, run.ErrSkillInstallationNotFound),
		errors.Is(err, chat.ErrSkillInstallationNotFound):
		return http.StatusNotFound, errorCodeSkillInstallationNotFound, "skill installation not found", true
	case errors.Is(err, skillsvc.ErrDefinitionMismatch),
		errors.Is(err, run.ErrSkillDefinitionMismatch),
		errors.Is(err, chat.ErrSkillDefinitionMismatch):
		return http.StatusBadRequest, errorCodeSkillDefinitionMismatch, err.Error(), true
	case errors.Is(err, skillsvc.ErrNoEnabledInstallation),
		errors.Is(err, run.ErrNoEnabledSkillInstallation),
		errors.Is(err, chat.ErrNoEnabledSkillInstallation):
		return http.StatusBadRequest, errorCodeNoEnabledSkillInstallation, "no enabled skill installation available for skill definition", true
	case errors.Is(err, skillsvc.ErrInstallationDisabled),
		errors.Is(err, run.ErrSkillInstallationDisabled),
		errors.Is(err, chat.ErrSkillInstallationDisabled):
		return http.StatusBadRequest, errorCodeSkillInstallationDisabled, "skill installation is disabled", true
	default:
		return 0, "", "", false
	}
}

func errorDetailsForFields(fields []string) map[string]any {
	return errorDetailsWithKey("fields", fields)
}
