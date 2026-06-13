package plugininstall

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugindiscovery "github.com/RayleaBot/RayleaBot/server/internal/plugins/discovery"
	"github.com/RayleaBot/RayleaBot/server/internal/schema"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
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
	service.SetAfterSuccess(func(pluginID string) error {
		called <- pluginID
		return nil
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

func TestInstallServiceFailsWhenAfterSuccessCallbackFails(t *testing.T) {
	t.Parallel()

	registry := tasks.NewRegistry()
	repoRoot := t.TempDir()
	sourceDir := writeInstallSourcePlugin(t, filepath.Join(t.TempDir(), "callback-fail-src"), "callback-fail-weather", "nodejs", "index.js")
	repository := &stubInstallRepository{}
	service, catalog := newInstallTestService(t, repoRoot, registry, nil, repository, installerDeps{})
	defer service.Close()

	service.SetAfterSuccess(func(pluginID string) error {
		if pluginID != "callback-fail-weather" {
			t.Fatalf("unexpected callback plugin id: got %q want callback-fail-weather", pluginID)
		}
		return fmt.Errorf("sync plugin render template callback-fail-weather: source conflict")
	})

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
	if _, ok := catalog.Get("callback-fail-weather"); ok {
		t.Fatal("plugin remained in catalog after after-success failure")
	}
	if _, err := os.Stat(filepath.Join(repoRoot, "plugins", "installed", "callback-fail-weather")); !os.IsNotExist(err) {
		t.Fatalf("installed directory was not rolled back: %v", err)
	}
	if repository.deletedPackage != "callback-fail-weather" {
		t.Fatalf("package metadata was not rolled back: got %q", repository.deletedPackage)
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

func TestInstallServiceRejectsInvalidRenderTemplatePackage(t *testing.T) {
	t.Parallel()

	registry := tasks.NewRegistry()
	repoRoot := t.TempDir()
	sourceDir := writeInstallSourcePlugin(t, filepath.Join(t.TempDir(), "template-src"), "template-weather", "python", "main.py")
	addRenderTemplateDeclarationToManifest(t, sourceDir, "templates/card")

	service, _ := newInstallTestService(t, repoRoot, registry, nil, &stubInstallRepository{}, installerDeps{})
	service.SetRenderTemplateValidator(func(snapshot plugins.Snapshot) error {
		return validateInstallRenderTemplates(snapshot)
	})
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

func TestInstallServiceInstallsRenderTemplatePackage(t *testing.T) {
	t.Parallel()

	registry := tasks.NewRegistry()
	repoRoot := t.TempDir()
	sourceDir := writeInstallSourcePlugin(t, filepath.Join(t.TempDir(), "template-ok-src"), "template-ok-weather", "python", "main.py")
	addRenderTemplateDeclarationToManifest(t, sourceDir, "templates/card")
	writeInstallRenderTemplate(t, filepath.Join(sourceDir, "templates", "card"), "card")

	service, catalog := newInstallTestService(t, repoRoot, registry, nil, &stubInstallRepository{}, installerDeps{})
	service.SetRenderTemplateValidator(validateInstallRenderTemplates)
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
		t.Fatalf("unexpected task status: got %q want %q (%#v)", snapshot.Status, tasks.StatusSucceeded, snapshot.Error)
	}
	installed, ok := catalog.Get("template-ok-weather")
	if !ok {
		t.Fatal("expected installed plugin in refreshed catalog")
	}
	if len(installed.RenderTemplates) != 1 || installed.RenderTemplates[0].Path != "templates/card" {
		t.Fatalf("unexpected render_templates: %#v", installed.RenderTemplates)
	}
}

func TestInstallServiceRejectsInvalidRenderTemplateManifest(t *testing.T) {
	t.Parallel()

	registry := tasks.NewRegistry()
	repoRoot := t.TempDir()
	sourceDir := writeInstallSourcePlugin(t, filepath.Join(t.TempDir(), "template-bad-src"), "template-bad-weather", "python", "main.py")
	addRenderTemplateDeclarationToManifest(t, sourceDir, "templates/card")
	writeInstallRenderTemplate(t, filepath.Join(sourceDir, "templates", "card"), "card/escaped")

	service, _ := newInstallTestService(t, repoRoot, registry, nil, &stubInstallRepository{}, installerDeps{})
	service.SetRenderTemplateValidator(validateInstallRenderTemplates)
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

func TestInstallServiceFailsDuplicatePluginID(t *testing.T) {
	t.Parallel()

	registry := tasks.NewRegistry()
	repoRoot := t.TempDir()
	existing := []plugins.Snapshot{{
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

func newInstallTestService(t *testing.T, repoRoot string, registry *tasks.Registry, initial []plugins.Snapshot, repository plugins.DesiredStateRepository, deps installerDeps) (*InstallService, *testCatalog) {
	t.Helper()

	validator, err := schema.Compile(filepath.Join("..", "..", "..", "..", "contracts", "plugin-info.schema.json"))
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

	catalog := newTestCatalog(initial)
	service, err := newInstallService(
		nil,
		registry,
		catalog,
		repository,
		validator,
		repoRoot,
		[]plugindiscovery.ScanRoot{
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
		return 20 * time.Second
	}
	return 15 * time.Second
}

type installSourceOptions struct {
	PythonDependencies    []string
	NodeDependencies      []string
	RequireInstallScripts bool
	WritePackageJSON      bool
}

type stubInstallRepository struct {
	saved          map[string]string
	lastPackage    plugins.PackageMetadata
	deletedPackage string
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

func (r *stubInstallRepository) SavePackageMetadata(_ context.Context, pkg plugins.PackageMetadata) error {
	r.lastPackage = pkg
	return nil
}

func (r *stubInstallRepository) DeleteDesiredState(_ context.Context, _ string) error {
	return nil
}

func (r *stubInstallRepository) DeletePackageMetadata(_ context.Context, pluginID string) error {
	r.deletedPackage = pluginID
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

func addRenderTemplateDeclarationToManifest(t *testing.T, pluginRoot, templatePath string) {
	t.Helper()

	infoPath := filepath.Join(pluginRoot, "info.json")
	bytes, err := os.ReadFile(infoPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(bytes, &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	manifest["render_templates"] = []map[string]any{{"path": templatePath}}
	manifest["capabilities"] = []string{"event.subscribe", "render.image"}
	manifest["permissions"] = map[string]any{
		"required": []string{"render.image"},
		"optional": []string{},
	}
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("encode manifest: %v", err)
	}
	if err := os.WriteFile(infoPath, encoded, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}

func writeInstallRenderTemplate(t *testing.T, templateDir, templateID string) {
	t.Helper()

	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		t.Fatalf("create template dir: %v", err)
	}
	html := "<html><body>{{ .title }}</body></html>"
	files := map[string]string{
		"template.json": fmt.Sprintf(`{
  "id": %q,
  "version": "1",
  "entry_html": "template.html",
  "stylesheet": "styles.css",
  "input_schema": "input.schema.json",
  "width": 320,
  "height": 240
}`, templateID),
		"template.html":     html,
		"styles.css":        "body { margin: 0; }",
		"input.schema.json": `{"type":"object","additionalProperties":true}`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(templateDir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write template %s: %v", name, err)
		}
	}
}

func validateInstallRenderTemplates(snapshot plugins.Snapshot) error {
	for _, declared := range snapshot.RenderTemplates {
		templateDir := filepath.Join(snapshot.PackageRootPath, filepath.FromSlash(declared.Path))
		if info, err := os.Stat(templateDir); err != nil || !info.IsDir() {
			return fmt.Errorf("load plugin render template %s: template directory is missing", snapshot.PluginID)
		}
		manifestPath := filepath.Join(templateDir, "template.json")
		document, err := schema.LoadJSONFile(manifestPath)
		if err != nil {
			return fmt.Errorf("load plugin render template %s: %w", snapshot.PluginID, err)
		}
		manifest, ok := document.(map[string]any)
		if !ok {
			return fmt.Errorf("load plugin render template %s: manifest must be an object", snapshot.PluginID)
		}
		id, ok := manifest["id"].(string)
		if !ok || id == "" || strings.Contains(id, "/") || strings.Contains(id, "\\") {
			return fmt.Errorf("load plugin render template %s: template id is invalid", snapshot.PluginID)
		}
	}
	return nil
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
		return 20 * time.Second
	}
	return 15 * time.Second
}
