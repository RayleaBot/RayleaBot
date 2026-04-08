package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/deps"
)

func TestAutoPrepareRuntimeEnvironmentsPreparesStartupManagedRuntimes(t *testing.T) {
	originalInspect := inspectStartupRuntime
	originalPrepare := prepareStartupRuntime
	t.Cleanup(func() {
		inspectStartupRuntime = originalInspect
		prepareStartupRuntime = originalPrepare
	})

	preparedKinds := make([]string, 0, 2)
	inspectStartupRuntime = func(_ string, kind string) (*deps.BootstrapInspection, error) {
		return &deps.BootstrapInspection{
			Kind:                 kind,
			MetadataComplete:     true,
			CachedArchivePresent: kind == "python-runtime",
			PreparedStorePresent: false,
		}, nil
	}
	prepareStartupRuntime = func(_ context.Context, _ string, kind string) (*deps.PrepareReport, error) {
		preparedKinds = append(preparedKinds, kind)
		return &deps.PrepareReport{
			Kind:              kind,
			UsedCachedArchive: true,
		}, nil
	}

	application := &App{
		appCore: appCore{repoRoot: t.TempDir()},
	}

	application.autoPrepareRuntimeEnvironments(context.Background())

	if !slices.Equal(preparedKinds, []string{"python-runtime", "nodejs-runtime"}) {
		t.Fatalf("unexpected prepared kinds: %#v", preparedKinds)
	}
	pythonState, ok := application.startupRuntimeState("python-runtime")
	if !ok || pythonState.Phase != startupRuntimeReady {
		t.Fatalf("python runtime state = %#v, want ready", pythonState)
	}
	nodeState, ok := application.startupRuntimeState("nodejs-runtime")
	if !ok || nodeState.Phase != startupRuntimeReady {
		t.Fatalf("nodejs runtime state = %#v, want ready", nodeState)
	}
}

func TestAutoPrepareRuntimeEnvironmentsWaitsForPrepareResult(t *testing.T) {
	originalInspect := inspectStartupRuntime
	originalPrepare := prepareStartupRuntime
	t.Cleanup(func() {
		inspectStartupRuntime = originalInspect
		prepareStartupRuntime = originalPrepare
	})

	inspectStartupRuntime = func(_ string, kind string) (*deps.BootstrapInspection, error) {
		if kind == "nodejs-runtime" {
			return &deps.BootstrapInspection{
				Kind:                 kind,
				MetadataComplete:     true,
				PreparedStorePresent: true,
			}, nil
		}
		return &deps.BootstrapInspection{
			Kind:                 kind,
			MetadataComplete:     true,
			PreparedStorePresent: false,
		}, nil
	}

	releasePrepare := make(chan struct{})
	prepareReturned := make(chan struct{})
	prepareStartupRuntime = func(_ context.Context, _ string, kind string) (*deps.PrepareReport, error) {
		if kind != "python-runtime" {
			t.Fatalf("unexpected prepare kind %q", kind)
		}
		<-releasePrepare
		close(prepareReturned)
		return &deps.PrepareReport{Kind: kind}, nil
	}

	application := &App{appCore: appCore{repoRoot: t.TempDir()}}

	finished := make(chan struct{})
	go func() {
		application.autoPrepareRuntimeEnvironments(context.Background())
		close(finished)
	}()

	select {
	case <-finished:
		t.Fatal("startup runtime prepare should wait for prepare result")
	case <-time.After(20 * time.Millisecond):
	}

	close(releasePrepare)

	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("startup runtime prepare did not finish after prepare returned")
	}

	select {
	case <-prepareReturned:
	default:
		t.Fatal("prepare function should complete before startup runtime prepare returns")
	}
}

func TestManagedRuntimeDiagnosticsUsesStartupFailureReason(t *testing.T) {
	repoRoot := t.TempDir()
	writeStartupRuntimeManifest(t, repoRoot)
	writeStartupPreparedRuntime(t, repoRoot, "node-test", "24.14.0", "node", "node.exe")
	writeStartupPreparedRuntime(t, repoRoot, "node-test", "24.14.0", "node", "npm.cmd")

	application := &App{appCore: appCore{repoRoot: repoRoot}}
	issue := startupRuntimeFailureIssue("python-runtime", &deps.BootstrapError{
		Kind:        "python-runtime",
		Stage:       "download",
		Remediation: "请联网准备 Python 运行环境。",
		Message:     "Python 运行环境归档下载失败",
		Err:         errors.New("offline"),
	})
	application.setStartupRuntimeState("python-runtime", startupRuntimeFailed, &issue)

	application.setStartupRuntimeState("nodejs-runtime", startupRuntimeReady, nil)

	issues := application.managedRuntimeDiagnostics(nil)
	if len(issues) != 1 {
		t.Fatalf("managedRuntimeDiagnostics returned %d issues, want 1", len(issues))
	}
	if issues[0].Code != "platform.resource_missing" {
		t.Fatalf("unexpected issue code: %#v", issues[0])
	}
	if issues[0].Summary != "Python 运行环境归档下载失败。" {
		t.Fatalf("unexpected issue summary: %#v", issues[0])
	}
	if issues[0].Remediation != "请联网准备 Python 运行环境。" {
		t.Fatalf("unexpected issue remediation: %#v", issues[0])
	}
}

func TestManagedRuntimeDiagnosticsDoesNotReportPendingStartupRuntime(t *testing.T) {
	repoRoot := t.TempDir()
	writeStartupRuntimeManifest(t, repoRoot)
	writeStartupPreparedRuntime(t, repoRoot, "node-test", "24.14.0", "node", "node.exe")
	writeStartupPreparedRuntime(t, repoRoot, "node-test", "24.14.0", "node", "npm.cmd")

	application := &App{appCore: appCore{repoRoot: repoRoot}}
	application.setStartupRuntimeState("python-runtime", startupRuntimePending, nil)
	application.setStartupRuntimeState("nodejs-runtime", startupRuntimeReady, nil)

	if issues := application.managedRuntimeDiagnostics(nil); len(issues) != 0 {
		t.Fatalf("managedRuntimeDiagnostics returned issues during pending prepare: %#v", issues)
	}
}

func TestManagedRuntimeDiagnosticsStillChecksStartupManagedRuntimesWithoutPluginDemand(t *testing.T) {
	repoRoot := t.TempDir()
	writeStartupRuntimeManifest(t, repoRoot)

	application := &App{appCore: appCore{repoRoot: repoRoot}}

	issues := application.managedRuntimeDiagnostics(nil)
	if len(issues) != 2 {
		t.Fatalf("managedRuntimeDiagnostics returned %d issues, want 2", len(issues))
	}
}

func writeStartupRuntimeManifest(t *testing.T, repoRoot string) {
	t.Helper()

	manifestPath := filepath.Join(repoRoot, ".deps", "manifest.json")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatalf("mkdir deps root: %v", err)
	}
	manifest := `{
  "manifest_version": 3,
  "resources": [
    {
      "id": "python-test",
      "kind": "python-runtime",
      "version": "3.12.13",
      "platform": "` + deps.CurrentPlatform() + `",
      "sources": [
        {
          "url": "https://example.invalid/python.tar.gz",
          "kind": "upstream"
        }
      ],
      "sha256": "10b7a95b928e551fc78cac665999e1ae1f08fb738b255adb0a8d3b9c2824a9c0",
      "archive_format": "tar.gz",
      "entrypoints": {
        "python": ["python/python.exe"],
        "pip": ["python/Scripts/pip.exe"]
      }
    },
    {
      "id": "node-test",
      "kind": "nodejs-runtime",
      "version": "24.14.0",
      "platform": "` + deps.CurrentPlatform() + `",
      "sources": [
        {
          "url": "https://example.invalid/node.zip",
          "kind": "upstream"
        }
      ],
      "sha256": "313fa40c0d7b18575821de8cb17483031fe07d95de5994f6f435f3b345f85c66",
      "archive_format": "zip",
      "entrypoints": {
        "node": ["node/node.exe"],
        "npm": ["node/npm.cmd"]
      }
    }
  ]
}`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatalf("write deps manifest: %v", err)
	}
}

func writeStartupPreparedRuntime(t *testing.T, repoRoot, id, version string, segments ...string) {
	t.Helper()

	target := filepath.Join(append([]string{repoRoot, ".deps", "store", id, version}, segments...)...)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir runtime target: %v", err)
	}
	if err := os.WriteFile(target, []byte("ok"), 0o755); err != nil {
		t.Fatalf("write runtime target: %v", err)
	}
}
