package lifecycle

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

func TestOperationGateScopesNestedOwnershipAndReleasesEntries(t *testing.T) {
	t.Parallel()
	gate := NewOperationGate()
	held, release, err := gate.Acquire(t.Context(), "weather")
	if err != nil {
		t.Fatal(err)
	}
	_, nested, err := gate.Acquire(held, "weather")
	if err != nil {
		t.Fatal(err)
	}
	nested()
	_, other, err := gate.Acquire(t.Context(), "other")
	if err != nil {
		t.Fatal(err)
	}
	other()
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, _, err := gate.Acquire(cancelled, "weather"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled acquisition: %v", err)
	}
	release()
	release()
	_, acquired, err := gate.Acquire(t.Context(), "weather")
	if err != nil {
		t.Fatal(err)
	}
	expired, timeout := context.WithTimeout(held, 20*time.Millisecond)
	defer timeout()
	if _, _, err := gate.Acquire(expired, "weather"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expired token bypassed current owner: %v", err)
	}
	acquired()
	gate.mu.Lock()
	remaining := len(gate.entries)
	gate.mu.Unlock()
	if remaining != 0 {
		t.Fatalf("released plugin gates leaked: %d", remaining)
	}
}

func TestInstallTransactionExcludesRuntimeMutationsAndAllowsSynchronousCallbacks(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	oldRoot := writeInstallSourcePlugin(t, filepath.Join(root, "plugins", "installed", "weather"), "weather")
	source := writeInstallSourcePlugin(t, filepath.Join(t.TempDir(), "replacement"), "weather")
	gate := NewOperationGate()
	entries := []plugins.Snapshot{{PluginID: "weather", Valid: true, RegistrationState: "installed", DesiredState: "disabled", SourceRoot: "plugins/installed", PackageRootPath: oldRoot, ManifestPath: filepath.Join(oldRoot, "info.json")}}
	registry := tasks.NewRegistry()
	service, _ := newInstallTestService(t, root, registry, entries, &stubInstallRepository{}, installerDeps{options: InstallOptions{Operations: gate}})
	t.Cleanup(func() { _ = service.Close() })
	cat := catalog.New(entries)
	service.catalog = cat
	controller := newTestController(t, Deps{Plugins: cat, Operations: gate})
	service.beforeReplace = controller.StopAndResetPluginWithContext
	entered, resume := make(chan struct{}), make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-resume:
		default:
			close(resume)
		}
	})
	service.afterSuccess = func(ctx context.Context, id string) error {
		if err := controller.StopAndResetPluginWithContext(ctx, id); err != nil {
			return err
		}
		close(entered)
		<-resume
		return nil
	}
	taskID, err := acceptInspected(t, service, plugins.InstallRequest{SourceType: "local_directory", Source: source, ReplaceExisting: true})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("install callbacks could not reuse transaction ownership")
	}
	for _, mutate := range []func(context.Context, string) (plugins.Snapshot, error){controller.Enable, controller.Disable, controller.Reload, controller.RecoverFromDeadLetter} {
		ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
		_, err := mutate(ctx, "weather")
		cancel()
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("runtime mutation crossed installation transaction: %v", err)
		}
	}
	close(resume)
	result := waitForTaskCompletion(t, registry, taskID)
	if result.Status != tasks.StatusSucceeded {
		t.Fatalf("installation result: %#v", result)
	}
	if snapshot, _ := cat.Get("weather"); snapshot.DesiredState != "disabled" {
		t.Fatalf("cancelled enable changed installed intent: %#v", snapshot)
	}
}

func TestUninstallTransactionExcludesRuntimeMutationUntilFilesAndCatalogCommit(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	installedRoot := filepath.Join(root, "plugins", "installed")
	writeInstallSourcePlugin(t, filepath.Join(installedRoot, "weather"), "weather")
	gate := NewOperationGate()
	cat := catalog.New([]plugins.Snapshot{{PluginID: "weather", Valid: true, RegistrationState: "installed", DesiredState: "disabled"}})
	controller := newTestController(t, Deps{Plugins: cat, Operations: gate})
	registry := tasks.NewRegistry()
	validator, err := config.Compile(filepath.Join("..", "..", "..", "..", "contracts", "plugin-info.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewUninstallService(nil, registry, cat, &stubInstallRepository{}, validator, root, []catalog.ScanRoot{{Label: "plugins/installed", Path: installedRoot}}, UninstallOptions{Operations: gate, StopPlugin: controller.StopAndResetPluginWithContext})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	entered, resume := make(chan struct{}), make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-resume:
		default:
			close(resume)
		}
	})
	service.deps.removeAll = func(path string) error { close(entered); <-resume; return os.RemoveAll(path) }
	taskID, err := service.Accept(t.Context(), "weather")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("uninstall did not reach file transaction")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	_, err = controller.Enable(ctx, "weather")
	cancel()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("enable crossed uninstall: %v", err)
	}
	close(resume)
	result := waitForTaskCompletion(t, registry, taskID)
	if result.Status != tasks.StatusSucceeded {
		t.Fatalf("uninstall result: %#v", result)
	}
	if _, exists := cat.Get("weather"); exists {
		t.Fatal("removed plugin remained in catalog")
	}
}
