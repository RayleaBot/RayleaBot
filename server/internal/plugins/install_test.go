package plugins

import (
	"archive/zip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"rayleabot/server/internal/schema"
	"rayleabot/server/internal/tasks"
)

func TestInstallServiceInstallsLocalDirectoryAndRefreshesCatalog(t *testing.T) {
	t.Parallel()

	registry := tasks.NewRegistry()
	repoRoot := t.TempDir()
	installedRoot := filepath.Join(repoRoot, "plugins", "installed")
	sourceDir := writeInstallSourcePlugin(t, filepath.Join(t.TempDir(), "weather-src"), "weather", "nodejs", "index.js")
	service, catalog := newInstallTestService(t, repoRoot, registry, nil, installerDeps{})
	defer service.Close()

	taskID, err := service.Accept(context.Background(), InstallRequest{
		SourceType: "local_directory",
		Source:     sourceDir,
	})
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	snapshot := waitForTaskCompletion(t, registry, taskID)
	if snapshot.Status != tasks.StatusSucceeded {
		t.Fatalf("unexpected task status: got %q want %q", snapshot.Status, tasks.StatusSucceeded)
	}
	if snapshot.Progress != 100 {
		t.Fatalf("unexpected progress: got %d want 100", snapshot.Progress)
	}
	if snapshot.Result == nil || snapshot.Result.Summary == "" {
		t.Fatalf("expected task result summary, got %#v", snapshot.Result)
	}

	if _, err := os.Stat(filepath.Join(installedRoot, "weather", "info.json")); err != nil {
		t.Fatalf("expected installed manifest to exist: %v", err)
	}

	installed, ok := catalog.Get("weather")
	if !ok {
		t.Fatal("expected installed plugin in refreshed catalog")
	}
	if installed.RegistrationState != "installed" {
		t.Fatalf("unexpected registration_state: got %q want installed", installed.RegistrationState)
	}
	if installed.DesiredState != "disabled" {
		t.Fatalf("unexpected desired_state: got %q want disabled", installed.DesiredState)
	}
	if installed.RuntimeState != "stopped" {
		t.Fatalf("unexpected runtime_state: got %q want stopped", installed.RuntimeState)
	}
}

func TestInstallServiceInstallsLocalZip(t *testing.T) {
	t.Parallel()

	registry := tasks.NewRegistry()
	repoRoot := t.TempDir()
	sourceDir := writeInstallSourcePlugin(t, filepath.Join(t.TempDir(), "zip-src"), "zip-weather", "python", "main.py")
	archivePath := filepath.Join(t.TempDir(), "zip-weather.zip")
	writePluginZip(t, archivePath, sourceDir)

	service, catalog := newInstallTestService(t, repoRoot, registry, nil, installerDeps{})
	defer service.Close()

	taskID, err := service.Accept(context.Background(), InstallRequest{
		SourceType: "local_zip",
		Source:     archivePath,
	})
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	snapshot := waitForTaskCompletion(t, registry, taskID)
	if snapshot.Status != tasks.StatusSucceeded {
		t.Fatalf("unexpected task status: got %q want %q", snapshot.Status, tasks.StatusSucceeded)
	}

	if _, ok := catalog.Get("zip-weather"); !ok {
		t.Fatal("expected zip-installed plugin in refreshed catalog")
	}
}

func TestInstallServiceFailsDuplicatePluginID(t *testing.T) {
	t.Parallel()

	registry := tasks.NewRegistry()
	repoRoot := t.TempDir()
	existing := []Snapshot{{
		PluginID:          "hello-node",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "disabled",
		RuntimeState:      "stopped",
		DisplayState:      "discovered",
	}}
	sourceDir := writeInstallSourcePlugin(t, filepath.Join(t.TempDir(), "dup-src"), "hello-node", "nodejs", "index.js")
	service, _ := newInstallTestService(t, repoRoot, registry, existing, installerDeps{})
	defer service.Close()

	taskID, err := service.Accept(context.Background(), InstallRequest{
		SourceType: "local_directory",
		Source:     sourceDir,
	})
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	snapshot := waitForTaskCompletion(t, registry, taskID)
	if snapshot.Status != tasks.StatusFailed {
		t.Fatalf("unexpected task status: got %q want %q", snapshot.Status, tasks.StatusFailed)
	}
	if snapshot.Error == nil || snapshot.Error.Code != codePluginInstallFailed {
		t.Fatalf("unexpected task error: %#v", snapshot.Error)
	}
}

func TestInstallServiceCancelsRunningTask(t *testing.T) {
	t.Parallel()

	registry := tasks.NewRegistry()
	repoRoot := t.TempDir()
	sourceDir := writeInstallSourcePlugin(t, filepath.Join(t.TempDir(), "cancel-src"), "cancel-weather", "nodejs", "index.js")

	copyStarted := make(chan struct{}, 1)
	deps := installerDeps{
		copyDir: func(ctx context.Context, sourceRoot, targetRoot string) error {
			select {
			case copyStarted <- struct{}{}:
			default:
			}
			<-ctx.Done()
			return ctx.Err()
		},
	}

	service, _ := newInstallTestService(t, repoRoot, registry, nil, deps)
	defer service.Close()

	taskID, err := service.Accept(context.Background(), InstallRequest{
		SourceType: "local_directory",
		Source:     sourceDir,
	})
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	select {
	case <-copyStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for running install phase")
	}

	if !service.Cancel(taskID) {
		t.Fatal("expected running install cancellation to be accepted")
	}

	snapshot := waitForTaskCompletion(t, registry, taskID)
	if snapshot.Status != tasks.StatusCancelled {
		t.Fatalf("unexpected cancelled status: got %q want %q", snapshot.Status, tasks.StatusCancelled)
	}
}

func newInstallTestService(t *testing.T, repoRoot string, registry *tasks.Registry, initial []Snapshot, deps installerDeps) (*InstallService, *Catalog) {
	t.Helper()

	validator, err := schema.Compile(filepath.Join("..", "..", "..", "contracts", "plugin-info.schema.json"))
	if err != nil {
		t.Fatalf("compile plugin-info schema: %v", err)
	}

	examplesRoot := filepath.Join(repoRoot, "examples", "plugins")
	installedRoot := filepath.Join(repoRoot, "plugins", "installed")
	if err := os.MkdirAll(examplesRoot, 0o755); err != nil {
		t.Fatalf("create examples root: %v", err)
	}
	if err := os.MkdirAll(installedRoot, 0o755); err != nil {
		t.Fatalf("create installed root: %v", err)
	}

	catalog := NewCatalog(initial)
	service, err := newInstallService(
		nil,
		registry,
		catalog,
		&stubDesiredStateRepository{},
		validator,
		repoRoot,
		[]ScanRoot{
			{Label: "examples/plugins", Path: examplesRoot},
			{Label: "plugins/installed", Path: installedRoot},
		},
		2*time.Second,
		deps,
	)
	if err != nil {
		t.Fatalf("newInstallService failed: %v", err)
	}
	return service, catalog
}

func writeInstallSourcePlugin(t *testing.T, root, pluginID, runtimeName, entry string) string {
	t.Helper()

	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("create plugin root: %v", err)
	}

	manifest := map[string]any{
		"id":                      pluginID,
		"name":                    pluginID,
		"version":                 "0.1.0",
		"manifest_version":        "1",
		"plugin_protocol_version": "1",
		"type":                    "managed_runtime",
		"runtime":                 runtimeName,
		"entry":                   entry,
		"license":                 "MIT",
		"description":             "test plugin",
		"author":                  "raylea",
		"capabilities":            []string{"event.subscribe"},
		"permissions": map[string]any{
			"required": []string{},
			"optional": []string{},
		},
	}

	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "info.json"), manifestBytes, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	entryContent := "console.log('ok')\n"
	if runtimeName == "python" {
		entryContent = "print('ok')\n"
	}
	if err := os.WriteFile(filepath.Join(root, entry), []byte(entryContent), 0o644); err != nil {
		t.Fatalf("write entry: %v", err)
	}

	return root
}

func writePluginZip(t *testing.T, archivePath, sourceDir string) {
	t.Helper()

	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create zip file: %v", err)
	}
	defer file.Close()

	writer := zip.NewWriter(file)

	if err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(filepath.Dir(sourceDir), path)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(rel)
		if info.IsDir() {
			_, err := writer.Create(name + "/")
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = name
		header.Method = zip.Deflate
		entryWriter, err := writer.CreateHeader(header)
		if err != nil {
			return err
		}
		bytes, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, err = entryWriter.Write(bytes)
		return err
	}); err != nil {
		t.Fatalf("write zip contents: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
}

func waitForTaskCompletion(t *testing.T, registry *tasks.Registry, taskID string) tasks.Snapshot {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		snapshot, ok := registry.Get(taskID)
		if ok {
			switch snapshot.Status {
			case tasks.StatusSucceeded, tasks.StatusFailed, tasks.StatusCancelled, tasks.StatusInterrupted:
				return snapshot
			}
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for task %s to complete", taskID)
	return tasks.Snapshot{}
}
