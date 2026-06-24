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
	"time"

	"github.com/go-chi/chi/v5"
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

type DomainError struct {
	Code        string
	HTTPStatus  int
	SafeMessage string
	MessageKey  string
	Details     map[string]any
	Cause       error
}

func (e *DomainError) Error() string {
	if e == nil {
		return ""
	}
	if strings.TrimSpace(e.SafeMessage) != "" {
		return e.SafeMessage
	}
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return e.Code
}

func (e *DomainError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

type RequestObserver interface {
	ObserveHTTPRequest(method, route string, status int, duration time.Duration)
	ObserveHTTPPanic(method, route string)
}

type requestContextOptions struct {
	observer RequestObserver
}

type RequestContextOption func(*requestContextOptions)

func WithRequestObserver(observer RequestObserver) RequestContextOption {
	return func(options *requestContextOptions) {
		options.observer = observer
	}
}

func WithRequestContext(logger *slog.Logger, opts ...RequestContextOption) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	options := requestContextOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startedAt := time.Now()
			requestID := newRequestID()
			ctx := context.WithValue(r.Context(), requestIDKey{}, requestID)
			r = r.WithContext(ctx)
			recorder := &statusResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			defer func() {
				recovered := recover()
				route := requestRoutePattern(r)
				if recovered != nil {
					if options.observer != nil {
						options.observer.ObserveHTTPPanic(r.Method, route)
					}

					logger.Error(
						"panic recovered",
						"component", "http",
						"request_id", requestID,
						"method", r.Method,
						"path", r.URL.Path,
						"route", route,
						"panic", fmt.Sprint(recovered),
						"stack", string(debug.Stack()),
					)
					WriteError(
						recorder,
						r,
						http.StatusInternalServerError,
						"platform.internal_error",
						"内部错误",
						"errors.platform.internal_error",
						nil,
					)
				}

				duration := time.Since(startedAt)
				if duration <= 0 {
					duration = time.Nanosecond
				}
				if options.observer != nil {
					options.observer.ObserveHTTPRequest(r.Method, route, recorder.statusCode, duration)
				}
				logger.Info(
					"http request completed",
					"component", "http",
					"request_id", requestID,
					"method", r.Method,
					"path", r.URL.Path,
					"route", route,
					"status", recorder.statusCode,
					"duration_ms", duration.Milliseconds(),
				)
			}()

			next.ServeHTTP(recorder, r)
		})
	}
}

type statusResponseWriter struct {
	http.ResponseWriter
	statusCode  int
	wroteHeader bool
}

func (w *statusResponseWriter) WriteHeader(statusCode int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *statusResponseWriter) Write(bytes []byte) (int, error) {
	if w.statusCode == 0 {
		w.statusCode = http.StatusOK
	}
	return w.ResponseWriter.Write(bytes)
}

func (w *statusResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func requestRoutePattern(r *http.Request) string {
	if r == nil {
		return "unknown"
	}
	if routeContext := chi.RouteContext(r.Context()); routeContext != nil {
		if pattern := routeContext.RoutePattern(); pattern != "" {
			return pattern
		}
	}
	if r.URL == nil || strings.TrimSpace(r.URL.Path) == "" {
		return "unknown"
	}
	return "unmatched"
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

func WriteDomainError(w http.ResponseWriter, r *http.Request, err *DomainError) {
	if err == nil {
		WriteError(w, r, http.StatusInternalServerError, "platform.upstream_request_failed", "请求处理失败", "errors.platform.upstream_request_failed", nil)
		return
	}
	statusCode := err.HTTPStatus
	if statusCode == 0 {
		statusCode = http.StatusInternalServerError
	}
	code := strings.TrimSpace(err.Code)
	if code == "" {
		code = "platform.upstream_request_failed"
	}
	messageKey := strings.TrimSpace(err.MessageKey)
	if messageKey == "" {
		messageKey = "errors.platform.upstream_request_failed"
	}
	message := strings.TrimSpace(err.SafeMessage)
	if message == "" {
		message = "请求处理失败"
	}
	WriteError(w, r, statusCode, code, message, messageKey, err.Details)
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
