package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
)

const (
	MaxManagementJSONBodyBytes int64 = 1 << 20
	MaxWebhookBodyBytes        int64 = 1 << 20
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

func WithRequestContext(logger *slog.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := newRequestID()
			ctx := context.WithValue(r.Context(), requestIDKey{}, requestID)
			r = r.WithContext(ctx)

			defer func() {
				recovered := recover()
				if recovered == nil {
					return
				}

				logger.Error(
					"panic recovered",
					"component", "http",
					"request_id", requestID,
					"method", r.Method,
					"path", r.URL.Path,
					"panic", fmt.Sprint(recovered),
					"stack", string(debug.Stack()),
				)
				WriteError(
					w,
					r,
					http.StatusInternalServerError,
					"platform.internal_error",
					"内部错误",
					"errors.platform.internal_error",
					nil,
				)
			}()

			next.ServeHTTP(w, r)
		})
	}
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

func DecodeStrictJSON(w http.ResponseWriter, r *http.Request, target any, maxBytes int64) error {
	reader := http.MaxBytesReader(w, r.Body, maxBytes)
	defer reader.Close()

	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}

	var trailing any
	if err := decoder.Decode(&trailing); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}

	return errors.New("unexpected trailing JSON content")
}

func ReadRequestBody(w http.ResponseWriter, r *http.Request, maxBytes int64) ([]byte, error) {
	reader := http.MaxBytesReader(w, r.Body, maxBytes)
	defer reader.Close()

	return io.ReadAll(reader)
}

func RequestRemoteIP(r *http.Request) string {
	if r == nil {
		return ""
	}

	host := strings.TrimSpace(r.RemoteAddr)
	if host == "" {
		return ""
	}

	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}

	return strings.Trim(host, "[]")
}

func newRequestID() string {
	bytes := make([]byte, 16)
	_, _ = rand.Read(bytes) // crypto/rand.Read never returns an error in Go 1.25+

	return "req_" + hex.EncodeToString(bytes)
}
