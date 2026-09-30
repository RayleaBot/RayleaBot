package system

import (
	"errors"
	"strings"
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

func TestUnavailableDependencyIssuesNameTheirRuntimeResource(t *testing.T) {
	t.Parallel()
	// A root without .deps has no FFmpeg; Chromium may still resolve to a browser installed on the machine.
	s := &Service{repoRoot: t.TempDir()}
	_, issues := s.diagnosticsDependencies()
	sawFFmpeg := false
	for _, issue := range issues {
		kind := strings.TrimPrefix(issue.Code, "dependency.")
		if len(issue.RuntimeResources) != 1 || issue.RuntimeResources[0] != kind {
			t.Fatalf("issue %s names runtime resources %#v", issue.Code, issue.RuntimeResources)
		}
		sawFFmpeg = sawFFmpeg || kind == "ffmpeg"
	}
	if !sawFFmpeg {
		t.Fatalf("dependency issues = %#v", issues)
	}
}

func TestDiagnosticProjectionsExposeUserAndInternalFields(t *testing.T) {
	t.Parallel()
	for name, project := range map[string]func([]health.DiagnosticIssue) []health.DiagnosticIssue{
		"deduplicated": dedupeDiagnosticIssues, "non-nil": nonNilIssues,
	} {
		t.Run(name, func(t *testing.T) {
			source := health.DiagnosticIssue{Code: "render.chromium_missing", Severity: "warning", Summary: "Chromium 不可用", Remediation: "请准备 Chromium 运行环境。"}
			items := project([]health.DiagnosticIssue{source})
			if len(items) != 1 || items[0].UserMessage != source.Summary || items[0].InternalReason != source.Code {
				t.Fatalf("diagnostic projection = %#v", items)
			}
		})
	}
}
