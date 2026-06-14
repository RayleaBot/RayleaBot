package lifecycle

import (
	"context"
	"encoding/base64"
	"log/slog"
	"os"
	"path/filepath"
	goruntime "runtime"
	"sync"
	"testing"
	"time"

	adaptershell "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/shell"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginconfig "github.com/RayleaBot/RayleaBot/server/internal/plugins/configstore"
	plugingrants "github.com/RayleaBot/RayleaBot/server/internal/plugins/grants"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
	pluginwebhook "github.com/RayleaBot/RayleaBot/server/internal/plugins/webhook"
	renderbrowser "github.com/RayleaBot/RayleaBot/server/internal/render/browser"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

type testApp struct {
	state    *testRuntimeState
	services struct {
		pluginLifecycle *Controller
	}
	platform struct {
		Tasks *tasks.Registry
	}
}

type testRuntimeState struct {
	Config   config.Config
	Logger   *slog.Logger
	repoRoot string
}

func (s *testRuntimeState) CurrentConfig() config.Config {
	if s == nil {
		return config.Config{}
	}
	return s.Config
}

func newTestAppState(cfg config.Config, logger *slog.Logger) *testApp {
	if logger == nil {
		logger = slog.Default()
	}
	return &testApp{
		state: &testRuntimeState{
			Config: cfg,
			Logger: logger,
		},
	}
}

func (a *testApp) setTestSystem(taskRegistry *tasks.Registry, _ any, _ any, _ any) {
	if a == nil {
		return
	}
	a.platform.Tasks = taskRegistry
}

func (a *testApp) setTestLifecycle(catalog *plugincatalog.Catalog, desiredRepo plugins.DesiredStateRepository, grantRepo plugins.GrantRepository, runtimes *testRuntimeRegistry, dispatcher *dispatch.Dispatcher, pluginConfigRepo pluginconfig.Repository, adapterShell *adaptershell.Shell, webhooks *pluginwebhook.Registry) {
	if a == nil {
		return
	}
	a.services.pluginLifecycle = NewController(Deps{
		CurrentConfig:    a.state.CurrentConfig,
		RepoRoot:         a.state.repoRoot,
		Logger:           a.state.Logger,
		Plugins:          catalog,
		DesiredStateRepo: desiredRepo,
		Grants: plugingrants.NewView(plugingrants.ViewDeps{
			Plugins:         catalog,
			GrantRepository: grantRepo,
		}),
		Runtimes:     runtimes,
		Dispatcher:   dispatcher,
		PluginConfig: pluginConfigRepo,
		Adapter:      adapterShell,
		Webhooks:     webhooks,
		Tasks:        a.platform.Tasks,
	})
}

type testRuntimeRegistry struct {
	logger  *slog.Logger
	options runtimemanager.Options

	mu      sync.RWMutex
	onCrash runtimemanager.CrashCallback
	items   map[string]*runtimemanager.Manager
}

func newRuntimeRegistry(logger *slog.Logger, options runtimemanager.Options) *testRuntimeRegistry {
	if logger == nil {
		logger = slog.Default()
	}
	return &testRuntimeRegistry{
		logger:  logger,
		options: options,
		items:   make(map[string]*runtimemanager.Manager),
	}
}

func (r *testRuntimeRegistry) Get(pluginID string) (*runtimemanager.Manager, bool) {
	if r == nil {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	manager, ok := r.items[pluginID]
	return manager, ok
}

func (r *testRuntimeRegistry) GetOrCreate(pluginID string) *runtimemanager.Manager {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if manager, ok := r.items[pluginID]; ok {
		return manager
	}
	manager := runtimemanager.New(r.logger, r.options)
	manager.SetOnCrash(r.onCrash)
	r.items[pluginID] = manager
	return manager
}

func (r *testRuntimeRegistry) NewDetached() *runtimemanager.Manager {
	if r == nil {
		return nil
	}
	manager := runtimemanager.New(r.logger, r.options)
	manager.SetOnCrash(r.onCrash)
	return manager
}

func (r *testRuntimeRegistry) Replace(pluginID string, manager *runtimemanager.Manager) *runtimemanager.Manager {
	if r == nil || manager == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	manager.SetOnCrash(r.onCrash)
	previous := r.items[pluginID]
	r.items[pluginID] = manager
	return previous
}

func (r *testRuntimeRegistry) Delete(pluginID string) *runtimemanager.Manager {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	manager := r.items[pluginID]
	delete(r.items, pluginID)
	return manager
}

func newPluginWebhookRegistry() *pluginwebhook.Registry {
	return pluginwebhook.NewRegistry()
}

type capturingRuntime struct {
	events chan runtimeprotocol.Event
}

func (r *capturingRuntime) DeliverEvent(_ context.Context, event runtimeprotocol.Event) (runtimemanager.Delivery, error) {
	select {
	case r.events <- event:
	default:
	}
	return runtimemanager.Delivery{
		RequestID: "event_test_1",
		Result:    map[string]any{},
	}, nil
}

func (r *capturingRuntime) Snapshot() runtimemanager.Snapshot {
	return runtimemanager.Snapshot{State: runtimemanager.StateRunning}
}

var (
	testRenderPNGBytes, _  = base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO2W4n8AAAAASUVORK5CYII=")
	testRenderJPEGBytes, _ = base64.StdEncoding.DecodeString("/9j/4AAQSkZJRgABAQAAAQABAAD/2wCEAAkGBxAQEBAQEA8PDw8PDw8PDw8PDw8PDw8QFREWFhURFRUYHSggGBolGxUVITEhJSkrLi4uFx8zODMsNygtLisBCgoKDg0OGxAQGy0lICYtLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLf/AABEIAAEAAQMBEQACEQEDEQH/xAAXAAEBAQEAAAAAAAAAAAAAAAAAAQID/8QAFBABAAAAAAAAAAAAAAAAAAAAAP/aAAwDAQACEAMQAAAB6gD/xAAXEAEBAQEAAAAAAAAAAAAAAAABEQAh/9oACAEBAAEFAjQ2qf/EABQRAQAAAAAAAAAAAAAAAAAAABD/2gAIAQMBAT8BP//EABQRAQAAAAAAAAAAAAAAAAAAABD/2gAIAQIBAT8BP//EABYQAQEBAAAAAAAAAAAAAAAAAAERIf/aAAgBAQAGPwIhZ//EABgQAQEBAQEAAAAAAAAAAAAAAAERACEx/9oACAEBAAE/IZmBliTFkY2l/9oADAMBAAIAAwAAABAP/8QAFBEBAAAAAAAAAAAAAAAAAAAAEP/aAAgBAwEBPxA//8QAFBEBAAAAAAAAAAAAAAAAAAAAEP/aAAgBAgEBPxA//8QAGBABAAMBAAAAAAAAAAAAAAAAAQARITFR/9oACAEBAAE/EKQhNQIfY0x0KGLX/9k=")
)

type captureRenderRunner struct {
	mu   sync.Mutex
	docs []renderbrowser.Document
}

func (r *captureRenderRunner) Render(_ context.Context, doc renderbrowser.Document) ([]byte, error) {
	r.mu.Lock()
	r.docs = append(r.docs, doc)
	r.mu.Unlock()
	if doc.Output == "jpeg" {
		return append([]byte(nil), testRenderJPEGBytes...), nil
	}
	return append([]byte(nil), testRenderPNGBytes...), nil
}

func (r *captureRenderRunner) lastHTML() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.docs) == 0 {
		return ""
	}
	return r.docs[len(r.docs)-1].HTML
}

func newRenderServiceForRepo(t *testing.T, repoRoot string, root string, runner renderbrowser.Runner) *renderservice.Service {
	t.Helper()

	store, err := storage.Open(filepath.Join(root, "render-state.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})

	service, err := renderservice.NewService(renderservice.Options{
		RepoRoot:           repoRoot,
		OutputRoot:         root,
		Store:              store,
		Runner:             runner,
		WorkerCount:        1,
		QueueMaxLength:     2,
		QueueWaitTimeout:   time.Second,
		RenderTimeout:      time.Second,
		MaxRenderDataBytes: 1 << 20,
	})
	if err != nil {
		t.Fatalf("renderservice.NewService: %v", err)
	}
	t.Cleanup(func() {
		_ = service.Close()
	})
	return service
}

func writePluginRenderTemplate(t *testing.T, repoRoot, pluginID, templateID string) {
	t.Helper()

	templateDir := filepath.Join(repoRoot, "plugins", "installed", pluginID, "templates", templateID)
	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		t.Fatalf("create plugin template dir: %v", err)
	}
	files := map[string]string{
		"template.json": `{
  "id": "` + templateID + `",
  "version": "1",
  "entry_html": "template.html",
  "stylesheet": "styles.css",
  "input_schema": "input.schema.json",
  "width": 320,
  "height": 240
}`,
		"template.html":     "<html><body>{{ .title }}</body></html>",
		"styles.css":        "body { margin: 0; }",
		"input.schema.json": `{"type":"object","additionalProperties":true}`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(templateDir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write plugin template file %s: %v", name, err)
		}
	}
}

func createPluginEntry(t *testing.T, repoRoot string, relativeDir string, entryName string) {
	t.Helper()

	pluginDir := filepath.Join(repoRoot, filepath.FromSlash(relativeDir))
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("create plugin dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "info.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, entryName), []byte("placeholder"), 0o644); err != nil {
		t.Fatalf("write entry: %v", err)
	}
}

func writeManagedRuntimeFixtures(t *testing.T, repoRoot string) {
	t.Helper()

	platform := testManifestPlatform()
	manifestPath := filepath.Join(repoRoot, ".deps", "manifest.json")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatalf("mkdir deps root: %v", err)
	}
	manifest := `{
  "manifest_version": 3,
  "resources": [
    {
      "id": "python-test",
      "kind": "python-runtime",
      "version": "3.12.13",
      "platform": "` + platform + `",
      "sources": [{"url": "https://example.invalid/python.tar.gz", "kind": "upstream"}],
      "sha256": "10b7a95b928e551fc78cac665999e1ae1f08fb738b255adb0a8d3b9c2824a9c0",
      "archive_format": "tar.gz",
      "entrypoints": {"python": ["python/install/bin/python3"]}
    },
    {
      "id": "node-test",
      "kind": "nodejs-runtime",
      "version": "24.14.0",
      "platform": "` + platform + `",
      "sources": [{"url": "https://example.invalid/node.tar.xz", "kind": "upstream"}],
      "sha256": "2bb9e071b229e9c0cb7d90297c51fa4cf3f5dbf4f88aded36d3f5892651baabf",
      "archive_format": "tar.xz",
      "entrypoints": {"node": ["node/bin/node"], "npm": ["node/bin/npm"]}
    }
  ]
}`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatalf("write deps manifest: %v", err)
	}
	writeManagedRuntimeEntry(t, filepath.Join(repoRoot, ".deps", "store", "python-test", "3.12.13", "python", "install", "bin", "python3"))
	writeManagedRuntimeEntry(t, filepath.Join(repoRoot, ".deps", "store", "python-test", "3.12.13", "python", "install", "bin", "pip3"))
	writeManagedRuntimeEntry(t, filepath.Join(repoRoot, ".deps", "store", "node-test", "24.14.0", "node", "bin", "node"))
	writeManagedRuntimeEntry(t, filepath.Join(repoRoot, ".deps", "store", "node-test", "24.14.0", "node", "bin", "npm"))
}

func writeManagedRuntimeEntry(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir managed runtime dir: %v", err)
	}
	if err := os.WriteFile(path, []byte("runtime"), 0o755); err != nil {
		t.Fatalf("write managed runtime entry: %v", err)
	}
}

func testManifestPlatform() string {
	switch goruntime.GOOS {
	case "windows":
		if goruntime.GOARCH == "amd64" {
			return "windows-x64"
		}
		return "windows-" + goruntime.GOARCH
	case "darwin":
		if goruntime.GOARCH == "amd64" {
			return "macos-x64"
		}
		return "macos-" + goruntime.GOARCH
	default:
		if goruntime.GOARCH == "amd64" {
			return goruntime.GOOS + "-x64"
		}
		return goruntime.GOOS + "-" + goruntime.GOARCH
	}
}

func waitTask(t *testing.T, registry *tasks.Registry, taskID string, want tasks.Status) tasks.Snapshot {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		snapshot, ok := registry.Get(taskID)
		if ok && snapshot.Status == want {
			return snapshot
		}
		time.Sleep(20 * time.Millisecond)
	}
	snapshot, _ := registry.Get(taskID)
	t.Fatalf("task %s did not reach %s: %#v", taskID, want, snapshot)
	return tasks.Snapshot{}
}
