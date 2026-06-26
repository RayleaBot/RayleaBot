package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWithRequestContextRecoversPanicAndLogsStack(t *testing.T) {
	t.Parallel()

	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuffer, &slog.HandlerOptions{Level: slog.LevelError}))

	handler := WithRequestContext(logger)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))

	request := httptest.NewRequest(http.MethodGet, "/api/panic?session_token=fixture-only-secret", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status: got %d want %d", recorder.Code, http.StatusInternalServerError)
	}

	var envelope ErrorEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if envelope.Error.Code != "platform.internal_error" {
		t.Fatalf("unexpected error code: got %q want %q", envelope.Error.Code, "platform.internal_error")
	}
	if !strings.HasPrefix(envelope.Error.RequestID, "req_") {
		t.Fatalf("unexpected request id: %q", envelope.Error.RequestID)
	}

	logLines := strings.Split(strings.TrimSpace(logBuffer.String()), "\n")
	if len(logLines) != 1 {
		t.Fatalf("expected exactly one panic log line, got %d", len(logLines))
	}
	if strings.Contains(logLines[0], "session_token") || strings.Contains(logLines[0], "fixture-only-secret") {
		t.Fatalf("panic log leaked raw query: %s", logLines[0])
	}

	var record map[string]any
	if err := json.Unmarshal([]byte(logLines[0]), &record); err != nil {
		t.Fatalf("decode panic log: %v", err)
	}

	if got := record["msg"]; got != "panic recovered" {
		t.Fatalf("unexpected log message: got %#v want %#v", got, "panic recovered")
	}
	if got := record["panic"]; got != "boom" {
		t.Fatalf("unexpected panic field: got %#v want %#v", got, "boom")
	}
	if got := record["method"]; got != http.MethodGet {
		t.Fatalf("unexpected method: got %#v want %#v", got, http.MethodGet)
	}
	if got := record["path"]; got != "/api/panic" {
		t.Fatalf("unexpected path: got %#v want %#v", got, "/api/panic")
	}
	if got := record["request_id"]; got != envelope.Error.RequestID {
		t.Fatalf("unexpected request_id in log: got %#v want %#v", got, envelope.Error.RequestID)
	}

	stack, ok := record["stack"].(string)
	if !ok || stack == "" {
		t.Fatalf("expected non-empty stack in panic log, got %#v", record["stack"])
	}
	if !strings.Contains(stack, "goroutine") {
		t.Fatalf("expected stack trace content, got %q", stack)
	}
}

func TestWithRequestContextLogsAccessAndObservesRequest(t *testing.T) {
	t.Parallel()

	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuffer, &slog.HandlerOptions{Level: slog.LevelInfo}))
	observer := &recordingRequestObserver{}
	handler := WithRequestContext(logger, WithRequestObserver(observer))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requestID := RequestIDFromContext(r.Context()); !strings.HasPrefix(requestID, "req_") {
			t.Fatalf("unexpected request id in context: %q", requestID)
		}
		WriteJSON(w, http.StatusCreated, map[string]string{"ok": "true"})
	}))

	request := httptest.NewRequest(http.MethodPost, "/api/config?access_token=fixture-only-secret", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("unexpected status: got %d want %d", recorder.Code, http.StatusCreated)
	}
	logLines := strings.Split(strings.TrimSpace(logBuffer.String()), "\n")
	if len(logLines) != 1 {
		t.Fatalf("expected one access log line, got %d", len(logLines))
	}
	if strings.Contains(logLines[0], "access_token") || strings.Contains(logLines[0], "fixture-only-secret") {
		t.Fatalf("access log leaked raw query: %s", logLines[0])
	}

	var record map[string]any
	if err := json.Unmarshal([]byte(logLines[0]), &record); err != nil {
		t.Fatalf("decode access log: %v", err)
	}
	if got := record["msg"]; got != "http request completed" {
		t.Fatalf("unexpected log message: got %#v want %#v", got, "http request completed")
	}
	if got := record["method"]; got != http.MethodPost {
		t.Fatalf("unexpected method: got %#v want %#v", got, http.MethodPost)
	}
	if got := record["path"]; got != "/api/config" {
		t.Fatalf("unexpected path: got %#v want %#v", got, "/api/config")
	}
	if got := record["route"]; got != "unmatched" {
		t.Fatalf("unexpected route: got %#v want %#v", got, "unmatched")
	}
	if got := record["status"]; got != float64(http.StatusCreated) {
		t.Fatalf("unexpected status: got %#v want %#v", got, http.StatusCreated)
	}
	if requestID, ok := record["request_id"].(string); !ok || !strings.HasPrefix(requestID, "req_") {
		t.Fatalf("unexpected access log request_id: %#v", record["request_id"])
	}
	if observer.request.method != http.MethodPost || observer.request.route != "unmatched" || observer.request.status != http.StatusCreated {
		t.Fatalf("unexpected observed request: %#v", observer.request)
	}
	if observer.request.duration <= 0 {
		t.Fatalf("expected positive observed duration, got %s", observer.request.duration)
	}
}

func TestWithRequestContextDowngradesSuccessfulManagementReadsToDebug(t *testing.T) {
	t.Parallel()

	var logBuffer bytes.Buffer
	infoLogger := slog.New(slog.NewJSONHandler(&logBuffer, &slog.HandlerOptions{Level: slog.LevelInfo}))
	successHandler := WithRequestContext(infoLogger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"ok": "true"})
	}))

	successHandler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/logs/log_0001", nil))
	if strings.TrimSpace(logBuffer.String()) != "" {
		t.Fatalf("successful management read should not be emitted at info level: %s", logBuffer.String())
	}

	debugLogger := slog.New(slog.NewJSONHandler(&logBuffer, &slog.HandlerOptions{Level: slog.LevelDebug}))
	successHandler = WithRequestContext(debugLogger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"ok": "true"})
	}))
	successHandler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/logs/log_0001", nil))

	var record map[string]any
	if err := json.Unmarshal(logBuffer.Bytes(), &record); err != nil {
		t.Fatalf("decode debug access log: %v", err)
	}
	if got := record["level"]; got != "DEBUG" {
		t.Fatalf("successful management read should be debug, got %#v", got)
	}
	if got := record["path"]; got != "/api/logs/log_0001" {
		t.Fatalf("unexpected path: got %#v want %#v", got, "/api/logs/log_0001")
	}

	logBuffer.Reset()
	failedHandler := WithRequestContext(infoLogger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "failed", http.StatusInternalServerError)
	}))
	failedHandler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/logs/log_0001", nil))
	if err := json.Unmarshal(logBuffer.Bytes(), &record); err != nil {
		t.Fatalf("decode failed access log: %v", err)
	}
	if got := record["level"]; got != "INFO" {
		t.Fatalf("failed management read should stay info, got %#v", got)
	}
}

func TestWithRequestContextObservesPanic(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	observer := &recordingRequestObserver{}
	handler := WithRequestContext(logger, WithRequestObserver(observer))(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/panic", nil))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status: got %d want %d", recorder.Code, http.StatusInternalServerError)
	}
	if observer.panic.method != http.MethodGet || observer.panic.route != "unmatched" {
		t.Fatalf("unexpected observed panic: %#v", observer.panic)
	}
	if observer.request.status != http.StatusInternalServerError {
		t.Fatalf("unexpected observed request status after panic: %#v", observer.request)
	}
}

type recordingRequestObserver struct {
	request struct {
		method   string
		route    string
		status   int
		duration time.Duration
	}
	panic struct {
		method string
		route  string
	}
}

func (o *recordingRequestObserver) ObserveHTTPRequest(method, route string, status int, duration time.Duration) {
	o.request.method = method
	o.request.route = route
	o.request.status = status
	o.request.duration = duration
}

func (o *recordingRequestObserver) ObserveHTTPPanic(method, route string) {
	o.panic.method = method
	o.panic.route = route
}
