package lifecycle

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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
	defer service.Close()

	called := make(chan string, 1)
	service.SetAfterSuccess(func(ctx context.Context, pluginID string) {
		if ctx == nil {
			t.Fatal("expected uninstall callback context")
		}
		called <- pluginID
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
