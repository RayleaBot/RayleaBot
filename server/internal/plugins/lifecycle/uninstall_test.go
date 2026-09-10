package lifecycle

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

func TestUninstallServiceRejectsFullQueueBeforeTaskCreation(t *testing.T) {
	registry := tasks.NewRegistry()
	repoRoot := t.TempDir()
	examplesRoot := filepath.Join(repoRoot, "examples", "plugins")
	installedRoot := filepath.Join(repoRoot, "plugins", "installed")
	if err := os.MkdirAll(examplesRoot, 0o755); err != nil {
		t.Fatalf("create examples root: %v", err)
	}
	writeInstallSourcePlugin(t, filepath.Join(installedRoot, "queued-plugin"), "queued-plugin")
	validator, err := config.Compile(filepath.Join("..", "..", "..", "..", "contracts", "plugin-info.schema.json"))
	if err != nil {
		t.Fatalf("compile plugin-info schema: %v", err)
	}
	service, err := NewUninstallService(nil, registry, newTestCatalog(nil), &stubInstallRepository{}, validator, repoRoot, []plugincatalog.ScanRoot{
		{Label: "examples/plugins", Path: examplesRoot},
		{Label: "plugins/installed", Path: installedRoot},
	}, nil)
	if err != nil {
		t.Fatalf("new uninstall service: %v", err)
	}
	started := make(chan struct{})
	release := make(chan struct{})
	var startedOnce sync.Once
	service.deps.removeAll = func(string) error {
		startedOnce.Do(func() { close(started) })
		<-release
		return context.Canceled
	}

	if _, err := service.Accept(context.Background(), "queued-plugin"); err != nil {
		t.Fatalf("submit running uninstall: %v", err)
	}
	<-started
	for index := 0; index < 32; index++ {
		if _, err := service.Accept(context.Background(), "queued-plugin"); err != nil {
			t.Fatalf("submit queued uninstall %d: %v", index, err)
		}
	}
	before := len(registry.List())
	if _, err := service.Accept(context.Background(), "queued-plugin"); !errors.Is(err, tasks.ErrQueueFull) {
		t.Fatalf("queue-full error = %v, want tasks.ErrQueueFull", err)
	}
	if after := len(registry.List()); after != before {
		t.Fatalf("queue-full uninstall created a task: before=%d after=%d", before, after)
	}
	close(release)
	if err := service.Close(); err != nil {
		t.Fatalf("close uninstall service: %v", err)
	}
}

func TestUninstallServiceInvokesAfterSuccessCallback(t *testing.T) {
	t.Parallel()

	registry := tasks.NewRegistry()
	repoRoot := t.TempDir()
	examplesRoot := filepath.Join(repoRoot, "examples", "plugins")
	installedRoot := filepath.Join(repoRoot, "plugins", "installed")
	if err := os.MkdirAll(examplesRoot, 0o755); err != nil {
		t.Fatalf("create examples root: %v", err)
	}
	pluginDir := writeInstallSourcePlugin(t, filepath.Join(installedRoot, "weather-remove"), "weather-remove")
	if pluginDir == "" {
		t.Fatal("expected plugin install source directory")
	}

	validator, err := config.Compile(filepath.Join("..", "..", "..", "..", "contracts", "plugin-info.schema.json"))
	if err != nil {
		t.Fatalf("compile plugin-info schema: %v", err)
	}
	catalog := newTestCatalog([]plugins.Snapshot{{
		PluginID:          "weather-remove",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "disabled",
		RuntimeState:      "stopped",
		DisplayState:      "discovered",
	}})
	repository := &stubInstallRepository{saved: map[string]string{"weather-remove": "disabled"}}
	service, err := NewUninstallService(
		nil,
		registry,
		catalog,
		repository,
		validator,
		repoRoot,
		[]plugincatalog.ScanRoot{
			{Label: "examples/plugins", Path: examplesRoot},
			{Label: "plugins/installed", Path: installedRoot},
		},
		nil,
	)
	if err != nil {
		t.Fatalf("NewUninstallService failed: %v", err)
	}
	defer func(release func() error) { _ = release() }(service.Close)

	called := make(chan string, 1)
	service.SetAfterSuccess(func(ctx context.Context, pluginID string) error {
		if ctx == nil {
			t.Fatal("expected uninstall callback context")
		}
		called <- pluginID
		return nil
	})

	taskID, err := service.Accept(context.Background(), "weather-remove")
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	snapshot := waitForTaskCompletion(t, registry, taskID)
	if snapshot.Status != tasks.StatusSucceeded {
		t.Fatalf("unexpected task status: got %q want %q", snapshot.Status, tasks.StatusSucceeded)
	}

	select {
	case pluginID := <-called:
		if pluginID != "weather-remove" {
			t.Fatalf("unexpected callback plugin id: got %q want weather-remove", pluginID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for uninstall after-success callback")
	}

	if _, err := os.Stat(filepath.Join(installedRoot, "weather-remove")); !os.IsNotExist(err) {
		t.Fatalf("expected installed plugin directory to be removed, got err=%v", err)
	}
}

func TestUninstallAggregatesIndependentCleanupFailuresAfterRemoval(t *testing.T) {
	t.Parallel()
	desiredErr := errors.New("test-secret-desired-state")
	metadataErr := errors.New("test-secret-metadata")
	callbackErr := errors.New("test-secret-templates")
	repoRoot := t.TempDir()
	registry := tasks.NewRegistry()
	installedRoot := filepath.Join(repoRoot, "plugins", "installed")
	writeInstallSourcePlugin(t, filepath.Join(installedRoot, "weather"), "weather")
	validator, err := config.Compile(filepath.Join("..", "..", "..", "..", "contracts", "plugin-info.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	repository := &failingUninstallRepository{desiredErr: desiredErr, metadataErr: metadataErr}
	service, err := NewUninstallService(nil, registry, newTestCatalog(nil), repository, validator, repoRoot,
		[]plugincatalog.ScanRoot{{Label: "plugins/installed", Path: installedRoot}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	service.SetAfterSuccess(func(context.Context, string) error { return callbackErr })
	// The error chain retains every cause, including independent cleanup work.
	err = service.runUninstall(uninstallJob{ctx: t.Context(), pluginID: "weather"})
	if !errors.Is(err, desiredErr) || !errors.Is(err, metadataErr) || !errors.Is(err, callbackErr) {
		t.Fatalf("cleanup causes lost: %v", err)
	}
	if _, err := os.Stat(filepath.Join(installedRoot, "weather")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("package not removed: %v", err)
	}
	// Retry an already absent package and expose safe, actionable task details.
	taskID, err := service.Accept(t.Context(), "weather")
	if err != nil {
		t.Fatal(err)
	}
	assertOperationFailure(t, waitForTaskCompletion(t, registry, taskID), "committed", []string{"desired_state", "metadata", "finalize"})
}

func TestUninstallStopsBeforeDestructiveWorkWhenRuntimeStopFails(t *testing.T) {
	t.Parallel()
	repoRoot := t.TempDir()
	installedRoot := filepath.Join(repoRoot, "plugins", "installed")
	writeInstallSourcePlugin(t, filepath.Join(installedRoot, "weather"), "weather")
	registry := tasks.NewRegistry()
	repository := &stubInstallRepository{}
	service, err := NewUninstallService(nil, registry, newTestCatalog(nil), repository, nil, repoRoot,
		[]plugincatalog.ScanRoot{{Label: "plugins/installed", Path: installedRoot}},
		func(context.Context, string) error { return errors.New("test-secret-process-stop") })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	taskID, err := service.Accept(t.Context(), "weather")
	if err != nil {
		t.Fatal(err)
	}
	assertOperationFailure(t, waitForTaskCompletion(t, registry, taskID), "unchanged", []string{"stop"})
	if _, err := os.Stat(filepath.Join(installedRoot, "weather", "info.json")); err != nil {
		t.Fatalf("package was removed after failed stop: %v", err)
	}
	if repository.deletedPackage != "" {
		t.Fatal("metadata was removed after failed stop")
	}
}

type failingUninstallRepository struct {
	stubInstallRepository
	desiredErr  error
	metadataErr error
}

func TestUninstallRejectsInvalidIdentifierBeforeCreatingTask(t *testing.T) {
	t.Parallel()
	registry := tasks.NewRegistry()
	repoRoot := t.TempDir()
	service, err := NewUninstallService(nil, registry, newTestCatalog(nil), nil, nil, repoRoot,
		[]plugincatalog.ScanRoot{{Label: "plugins/installed", Path: filepath.Join(repoRoot, "plugins", "installed")}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	for _, id := range []string{"", ".", "..", "../outside", `..\outside`, "C:\\outside", "/outside", "UPPER", "trailing.", strings.Repeat("x", 65)} {
		if _, err := service.Accept(t.Context(), id); !errors.Is(err, plugins.ErrInvalidPluginID) {
			t.Errorf("Accept(%q) = %v, want invalid identifier", id, err)
		}
	}
	if len(registry.List()) != 0 {
		t.Fatal("invalid identifiers created uninstall tasks")
	}
}

func (r *failingUninstallRepository) DeleteDesiredState(context.Context, string) error {
	return r.desiredErr
}

func (r *failingUninstallRepository) DeletePackageMetadata(context.Context, string) error {
	return r.metadataErr
}
