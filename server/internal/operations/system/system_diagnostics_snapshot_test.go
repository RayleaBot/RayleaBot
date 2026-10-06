package system

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/deps"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/health"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
)

func TestOptionalFFmpegKeepsManualPreparationAction(t *testing.T) {
	root := t.TempDir()
	resource := deps.Resource{
		ID: "ffmpeg-test", Kind: "ffmpeg", Platform: deps.CurrentPlatform(), Version: "1",
		Sources: []deps.ResourceSource{{URL: "https://example.invalid/ffmpeg.zip", Kind: "upstream"}},
		SHA256:  strings.Repeat("a", 64), ArchiveFormat: "zip",
		Entrypoints: map[string][]string{"ffmpeg": {"ffmpeg"}, "ffprobe": {"ffprobe"}},
	}
	payload, err := json.Marshal(deps.Manifest{ManifestVersion: deps.ManifestVersion, Resources: []deps.Resource{resource}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".deps"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".deps", "manifest.json"), payload, 0o600); err != nil {
		t.Fatal(err)
	}
	service, err := New(Deps{
		CurrentConfig:  func() config.Config { return config.Config{} },
		CurrentSummary: func() config.Summary { return config.Summary{} },
		Plugins:        plugincatalog.New(nil), RepoRoot: root,
	})
	if err != nil {
		t.Fatal(err)
	}
	service.resetStartupRuntimeStates(nil)
	_, issues := service.diagnosticsDependencies()
	for _, issue := range issues {
		if issue.Code == "dependency.ffmpeg" && len(issue.RuntimeResources) == 1 && issue.RuntimeResources[0] == "ffmpeg" {
			return
		}
	}
	t.Fatalf("optional missing media tools lost their preparation action: %#v", issues)
}

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

func TestDiagnosticsUsesOneFFmpegIssue(t *testing.T) {
	t.Parallel()
	for _, bootstrapped := range []bool{true, false} {
		name := "readiness issue"
		if !bootstrapped {
			name = "dependency fallback before setup"
		}
		t.Run(name, func(t *testing.T) {
			service, err := New(Deps{
				CurrentConfig:  func() config.Config { return config.Config{} },
				CurrentSummary: func() config.Summary { return config.Summary{} },
				Plugins:        plugincatalog.New(nil),
				RepoRoot:       t.TempDir(),
				Auth:           readinessAuthState(bootstrapped),
				Storage:        openReadinessStore(t),
			})
			if err != nil {
				t.Fatal(err)
			}
			startup := startupFailureIssue("ffmpeg", errors.New("fixture failure"))
			service.setStartupRuntimeState("ffmpeg", StartupRuntimePhaseFailed, &startup)
			snapshot := service.DiagnosticsSnapshot(context.Background())
			count := 0
			for _, issue := range snapshot.Issues {
				if !containsRuntimeKind(issue.RuntimeResources, "ffmpeg") {
					continue
				}
				count++
				if bootstrapped && (issue.Code != startup.Code || issue.Summary != startup.Summary || issue.Remediation != startup.Remediation) {
					t.Fatalf("diagnostics discarded the preparation failure: %#v", issue)
				}
				if !bootstrapped && issue.Code != "dependency.ffmpeg" {
					t.Fatalf("missing dependency fallback: %#v", issue)
				}
			}
			if count != 1 {
				t.Fatalf("FFmpeg issue count = %d, want 1: %#v", count, snapshot.Issues)
			}
		})
	}
}
