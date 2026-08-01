package system

import (
	"bytes"
	"context"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/deps"
)

func TestAutoPrepareRuntimeEnvironmentsPreparesOnlyChromium(t *testing.T) {
	originalInspect := inspectStartupRuntime
	originalPrepare := prepareStartupRuntimeWithProgress
	t.Cleanup(func() {
		inspectStartupRuntime = originalInspect
		prepareStartupRuntimeWithProgress = originalPrepare
	})

	preparedKinds := []string{}
	inspectStartupRuntime = func(_ string, kind string) (*deps.BootstrapInspection, error) {
		return &deps.BootstrapInspection{Kind: kind, MetadataComplete: true}, nil
	}
	prepareStartupRuntimeWithProgress = func(_ context.Context, _ string, kind string, _ deps.PrepareProgressReporter) (*deps.PrepareReport, error) {
		preparedKinds = append(preparedKinds, kind)
		return &deps.PrepareReport{Kind: kind}, nil
	}

	application := newTestAppState(config.Config{}, nil)
	application.state.repoRoot = t.TempDir()
	application.setTestSystem(nil, nil, nil, nil)
	application.autoPrepareRuntimeEnvironments(context.Background())

	if !slices.Equal(preparedKinds, []string{"chromium"}) {
		t.Fatalf("prepared kinds = %#v, want Chromium only", preparedKinds)
	}
	state, ok := application.startupRuntimeState("chromium")
	if !ok || state.Phase != StartupRuntimePhaseReady {
		t.Fatalf("Chromium state = %#v, want ready", state)
	}
}

func TestAutoPrepareRuntimeEnvironmentsWaitsForChromiumPrepare(t *testing.T) {
	originalInspect := inspectStartupRuntime
	originalPrepare := prepareStartupRuntimeWithProgress
	t.Cleanup(func() {
		inspectStartupRuntime = originalInspect
		prepareStartupRuntimeWithProgress = originalPrepare
	})

	inspectStartupRuntime = func(_ string, kind string) (*deps.BootstrapInspection, error) {
		return &deps.BootstrapInspection{Kind: kind, MetadataComplete: true}, nil
	}
	releasePrepare := make(chan struct{})
	prepareStartupRuntimeWithProgress = func(_ context.Context, _ string, kind string, _ deps.PrepareProgressReporter) (*deps.PrepareReport, error) {
		if kind != "chromium" {
			t.Fatalf("unexpected prepare kind %q", kind)
		}
		<-releasePrepare
		return &deps.PrepareReport{Kind: kind}, nil
	}

	application := newTestAppState(config.Config{}, nil)
	application.state.repoRoot = t.TempDir()
	application.setTestSystem(nil, nil, nil, nil)
	finished := make(chan struct{})
	go func() {
		application.autoPrepareRuntimeEnvironments(context.Background())
		close(finished)
	}()

	select {
	case <-finished:
		t.Fatal("startup prepare returned before Chromium preparation completed")
	case <-time.After(20 * time.Millisecond):
	}
	close(releasePrepare)
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("startup prepare did not finish")
	}
}

func TestAutoPrepareRuntimeEnvironmentsLogsChromiumProgress(t *testing.T) {
	originalInspect := inspectStartupRuntime
	originalPrepare := prepareStartupRuntimeWithProgress
	t.Cleanup(func() {
		inspectStartupRuntime = originalInspect
		prepareStartupRuntimeWithProgress = originalPrepare
	})

	inspectStartupRuntime = func(_ string, kind string) (*deps.BootstrapInspection, error) {
		return &deps.BootstrapInspection{Kind: kind, MetadataComplete: true}, nil
	}
	repoRoot := t.TempDir()
	prepareStartupRuntimeWithProgress = func(_ context.Context, _ string, kind string, progress deps.PrepareProgressReporter) (*deps.PrepareReport, error) {
		progress(deps.PrepareProgress{
			Kind: kind, Label: "图片渲染 Chromium", ResourceID: "chromium-test", Version: "147.0.0",
			SourceLabel: "upstream", SourceURL: "https://example.invalid/chromium.zip",
			ArchivePath: filepath.Join(repoRoot, "cache", "downloads", "chromium.zip"),
			StoreRoot:   filepath.Join(repoRoot, ".deps", "store", "chromium-test", "147.0.0"),
			Stage:       "download", Status: "running", Progress: 25, Summary: "正在下载 Chromium",
		})
		return &deps.PrepareReport{Kind: kind}, nil
	}

	var logs bytes.Buffer
	application := newTestAppState(config.Config{}, slog.New(slog.NewJSONHandler(&logs, nil)))
	application.state.repoRoot = repoRoot
	application.setTestSystem(nil, nil, nil, nil)
	application.autoPrepareRuntimeEnvironments(context.Background())

	logText := logs.String()
	if !strings.Contains(logText, `"resource_kind":"chromium"`) || !strings.Contains(logText, `"source_url":"https://example.invalid/chromium.zip"`) {
		t.Fatalf("Chromium progress was not logged: %s", logText)
	}
	if strings.Contains(logText, repoRoot) {
		t.Fatalf("progress log should use repo-relative paths: %s", logText)
	}
}

func TestStartupRequiredRuntimeKindsEmptyWhenBrowserPathConfigured(t *testing.T) {
	application := newTestAppState(config.Config{Render: config.RenderConfig{BrowserPath: "C:\\chromium\\chrome.exe"}}, nil)
	application.setTestSystem(nil, nil, nil, nil)
	if got := application.services.system.startupRequiredRuntimeKinds(); len(got) != 0 {
		t.Fatalf("startupRequiredRuntimeKinds() = %#v, want no managed resources", got)
	}
}
