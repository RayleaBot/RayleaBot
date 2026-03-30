package httpapi

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWithRequestContextRecoversPanicAndLogsStack(t *testing.T) {
	t.Parallel()

	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuffer, &slog.HandlerOptions{Level: slog.LevelError}))

	handler := WithRequestContext(logger)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))

	request := httptest.NewRequest(http.MethodGet, "/api/panic", nil)
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
