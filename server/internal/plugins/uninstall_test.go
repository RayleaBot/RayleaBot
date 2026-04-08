package plugins

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/schema"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

func TestUninstallServiceInvokesAfterSuccessCallback(t *testing.T) {
	t.Parallel()

	registry := tasks.NewRegistry()
	repoRoot := t.TempDir()
	examplesRoot := filepath.Join(repoRoot, "examples", "plugins")
	installedRoot := filepath.Join(repoRoot, "plugins", "installed")
	if err := os.MkdirAll(examplesRoot, 0o755); err != nil {
		t.Fatalf("create examples root: %v", err)
	}
	pluginDir := writeInstallSourcePlugin(t, filepath.Join(installedRoot, "weather-remove"), "weather-remove", "nodejs", "index.js")
	if pluginDir == "" {
		t.Fatal("expected plugin install source directory")
	}

	validator, err := schema.Compile(filepath.Join("..", "..", "..", "contracts", "plugin-info.schema.json"))
	if err != nil {
		t.Fatalf("compile plugin-info schema: %v", err)
	}
	catalog := NewCatalog([]Snapshot{{
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
		[]ScanRoot{
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
	service.SetAfterSuccess(func(pluginID string) {
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
