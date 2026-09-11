package desktop

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeRecoverySummary(t *testing.T, payload string) string {
	t.Helper()
	logDirectory := t.TempDir()
	if err := os.WriteFile(filepath.Join(logDirectory, "recovery-summary.json"), []byte(payload), 0o600); err != nil {
		t.Fatal(err)
	}
	return logDirectory
}

func TestReadRecoverySummaryPreservesValidatedContractShape(t *testing.T) {
	summary, err := readRecoverySummary(writeRecoverySummary(t, `{
		"status":"degraded",
		"phase":"post_startup",
		"operation":"restore",
		"created_at":"2026-08-17T10:00:00Z",
		"updated_at":"2026-08-17T10:01:00Z",
		"requires_post_start_checks":true,
		"issues":[{"code":"plugin.review","severity":"warning","summary":" Review required ","remediation":" Review the plugin "}],
		"skipped_plugins":[{"plugin_id":"calendar","reason_code":"plugin.incompatible","summary":"Skipped","review_id":"review-1","review_status":"pending"}],
		"manual_actions":[" Review plugin ",""],
		"next_steps":[" Continue recovery "],
		"audit":[{"task_id":"task-1","created_at":"2026-08-17T10:02:00Z","operator_id":"admin","note":" reviewed ","items":[{"review_id":"review-1","plugin_id":"calendar","reason_code":"plugin.incompatible","summary":"Confirmed"}]}]
	}`))
	if err != nil || summary == nil {
		t.Fatalf("readRecoverySummary() rejected a valid contract payload: %v", err)
	}
	if summary.Status != "degraded" || summary.Phase != "post_startup" || summary.Operation != "restore" {
		t.Fatalf("summary identity = %#v", summary)
	}
	if len(summary.ManualActions) != 2 || summary.ManualActions[0] != " Review plugin " {
		t.Fatalf("valid payload text was changed: %#v", summary.ManualActions)
	}
	if len(summary.Issues) != 1 || summary.Issues[0].Summary != " Review required " {
		t.Fatalf("issues = %#v", summary.Issues)
	}
}

func TestReadRecoverySummaryRejectsContractDrift(t *testing.T) {
	tests := map[string]string{
		"missing required phase": `{"status":"compatible","operation":"restore","created_at":"2026-08-17T10:00:00Z","updated_at":"2026-08-17T10:01:00Z"}`,
		"unknown property":       `{"status":"compatible","phase":"pre_restore","operation":"restore","created_at":"2026-08-17T10:00:00Z","updated_at":"2026-08-17T10:01:00Z","legacy_status":"ok"}`,
		"invalid date time":      `{"status":"compatible","phase":"pre_restore","operation":"restore","created_at":"yesterday","updated_at":"2026-08-17T10:01:00Z"}`,
		"invalid nested enum":    `{"status":"blocked","phase":"pre_restore","operation":"restore","created_at":"2026-08-17T10:00:00Z","updated_at":"2026-08-17T10:01:00Z","issues":[{"code":"broken","severity":"fatal","summary":"Broken"}]}`,
		"removed operation":      `{"status":"compatible","phase":"pre_restore","operation":"upgrade","created_at":"2026-08-17T10:00:00Z","updated_at":"2026-08-17T10:01:00Z"}`,
		"missing audit note":     `{"status":"compatible","phase":"post_startup","operation":"restore","created_at":"2026-08-17T10:00:00Z","updated_at":"2026-08-17T10:01:00Z","audit":[{"task_id":"task-1","created_at":"2026-08-17T10:02:00Z","operator_id":"admin","items":[]}]}`,
		"null optional field":    `{"status":"compatible","phase":"pre_restore","operation":"restore","created_at":"2026-08-17T10:00:00Z","updated_at":"2026-08-17T10:01:00Z","issues":null}`,
	}
	for name, payload := range tests {
		t.Run(name, func(t *testing.T) {
			summary, err := readRecoverySummary(writeRecoverySummary(t, payload))
			var boundary *BoundaryError
			if summary != nil || !errors.As(err, &boundary) || boundary.Code != "launcher.recovery_summary_invalid" {
				t.Fatalf("readRecoverySummary() = %#v, %v; want launcher.recovery_summary_invalid", summary, err)
			}
		})
	}
}
