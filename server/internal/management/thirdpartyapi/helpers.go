package thirdpartyapi

import "time"

const (
	codeInvalidRequest = "platform.invalid_request"
	codeInternalError  = "platform.internal_error"
)

func timeString(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func timeStringPtr(value *time.Time) *string {
	if value == nil || value.IsZero() {
		return nil
	}
	text := value.UTC().Format(time.RFC3339)
	return &text
}
