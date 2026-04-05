package plugins

import (
	"archive/zip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
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
	repository := &stubInstallRepository{}
	service, catalog := newInstallTestService(t, repoRoot, registry, nil, repository, installerDeps{})
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
	if repository.lastPackage.PluginID != "weather" {
		t.Fatalf("expected package metadata for weather, got %#v", repository.lastPackage)
	}
	if repository.lastPackage.SourceType != "local_directory" {
		t.Fatalf("unexpected source_type metadata: got %q want local_directory", repository.lastPackage.SourceType)
	}
	if repository.lastPackage.Version != "0.1.0" {
		t.Fatalf("unexpected version metadata: got %q want 0.1.0", repository.lastPackage.Version)
	}
	if repository.lastPackage.ManifestHash == "" || repository.lastPackage.PackageHash == "" {
		t.Fatalf("expected package metadata hashes to be populated, got %#v", repository.lastPackage)
	}
}

func TestInstallServiceInvokesAfterSuccessCallback(t *testing.T) {
	t.Parallel()

	registry := tasks.NewRegistry()
	repoRoot := t.TempDir()
	sourceDir := writeInstallSourcePlugin(t, filepath.Join(t.TempDir(), "callback-src"), "callback-weather", "nodejs", "index.js")
	service, _ := newInstallTestService(t, repoRoot, registry, nil, &stubInstallRepository{}, installerDeps{})
	defer service.Close()

	called := make(chan string, 1)
	service.SetAfterSuccess(func(pluginID string) {
		called <- pluginID
	})

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

	select {
	case pluginID := <-called:
		if pluginID != "callback-weather" {
			t.Fatalf("unexpected callback plugin id: got %q want callback-weather", pluginID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for install after-success callback")
	}
}

func TestInstallServiceInstallsLocalZip(t *testing.T) {
	t.Parallel()

	registry := tasks.NewRegistry()
	repoRoot := t.TempDir()
	sourceDir := writeInstallSourcePlugin(t, filepath.Join(t.TempDir(), "zip-src"), "zip-weather", "python", "main.py")
	archivePath := filepath.Join(t.TempDir(), "zip-weather.zip")
	writePluginZip(t, archivePath, sourceDir)

	service, catalog := newInstallTestService(t, repoRoot, registry, nil, &stubInstallRepository{}, installerDeps{})
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
	service, _ := newInstallTestService(t, repoRoot, registry, existing, &stubInstallRepository{}, installerDeps{})
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

	service, _ := newInstallTestService(t, repoRoot, registry, nil, &stubInstallRepository{}, deps)
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

func TestInstallServiceBlocksInstallScriptsWithoutAuthorization(t *testing.T) {
	t.Parallel()

	registry := tasks.NewRegistry()
	repoRoot := t.TempDir()
	sourceDir := writeInstallSourcePlugin(t, filepath.Join(t.TempDir(), "script-src"), "scripted-node", "nodejs", "index.js", installSourceOptions{
		RequireInstallScripts: true,
		WritePackageJSON:      true,
	})
	service, _ := newInstallTestService(t, repoRoot, registry, nil, &stubInstallRepository{}, installerDeps{})
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
	if snapshot.Error == nil || snapshot.Error.Code != "platform.install_script_blocked" {
		t.Fatalf("unexpected task error: %#v", snapshot.Error)
	}
}

func TestInstallServicePreparesRuntimeDependencies(t *testing.T) {
	t.Parallel()

	registry := tasks.NewRegistry()
	repoRoot := t.TempDir()
	pythonSource := writeInstallSourcePlugin(t, filepath.Join(t.TempDir(), "python-src"), "weather-python", "python", "main.py", installSourceOptions{
		PythonDependencies: []string{"httpx==0.27.0"},
	})
	nodeSource := writeInstallSourcePlugin(t, filepath.Join(t.TempDir(), "node-src"), "weather-node", "nodejs", "index.js", installSourceOptions{
		NodeDependencies:      []string{"left-pad@1.3.0"},
		RequireInstallScripts: true,
		WritePackageJSON:      true,
	})

	var (
		mu                sync.Mutex
		pythonPreparedFor []string
		nodePrepared      []struct {
			pluginID            string
			allowInstallScripts bool
		}
	)

	repository := &stubInstallRepository{}
	service, _ := newInstallTestService(t, repoRoot, registry, nil, repository, installerDeps{
		preparePython: func(_ context.Context, pluginDir string, dependencies []string) error {
			mu.Lock()
			defer mu.Unlock()
			pythonPreparedFor = append(pythonPreparedFor, filepath.Base(pluginDir)+":"+dependencies[0])
			return nil
		},
		prepareNode: func(_ context.Context, pluginDir string, dependencies []string, allowInstallScripts bool) error {
			mu.Lock()
			defer mu.Unlock()
			nodePrepared = append(nodePrepared, struct {
				pluginID            string
				allowInstallScripts bool
			}{
				pluginID:            filepath.Base(pluginDir),
				allowInstallScripts: allowInstallScripts,
			})
			return nil
		},
	})
	defer service.Close()

	pythonTaskID, err := service.Accept(context.Background(), InstallRequest{
		SourceType: "local_directory",
		Source:     pythonSource,
	})
	if err != nil {
		t.Fatalf("python Accept failed: %v", err)
	}
	nodeTaskID, err := service.Accept(context.Background(), InstallRequest{
		SourceType:          "local_directory",
		Source:              nodeSource,
		AllowInstallScripts: true,
	})
	if err != nil {
		t.Fatalf("node Accept failed: %v", err)
	}

	pythonSnapshot := waitForTaskCompletion(t, registry, pythonTaskID)
	if pythonSnapshot.Status != tasks.StatusSucceeded {
		t.Fatalf("unexpected python task status: got %q want %q", pythonSnapshot.Status, tasks.StatusSucceeded)
	}
	nodeSnapshot := waitForTaskCompletion(t, registry, nodeTaskID)
	if nodeSnapshot.Status != tasks.StatusSucceeded {
		t.Fatalf("unexpected node task status: got %q want %q", nodeSnapshot.Status, tasks.StatusSucceeded)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(pythonPreparedFor) != 1 {
		t.Fatalf("expected python dependency preparation once, got %#v", pythonPreparedFor)
	}
	if len(nodePrepared) != 1 {
		t.Fatalf("expected node dependency preparation once, got %#v", nodePrepared)
	}
	if !nodePrepared[0].allowInstallScripts {
		t.Fatalf("expected allow_install_scripts=true to reach node preparation, got %#v", nodePrepared)
	}
}

func newInstallTestService(t *testing.T, repoRoot string, registry *tasks.Registry, initial []Snapshot, repository DesiredStateRepository, deps installerDeps) (*InstallService, *Catalog) {
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
		repository,
		validator,
		repoRoot,
		[]ScanRoot{
			{Label: "examples/plugins", Path: examplesRoot},
			{Label: "plugins/installed", Path: installedRoot},
		},
		installServiceTimeout(),
		deps,
	)
	if err != nil {
		t.Fatalf("newInstallService failed: %v", err)
	}
	return service, catalog
}

func installServiceTimeout() time.Duration {
	if testing.CoverMode() != "" || testRaceEnabled {
		return 10 * time.Second
	}
	return 5 * time.Second
}

type installSourceOptions struct {
	PythonDependencies    []string
	NodeDependencies      []string
	RequireInstallScripts bool
	WritePackageJSON      bool
}

type stubInstallRepository struct {
	saved       map[string]string
	lastPackage PackageMetadata
}

func (r *stubInstallRepository) LoadDesiredStates(context.Context) (map[string]string, error) {
	if r == nil {
		return nil, nil
	}
	return r.saved, nil
}

func (r *stubInstallRepository) SaveDesiredState(_ context.Context, pluginID string, desiredState string, _ time.Time) error {
	if r.saved == nil {
		r.saved = make(map[string]string)
	}
	r.saved[pluginID] = desiredState
	return nil
}

func (r *stubInstallRepository) SavePackageMetadata(_ context.Context, pkg PackageMetadata) error {
	r.lastPackage = pkg
	return nil
}

func (r *stubInstallRepository) DeleteDesiredState(_ context.Context, _ string) error {
	return nil
}

func (r *stubInstallRepository) DeletePackageMetadata(_ context.Context, _ string) error {
	return nil
}

func writeInstallSourcePlugin(t *testing.T, root, pluginID, runtimeName, entry string, options ...installSourceOptions) string {
	t.Helper()

	opts := installSourceOptions{}
	if len(options) > 0 {
		opts = options[0]
	}

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
		"dependencies": map[string]any{
			"python": append([]string{}, opts.PythonDependencies...),
			"nodejs": append([]string{}, opts.NodeDependencies...),
		},
		"require_install_scripts": opts.RequireInstallScripts,
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
	if opts.WritePackageJSON {
		packageJSON := map[string]any{
			"name":    pluginID,
			"version": "0.1.0",
		}
		packageJSONBytes, err := json.MarshalIndent(packageJSON, "", "  ")
		if err != nil {
			t.Fatalf("marshal package.json: %v", err)
		}
		if err := os.WriteFile(filepath.Join(root, "package.json"), packageJSONBytes, 0o644); err != nil {
			t.Fatalf("write package.json: %v", err)
		}
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

	deadline := time.Now().Add(taskCompletionTimeout())
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

func taskCompletionTimeout() time.Duration {
	if testing.CoverMode() != "" || testRaceEnabled {
		return 10 * time.Second
	}
	return 5 * time.Second
}
