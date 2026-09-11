package system

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/deps"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/health"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	"github.com/RayleaBot/RayleaBot/server/internal/render"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
	"github.com/RayleaBot/RayleaBot/server/tests/testutil"
)

func TestChromiumTaskProgressSummarizesSourceProbe(t *testing.T) {
	percent, summary := managedRuntimeTaskProgress(1, 0, deps.PrepareProgress{
		Kind:     "chromium",
		Label:    deps.ManagedResourceLabel("chromium"),
		Stage:    "probe",
		Status:   "running",
		Progress: 0,
	})

	if percent != 0 {
		t.Fatalf("unexpected probe percent: got %d want 0", percent)
	}
	if summary != "正在测试 图片渲染 Chromium 下载来源" {
		t.Fatalf("unexpected probe summary: %q", summary)
	}
}

func TestRuntimeBootstrapRefreshesChromiumDiagnostics(t *testing.T) {
	t.Parallel()
	repoRoot := t.TempDir()
	testutil.WritePlatformDepsManifest(t, repoRoot)
	platform := deps.CurrentPlatform()
	store, err := storage.Open(filepath.Join(repoRoot, "state.db"))
	if err != nil {
		t.Fatalf("open sqlite store: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	renderer, err := render.NewService(render.Options{
		RepoRoot:   repoRoot,
		OutputRoot: filepath.Join(repoRoot, "render-out"),
		Store:      store,
		InspectRuntime: func(string) (*deps.BootstrapInspection, error) {
			return nil, errors.New("fixture browser is not prepared")
		},
	})
	if err != nil {
		t.Fatalf("create render service: %v", err)
	}
	t.Cleanup(func() {
		_ = renderer.Close()
	})
	// Exercise preparation from an unavailable browser even on developer machines
	// where construction can discover a system installation.
	renderer.RefreshBrowserPath("")

	prepare := func(_ context.Context, _ string, kind string, progress deps.PrepareProgressReporter) (*deps.PrepareReport, error) {
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
		testutil.WritePreparedRuntime(t, repoRoot, "chromium-"+platform, "152.0.7977.42", "chrome-win64", "chrome.exe")
		return &deps.PrepareReport{
			Kind:               kind,
			ArchivePath:        filepath.Join(repoRoot, "cache", "downloads", "runtime", "chromium-"+platform+"-152.0.7977.42.zip"),
			StoreRoot:          filepath.Join(repoRoot, ".deps", "store", "chromium-"+platform, "152.0.7977.42"),
			UsedPreparedStore:  false,
			UsedCachedArchive:  false,
			PreparedEntrypoint: filepath.Join(repoRoot, ".deps", "store", "chromium-"+platform, "152.0.7977.42", "chrome-win64", "chrome.exe"),
		}, nil
	}

	registry := tasks.NewRegistry()
	executor := tasks.NewExecutor(registry, 2*time.Second)
	t.Cleanup(func() {
		if err := executor.Close(); err != nil {
			t.Error(err)
		}
	})
	service, err := New(Deps{CurrentConfig: func() config.Config { return config.Config{} }, CurrentSummary: func() config.Summary { return config.Summary{} }, Plugins: plugincatalog.New(nil), RepoRoot: repoRoot, Renderer: renderer, TaskExecutor: executor, PrepareRuntime: prepare})
	if err != nil {
		t.Fatal(err)
	}

	if !containsIssueCode(renderer.Diagnostics(), "platform.resource_missing") {
		t.Fatalf("expected pre-bootstrap render diagnostics to warn about missing chromium")
	}

	taskID, err := service.SubmitRuntimeBootstrapTask([]string{"chromium"})
	if err != nil {
		t.Fatalf("submit runtime bootstrap task: %v", err)
	}
	testutil.WaitTask(t, registry, taskID, tasks.StatusSucceeded)

	if containsIssueCode(renderer.Diagnostics(), "platform.resource_missing") {
		t.Fatalf("expected runtime bootstrap to refresh chromium diagnostics")
	}
}

func containsIssueCode(issues []health.DiagnosticIssue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}
