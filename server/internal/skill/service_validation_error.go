package skill

func newValidationError(code ValidationErrorCode, message string, fields, allowedFields []string) error {
	validationErr := &ValidationError{
		Code:    code,
		Message: message,
	}
	if len(fields) > 0 {
		validationErr.Fields = append([]string(nil), fields...)
	}
	if len(allowedFields) > 0 {
		validationErr.AllowedFields = append([]string(nil), allowedFields...)
	}
	return validationErr
}
