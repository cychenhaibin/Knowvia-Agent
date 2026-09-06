package httpapi

import (
	"net/http"
	"strings"
)

type apiErrorPayload struct {
	Code    string         `json:"code,omitempty"`
	Error   string         `json:"error"`
	Field   string         `json:"field,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

const (
	errorCodeInvalidRequestBody = "invalid_request_body"
	errorCodeUnauthorized       = "unauthorized"
	errorCodeNotFound           = "not_found"
	errorCodeConflict           = "conflict"
	errorCodeBadGateway         = "bad_gateway"
	errorCodeValidationFailed   = "validation_failed"
	errorCodeInternalError      = "internal_error"
	errorCodeServiceUnavailable = "service_unavailable"
	errorCodeRateLimited        = "rate_limited"
)

func writeAPIError(w http.ResponseWriter, status int, code, message, field string, details map[string]any) {
	writeJSON(w, status, apiErrorPayload{
		Code:    code,
		Error:   message,
		Field:   field,
		Details: normalizeErrorDetails(details),
	})
}

func writeErrorCode(w http.ResponseWriter, status int, code, message string) {
	writeAPIError(w, status, code, message, "", nil)
}

func writeInvalidRequestBody(w http.ResponseWriter) {
	writeErrorCode(w, http.StatusBadRequest, errorCodeInvalidRequestBody, "invalid request body")
}

func writeUnauthorized(w http.ResponseWriter, message string) {
	writeErrorCode(w, http.StatusUnauthorized, errorCodeUnauthorized, message)
}

func writeNotFound(w http.ResponseWriter, message, resource string) {
	writeAPIError(w, http.StatusNotFound, errorCodeNotFound, message, "", errorDetailResource(resource))
}

func writeConflict(w http.ResponseWriter, message, resource string) {
	writeAPIError(w, http.StatusConflict, errorCodeConflict, message, "", errorDetailResource(resource))
}

func writeBadGateway(w http.ResponseWriter, message string, details map[string]any) {
	writeAPIError(w, http.StatusBadGateway, errorCodeBadGateway, message, "", details)
}

func writeValidationError(w http.ResponseWriter, message string, fields []string) {
	if len(fields) == 1 {
		writeAPIError(w, http.StatusBadRequest, errorCodeValidationFailed, message, fields[0], errorDetailsWithKey("fields", fields))
		return
	}
	writeAPIError(w, http.StatusBadRequest, errorCodeValidationFailed, message, "", errorDetailsWithKey("fields", fields))
}

func writeInternalError(w http.ResponseWriter, message string) {
	writeErrorCode(w, http.StatusInternalServerError, errorCodeInternalError, message)
}

func writeServiceUnavailable(w http.ResponseWriter, message string) {
	writeErrorCode(w, http.StatusServiceUnavailable, errorCodeServiceUnavailable, message)
}

func streamError(w http.ResponseWriter, flusher http.Flusher, code, message string) {
	payload := map[string]any{
		"type":  "error",
		"error": message,
	}
	if strings.TrimSpace(code) != "" {
		payload["code"] = code
	}
	streamJSON(w, flusher, payload)
}

func streamInternalError(w http.ResponseWriter, flusher http.Flusher, message string) {
	streamError(w, flusher, errorCodeInternalError, message)
}

func errorDetailsWithKey(key string, values []string) map[string]any {
	if len(values) == 0 {
		return nil
	}
	items := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		items = append(items, value)
	}
	if len(items) == 0 {
		return nil
	}
	return map[string]any{key: items}
}

func errorDetailResource(resource string) map[string]any {
	resource = strings.TrimSpace(resource)
	if resource == "" {
		return nil
	}
	return map[string]any{"resource": resource}
}

func errorDetailsString(key, value string) map[string]any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return map[string]any{key: value}
}

func normalizeErrorDetails(details map[string]any) map[string]any {
	if len(details) == 0 {
		return nil
	}
	return details
}
