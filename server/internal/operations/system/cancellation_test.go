package system

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/deps"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
	"github.com/RayleaBot/RayleaBot/server/tests/testutil"
)

func TestAutomaticPreparationCancellationDoesNotPublishFailure(t *testing.T) {
	for _, kind := range []string{"chromium", "ffmpeg"} {
		t.Run(kind, func(t *testing.T) {
			var logs bytes.Buffer
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			s, err := New(Deps{
				CurrentConfig:  func() config.Config { return config.Config{} },
				CurrentSummary: func() config.Summary { return config.Summary{} }, Plugins: plugincatalog.New(nil),
				RepoRoot: t.TempDir(), Logger: slog.New(slog.NewJSONHandler(&logs, nil)),
				InspectRuntime: func(_ string, k string) (*deps.BootstrapInspection, error) {
					return &deps.BootstrapInspection{Kind: k, MetadataComplete: true}, nil
				},
				PrepareRuntime: func(_ context.Context, _ string, k string, progress deps.PrepareProgressReporter) (*deps.PrepareReport, error) {
					if k != kind {
						return &deps.PrepareReport{Kind: k}, nil
					}
					cancel()
					progress(deps.PrepareProgress{Kind: k, Status: "failed", Error: context.Canceled.Error()})
					return nil, fmt.Errorf("prepare: %w", context.Canceled)
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			s.AutoPrepareRuntimeEnvironments(ctx)
			state, _ := s.startupRuntimeState(kind)
			if state.Phase == StartupRuntimePhaseFailed || state.Issue != nil {
				t.Fatalf("state = %#v", state)
			}
			if strings.Contains(logs.String(), `"level":"WARN"`) {
				t.Fatalf("cancellation logged as failure: %s", logs.String())
			}
		})
	}
}

func TestBackupWrapperPreservesCancellation(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "user.yaml")
	if err := os.WriteFile(configPath, []byte("schema_version: \"4\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	store, err := storage.Open(filepath.Join(root, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	registry := tasks.NewRegistry()
	executor := tasks.NewExecutor(registry, time.Minute)
	defer func() { _ = executor.Close() }()
	s, err := New(Deps{
		CurrentConfig:  func() config.Config { return config.Config{} },
		CurrentSummary: func() config.Summary { return config.Summary{ConfigPath: configPath} },
		Plugins:        plugincatalog.New(nil), RepoRoot: root, Storage: store,
		ResolveDatabasePath: func(string, string) (string, error) { return store.Path, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	id, err := executor.Submit("backup.create", "fixture", func(ctx context.Context, progress tasks.ProgressReporter) (*tasks.ResultSummary, error) {
		ctx, cancel := context.WithCancel(ctx)
		cancel()
		_, err := s.createBackupArchive(ctx, progress)
		return nil, err
	})
	if err != nil {
		t.Fatal(err)
	}
	testutil.WaitTask(t, registry, id, tasks.StatusCancelled)
	snapshot, _ := registry.Get(id)
	if snapshot.Error != nil {
		t.Fatalf("backup cancellation became failure: %#v", snapshot)
	}
}

func TestDiagnosticsExcludesInterruptedAndCancelledTasksFromFailures(t *testing.T) {
	registry := tasks.NewRegistry()
	executor := tasks.NewExecutor(registry, time.Minute)
	defer func() { _ = executor.Close() }()
	for _, status := range []tasks.Status{tasks.StatusPending, tasks.StatusRunning, tasks.StatusFailed, tasks.StatusInterrupted, tasks.StatusCancelled} {
		id, err := registry.Create("fixture", "fixture")
		if err != nil {
			t.Fatal(err)
		}
		registry.Update(id, tasks.Update{Status: &status})
	}
	s := &Service{taskExecutor: executor}
	got := s.diagnosticsTasks()
	if got.Pending != 1 || got.Running != 1 || got.Failed != 1 || got.Interrupted != 1 {
		t.Fatalf("summary = %#v", got)
	}
}
