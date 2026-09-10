package system

import (
	"errors"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/health"
)

func TestRuntimeResourceSelectionSurvivesReadinessProjectionWithoutAliasing(t *testing.T) {
	t.Parallel()
	issue := startupFailureIssue("ffmpeg", errors.New("Chromium text is irrelevant to the selected resource"))
	if len(issue.RuntimeResources) != 1 || issue.RuntimeResources[0] != "ffmpeg" {
		t.Fatalf("startup issue selected wrong resource: %#v", issue)
	}
	s := &Service{}
	s.setStartupRuntimeState("ffmpeg", StartupRuntimePhaseFailed, &issue)
	issue.RuntimeResources[0] = "chromium"
	state, ok := s.startupRuntimeState("ffmpeg")
	if !ok || state.Issue.RuntimeResources[0] != "ffmpeg" {
		t.Fatalf("caller mutation changed runtime state: %#v", state)
	}
	state.Issue.RuntimeResources[0] = "chromium"
	state, _ = s.startupRuntimeState("ffmpeg")
	if state.Issue.RuntimeResources[0] != "ffmpeg" {
		t.Fatal("snapshot shared the stored resource slice")
	}
}

func TestDiagnosticsIssuesExposeUserAndInternalFields(t *testing.T) {
	t.Parallel()

	items := dedupeDiagnosticIssues([]health.DiagnosticIssue{{
		Code:        "render.chromium_missing",
		Severity:    "warning",
		Summary:     "Chromium 不可用",
		Remediation: "请准备 Chromium 运行环境。",
	}})

	if len(items) != 1 {
		t.Fatalf("dedupeDiagnosticIssues returned %d items, want 1", len(items))
	}
	if items[0].UserMessage != items[0].Summary {
		t.Fatalf("user_message = %q, want summary %q", items[0].UserMessage, items[0].Summary)
	}
	if items[0].InternalReason != items[0].Code {
		t.Fatalf("internal_reason = %q, want code %q", items[0].InternalReason, items[0].Code)
	}
}

func TestDiagnosticsIssueGroupsExposeUserAndInternalFields(t *testing.T) {
	t.Parallel()

	items := nonNilIssues([]health.DiagnosticIssue{{
		Code:        "third_party.credential_invalid",
		Severity:    "error",
		Summary:     "三方账号凭据失效",
		Remediation: "请重新扫码登录。",
	}})

	if len(items) != 1 {
		t.Fatalf("nonNilIssues returned %d items, want 1", len(items))
	}
	if items[0].UserMessage == "" || items[0].InternalReason == "" {
		t.Fatalf("diagnostic issue must expose user and internal fields: %#v", items[0])
	}
}
