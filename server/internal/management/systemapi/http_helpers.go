package systemapi

import (
	"net/http"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
)

const (
	codePermissionDenied = "permission.denied"
	codeInvalidRequest   = "platform.invalid_request"
	codeResourceMissing  = "platform.resource_missing"
	codeInternalError    = "platform.internal_error"
)

type taskAcceptedResponse struct {
	TaskID string `json:"task_id"`
}

func writeAppError(w http.ResponseWriter, r *http.Request, statusCode int, code, message, messageKey string, details map[string]any) {
	httpapi.WriteError(w, r, statusCode, code, message, messageKey, details)
}

func writeAuthError(w http.ResponseWriter, r *http.Request, statusCode int, code, message, messageKey string) {
	writeAppError(w, r, statusCode, code, message, messageKey, nil)
}

func writeAuthJSON(w http.ResponseWriter, statusCode int, body any) {
	httpapi.WriteJSON(w, statusCode, body)
}
