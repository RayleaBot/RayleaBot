package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
)

type requestIDKey struct{}

type ErrorEnvelope struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code       string         `json:"code"`
	Message    string         `json:"message"`
	MessageKey string         `json:"message_key"`
	RequestID  string         `json:"request_id"`
	Details    map[string]any `json:"details,omitempty"`
}

func WithRequestContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := newRequestID()
		ctx := context.WithValue(r.Context(), requestIDKey{}, requestID)
		r = r.WithContext(ctx)

		defer func() {
			if recover() != nil {
				WriteError(
					w,
					r,
					http.StatusInternalServerError,
					"platform.internal_error",
					"内部错误",
					"errors.platform.internal_error",
					nil,
				)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if requestID, ok := ctx.Value(requestIDKey{}).(string); ok {
		return requestID
	}
	return ""
}

func WriteError(w http.ResponseWriter, r *http.Request, statusCode int, code, message, messageKey string, details map[string]any) {
	requestID := ""
	if r != nil {
		requestID = RequestIDFromContext(r.Context())
	}
	if requestID == "" {
		requestID = newRequestID()
	}

	WriteJSON(
		w,
		statusCode,
		ErrorEnvelope{
			Error: ErrorBody{
				Code:       code,
				Message:    message,
				MessageKey: messageKey,
				RequestID:  requestID,
				Details:    details,
			},
		},
	)
}

func WriteJSON(w http.ResponseWriter, statusCode int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(body)
}

func newRequestID() string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return "req_0000000000000000"
	}

	return "req_" + hex.EncodeToString(bytes)
}
