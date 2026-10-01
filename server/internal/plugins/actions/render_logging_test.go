package actions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"testing"
)

type stagedRenderFailure struct{ error }

func (e stagedRenderFailure) Unwrap() error        { return e.error }
func (stagedRenderFailure) RenderingPhase() string { return "browser_startup" }

func TestRenderFailureLogKeepsExecutionStage(t *testing.T) {
	var output bytes.Buffer
	failure := fmt.Errorf("render failed: %w", stagedRenderFailure{context.DeadlineExceeded})
	logRenderImageFailure(Deps{Logger: slog.New(slog.NewJSONHandler(&output, nil))}, ActionRequest{PluginID: "fixture", RequestID: "fixture-render"}, "render", "fixture.card", failure)
	var entry map[string]any
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["phase"] != "render" || entry["render_stage"] != "browser_startup" || entry["cause"] != context.DeadlineExceeded.Error() {
		t.Fatalf("render diagnostics lost the execution stage: %#v", entry)
	}
}
