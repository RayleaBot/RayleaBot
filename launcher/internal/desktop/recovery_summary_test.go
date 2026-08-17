package desktop

import "testing"

func TestParseRecoverySummaryValidatesAndSanitizesContractShape(t *testing.T) {
	summary := parseRecoverySummary([]byte(`{
		"status":"degraded",
		"phase":"post_startup",
		"operation":"upgrade",
		"created_at":"2026-08-17T10:00:00Z",
		"updated_at":"2026-08-17T10:01:00Z",
		"requires_post_start_checks":true,
		"issues":[{"code":"plugin.review","severity":"warning","summary":" Review required ","remediation":" Review the plugin "}],
		"skipped_plugins":[{"plugin_id":"calendar","reason_code":"plugin.incompatible","summary":"Skipped","review_id":"review-1","review_status":"pending"}],
		"manual_actions":[" Review plugin ",""],
		"next_steps":[" Continue recovery "],
		"audit":[{"task_id":"task-1","created_at":"2026-08-17T10:02:00Z","operator_id":"admin","note":" reviewed ","items":[{"review_id":"review-1","plugin_id":"calendar","reason_code":"plugin.incompatible","summary":"Confirmed"}]}]
	}`))
	if summary == nil {
		t.Fatal("parseRecoverySummary() rejected a valid contract payload")
	}
	if summary["status"] != "degraded" || summary["phase"] != "post_startup" || summary["operation"] != "upgrade" {
		t.Fatalf("summary identity fields = %#v", summary)
	}
	manualActions, ok := summary["manual_actions"].([]any)
	if !ok || len(manualActions) != 1 || manualActions[0] != "Review plugin" {
		t.Fatalf("manual_actions = %#v", summary["manual_actions"])
	}
	issues, ok := summary["issues"].([]any)
	if !ok || len(issues) != 1 {
		t.Fatalf("issues = %#v", summary["issues"])
	}
	issue, ok := issues[0].(map[string]any)
	if !ok || issue["summary"] != "Review required" || issue["remediation"] != "Review the plugin" {
		t.Fatalf("normalized issue = %#v", issues[0])
	}
}

func TestParseRecoverySummaryRejectsContractDrift(t *testing.T) {
	tests := map[string]string{
		"missing required phase": `{"status":"compatible","operation":"restore","created_at":"2026-08-17T10:00:00Z","updated_at":"2026-08-17T10:01:00Z"}`,
		"unknown property":       `{"status":"compatible","phase":"pre_restore","operation":"restore","created_at":"2026-08-17T10:00:00Z","updated_at":"2026-08-17T10:01:00Z","legacy_status":"ok"}`,
		"invalid date time":      `{"status":"compatible","phase":"pre_restore","operation":"restore","created_at":"yesterday","updated_at":"2026-08-17T10:01:00Z"}`,
		"invalid nested enum":    `{"status":"blocked","phase":"pre_restore","operation":"rollback","created_at":"2026-08-17T10:00:00Z","updated_at":"2026-08-17T10:01:00Z","issues":[{"code":"broken","severity":"fatal","summary":"Broken"}]}`,
		"missing audit note":     `{"status":"compatible","phase":"post_startup","operation":"restore","created_at":"2026-08-17T10:00:00Z","updated_at":"2026-08-17T10:01:00Z","audit":[{"task_id":"task-1","created_at":"2026-08-17T10:02:00Z","operator_id":"admin","items":[]}]}`,
		"null optional field":    `{"status":"compatible","phase":"pre_restore","operation":"restore","created_at":"2026-08-17T10:00:00Z","updated_at":"2026-08-17T10:01:00Z","issues":null}`,
	}
	for name, payload := range tests {
		t.Run(name, func(t *testing.T) {
			if summary := parseRecoverySummary([]byte(payload)); summary != nil {
				t.Fatalf("parseRecoverySummary() = %#v, want nil", summary)
			}
		})
	}
}
