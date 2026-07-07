package system

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/deps"
	"github.com/RayleaBot/RayleaBot/server/internal/health"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
	"github.com/RayleaBot/RayleaBot/server/tests/testutil"
)

func TestManagedRuntimeTaskProgressSummarizesSourceProbe(t *testing.T) {
	percent, summary := managedRuntimeTaskProgress(1, 0, deps.PrepareProgress{
		Kind:     "nodejs-runtime",
		Label:    deps.ManagedResourceLabel("nodejs-runtime"),
		Stage:    "probe",
		Status:   "running",
		Progress: 0,
	})

	if percent != 0 {
		t.Fatalf("unexpected probe percent: got %d want 0", percent)
	}
	if summary != "正在测试 Node.js / npm 环境下载来源" {
		t.Fatalf("unexpected probe summary: %q", summary)
	}
}

func TestRuntimeBootstrapRefreshesChromiumDiagnostics(t *testing.T) {
	repoRoot := t.TempDir()
	t.Cleanup(deps.SetSystemChromiumFinderForTest(func(context.Context) (string, error) {
		return "", errors.New("system chromium disabled for test")
	}))
	testutil.WritePlatformDepsManifest(t, repoRoot)
	platform := deps.CurrentPlatform()
	store, err := storage.Open(filepath.Join(repoRoot, "state.db"))
	if err != nil {
		t.Fatalf("open sqlite store: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	renderer, err := renderservice.NewService(renderservice.Options{
		RepoRoot:   repoRoot,
		OutputRoot: filepath.Join(repoRoot, "render-out"),
		Store:      store,
	})
	if err != nil {
		t.Fatalf("create render service: %v", err)
	}
	t.Cleanup(func() {
		_ = renderer.Close()
	})

	application := newTaskOnlyApp(t, repoRoot)
	application.pluginStack.renderer = renderer
	application.services.system.renderer = renderer

	original := prepareManagedRuntimeWithProgress
	t.Cleanup(func() {
		prepareManagedRuntimeWithProgress = original
	})
	prepareManagedRuntimeWithProgress = func(_ context.Context, _ string, kind string, progress deps.PrepareProgressReporter) (*managedRuntimePrepareReport, error) {
		if progress != nil {
			progress(deps.PrepareProgress{
				Kind:     kind,
				Label:    deps.ManagedResourceLabel(kind),
				Stage:    "complete",
				Status:   "succeeded",
				Progress: 100,
				Summary:  deps.ManagedResourceLabel(kind) + "已准备完成",
			})
		}
		testutil.WritePreparedRuntime(t, repoRoot, "chromium-"+platform, "147.0.7727.24", "chrome-win64", "chrome.exe")
		return &managedRuntimePrepareReport{
			Kind:               kind,
			ArchivePath:        filepath.Join(repoRoot, "cache", "downloads", "runtime", "chromium-"+platform+"-147.0.7727.24.zip"),
			StoreRoot:          filepath.Join(repoRoot, ".deps", "store", "chromium-"+platform, "147.0.7727.24"),
			UsedPreparedStore:  false,
			UsedCachedArchive:  false,
			PreparedEntrypoint: filepath.Join(repoRoot, ".deps", "store", "chromium-"+platform, "147.0.7727.24", "chrome-win64", "chrome.exe"),
		}, nil
	}

	if !containsIssueCode(renderer.Diagnostics(), "platform.resource_missing") {
		t.Fatalf("expected pre-bootstrap render diagnostics to warn about missing chromium")
	}

	taskID, err := application.services.system.SubmitRuntimeBootstrapTask([]string{"chromium"})
	if err != nil {
		t.Fatalf("submit runtime bootstrap task: %v", err)
	}
	testutil.WaitTask(t, application.platform.Tasks, taskID, tasks.StatusSucceeded)

	if containsIssueCode(renderer.Diagnostics(), "platform.resource_missing") {
		t.Fatalf("expected runtime bootstrap to refresh chromium diagnostics")
	}
}

func newTaskOnlyApp(t *testing.T, repoRoot string) *App {
	t.Helper()
	registry := tasks.NewRegistry()
	executor := tasks.NewExecutor(registry, 2*time.Second)
	t.Cleanup(func() {
		_ = executor.Close()
	})
	application := newTestAppState(config.Config{}, nil)
	application.state.repoRoot = repoRoot
	application.state.startedAt = time.Now()
	application.platform.Tasks = registry
	application.platform.taskExecutor = executor
	application.pluginStack.Plugins = plugincatalog.New(nil)
	application.setTestSystem(registry, executor, nil, nil)
	return application
}

func containsIssueCode(issues []health.DiagnosticIssue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}
