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
	"github.com/RayleaBot/RayleaBot/server/internal/platform/deps"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	"github.com/RayleaBot/RayleaBot/server/tests/testutil"
)

func TestAutoPrepareRuntimeEnvironmentsPreparesManagedRuntimes(t *testing.T) {
	t.Parallel()

	preparedKinds := []string{}
	inspect := func(_ string, kind string) (*deps.BootstrapInspection, error) {
		return &deps.BootstrapInspection{Kind: kind, MetadataComplete: true}, nil
	}
	prepare := func(_ context.Context, _ string, kind string, _ deps.PrepareProgressReporter) (*deps.PrepareReport, error) {
		preparedKinds = append(preparedKinds, kind)
		return &deps.PrepareReport{Kind: kind}, nil
	}

	service, err := New(Deps{
		CurrentConfig: func() config.Config { return config.Config{} }, CurrentSummary: func() config.Summary { return config.Summary{} },
		Plugins: plugincatalog.New(nil), PluginRepository: &testutil.DesiredStateRecorder{}, RepoRoot: t.TempDir(), InspectRuntime: inspect, PrepareRuntime: prepare,
	})
	if err != nil {
		t.Fatal(err)
	}
	service.autoPrepareRuntimeEnvironments(context.Background())

	if !slices.Equal(preparedKinds, []string{"chromium", "ffmpeg"}) {
		t.Fatalf("prepared kinds = %#v, want Chromium and FFmpeg", preparedKinds)
	}
	state, ok := service.startupRuntimeState("chromium")
	if !ok || state.Phase != StartupRuntimePhaseReady {
		t.Fatalf("Chromium state = %#v, want ready", state)
	}
	state, ok = service.startupRuntimeState("ffmpeg")
	if !ok || state.Phase != StartupRuntimePhaseReady {
		t.Fatalf("FFmpeg state = %#v, want ready", state)
	}
}

func TestAutoPrepareRuntimeEnvironmentsWaitsForChromiumPrepare(t *testing.T) {
	t.Parallel()

	inspect := func(_ string, kind string) (*deps.BootstrapInspection, error) {
		return &deps.BootstrapInspection{Kind: kind, MetadataComplete: true}, nil
	}
	releasePrepare := make(chan struct{})
	startedPrepare := make(chan struct{})
	prepare := func(_ context.Context, _ string, kind string, _ deps.PrepareProgressReporter) (*deps.PrepareReport, error) {
		if kind == "ffmpeg" {
			return &deps.PrepareReport{Kind: kind}, nil
		}
		if kind != "chromium" {
			t.Fatalf("unexpected prepare kind %q", kind)
		}
		close(startedPrepare)
		<-releasePrepare
		return &deps.PrepareReport{Kind: kind}, nil
	}

	service, err := New(Deps{
		CurrentConfig: func() config.Config { return config.Config{} }, CurrentSummary: func() config.Summary { return config.Summary{} },
		Plugins: plugincatalog.New(nil), PluginRepository: &testutil.DesiredStateRecorder{}, RepoRoot: t.TempDir(), InspectRuntime: inspect, PrepareRuntime: prepare,
	})
	if err != nil {
		t.Fatal(err)
	}
	finished := make(chan struct{})
	go func() {
		service.autoPrepareRuntimeEnvironments(context.Background())
		close(finished)
	}()

	select {
	case <-startedPrepare:
	case <-time.After(time.Second):
		t.Fatal("Chromium preparation did not start")
	}
	select {
	case <-finished:
		t.Fatal("startup prepare returned before Chromium preparation completed")
	default:
	}
	close(releasePrepare)
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("startup prepare did not finish")
	}
}

func TestAutoPrepareRuntimeEnvironmentsLogsChromiumProgress(t *testing.T) {
	t.Parallel()

	inspect := func(_ string, kind string) (*deps.BootstrapInspection, error) {
		return &deps.BootstrapInspection{Kind: kind, MetadataComplete: true}, nil
	}
	repoRoot := t.TempDir()
	prepare := func(_ context.Context, _ string, kind string, progress deps.PrepareProgressReporter) (*deps.PrepareReport, error) {
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
	service, err := New(Deps{
		CurrentConfig: func() config.Config { return config.Config{} }, CurrentSummary: func() config.Summary { return config.Summary{} },
		Plugins: plugincatalog.New(nil), PluginRepository: &testutil.DesiredStateRecorder{}, RepoRoot: repoRoot, Logger: slog.New(slog.NewJSONHandler(&logs, nil)), InspectRuntime: inspect, PrepareRuntime: prepare,
	})
	if err != nil {
		t.Fatal(err)
	}
	service.autoPrepareRuntimeEnvironments(context.Background())

	logText := logs.String()
	if !strings.Contains(logText, `"resource_kind":"chromium"`) || !strings.Contains(logText, `"source_url":"https://example.invalid/chromium.zip"`) {
		t.Fatalf("Chromium progress was not logged: %s", logText)
	}
	if strings.Contains(logText, repoRoot) {
		t.Fatalf("progress log should use repo-relative paths: %s", logText)
	}
}

func TestStartupRequiredRuntimeKindsKeepsFFmpegWhenBrowserPathConfigured(t *testing.T) {
	t.Parallel()
	service, err := New(Deps{
		CurrentConfig: func() config.Config {
			return config.Config{Render: config.RenderConfig{BrowserPath: "configured-chromium"}}
		}, CurrentSummary: func() config.Summary { return config.Summary{} }, Plugins: plugincatalog.New(nil), PluginRepository: &testutil.DesiredStateRecorder{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := service.startupRequiredRuntimeKinds(); !slices.Equal(got, []string{"ffmpeg"}) {
		t.Fatalf("startupRequiredRuntimeKinds() = %#v, want FFmpeg", got)
	}
}
