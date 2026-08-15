package actions

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

func TestRenderImageActionErrorPreservesApplicableRenderCode(t *testing.T) {
	t.Parallel()

	root := errors.New("renderer stopped")
	err := &RenderTemplateError{
		Code:    "platform.render_timeout",
		Message: "render execution timed out",
		Err:     root,
	}

	actionErr := renderImageActionError(err)
	if actionErr.Code != "platform.render_timeout" || actionErr.Message != "render execution timed out" {
		t.Fatalf("unexpected action error: %#v", actionErr)
	}
	if !errors.Is(actionErr, root) {
		t.Fatal("action error must retain the renderer cause")
	}
}

func TestRenderImageActionErrorMapsInapplicableCode(t *testing.T) {
	t.Parallel()

	err := &RenderTemplateError{
		Code:    "platform.invalid_request",
		Message: "render input does not match the template schema",
	}

	actionErr := renderImageActionError(err)
	if actionErr.Code != "plugin.internal_error" {
		t.Fatalf("action error code = %q, want plugin.internal_error", actionErr.Code)
	}
	if actionErr.Message != err.Message {
		t.Fatalf("action error message = %q, want %q", actionErr.Message, err.Message)
	}
}

func TestLogRenderImageFailureRecordsRedactedRootCause(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	deps := Deps{
		Logger: slog.New(slog.NewJSONHandler(&output, nil)),
		RedactText: func(value string) string {
			return strings.ReplaceAll(value, "secret-value", "[redacted]")
		},
	}
	err := &RenderTemplateError{
		Code:    "platform.invalid_request",
		Message: "render input does not match the template schema",
		Err:     errors.New("schema validation failed for secret-value"),
	}

	logRenderImageFailure(deps, ActionRequest{
		PluginID:  "raylea.subscription-hub",
		RequestID: "render-request-1",
	}, "render", "plugin.raylea.subscription-hub.weibo-update", err)

	var record map[string]any
	if unmarshalErr := json.Unmarshal(output.Bytes(), &record); unmarshalErr != nil {
		t.Fatalf("decode log record: %v", unmarshalErr)
	}
	if record["phase"] != "render" || record["error_code"] != "platform.invalid_request" {
		t.Fatalf("unexpected log record: %#v", record)
	}
	if record["cause"] != "schema validation failed for [redacted]" {
		t.Fatalf("log cause was not redacted: %#v", record["cause"])
	}
	if strings.Contains(output.String(), "secret-value") {
		t.Fatalf("log contains unredacted secret: %s", output.String())
	}
}
