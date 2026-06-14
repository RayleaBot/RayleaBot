package protocolapi

import (
	"net/http"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
)

const (
	codeInvalidRequest = "platform.invalid_request"
	codeInternalError  = "platform.internal_error"
)

type Handlers = ProtocolHandlers
type Service = ProtocolService

var NewHandlers = NewProtocolHandlers
var NewService = NewProtocolService

func writeAppError(w http.ResponseWriter, r *http.Request, statusCode int, code, message, messageKey string, details map[string]any) {
	httpapi.WriteError(w, r, statusCode, code, message, messageKey, details)
}

func writeAuthJSON(w http.ResponseWriter, statusCode int, body any) {
	httpapi.WriteJSON(w, statusCode, body)
}
