package lifecycle

import (
	"context"
	"encoding/base64"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/onebot11"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/pluginstore"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
	pluginsettings "github.com/RayleaBot/RayleaBot/server/internal/plugins/settings"
	pluginwebhook "github.com/RayleaBot/RayleaBot/server/internal/plugins/webhook"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render"
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

func (a *testApp) setTestLifecycle(t *testing.T, catalog *plugincatalog.Catalog, desiredRepo plugins.DesiredStateRepository, runtimes *testRuntimeRegistry, dispatcher *dispatch.Dispatcher, pluginConfigRepo pluginstore.ConfigRepository, adapterShell *onebot11.Shell, webhooks *pluginwebhook.Registry) {
	if a == nil {
		return
	}
	if runtimes == nil {
		runtimes = newRuntimeRegistry(a.state.Logger, pluginruntime.Options{})
	}
	deps := Deps{
		CurrentConfig:    a.state.CurrentConfig,
		RepoRoot:         a.state.repoRoot,
		Logger:           a.state.Logger,
		Plugins:          catalog,
		DesiredStateRepo: desiredRepo,
		Runtimes:         runtimes,
		Dispatcher:       dispatcher,
		Webhooks:         webhooks,
		Tasks:            a.platform.Tasks,
	}
	// Assign the adapter only when non-nil so the interface dep stays nil
	// instead of holding a typed nil.
	if adapterShell != nil {
		deps.Identities = testAdapterIdentities{shell: adapterShell}
	}
	a.services.pluginLifecycle = newTestController(t, deps, pluginConfigRepo)
}

type testRuntimeRegistry struct {
	logger  *slog.Logger
	options pluginruntime.Options

	mu      sync.RWMutex
	onCrash pluginruntime.CrashCallback
	items   map[string]*pluginruntime.Manager
}

func newRuntimeRegistry(logger *slog.Logger, options pluginruntime.Options) *testRuntimeRegistry {
	if logger == nil {
		logger = slog.Default()
	}
	return &testRuntimeRegistry{
		logger:  logger,
		options: options,
		items:   make(map[string]*pluginruntime.Manager),
	}
}

func (r *testRuntimeRegistry) Get(pluginID string) (*pluginruntime.Manager, bool) {
	if r == nil {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	manager, ok := r.items[pluginID]
	return manager, ok
}

func (r *testRuntimeRegistry) GetOrCreate(pluginID string) *pluginruntime.Manager {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if manager, ok := r.items[pluginID]; ok {
		return manager
	}
	manager := pluginruntime.NewManager(r.logger, r.options)
	manager.SetOnCrash(r.onCrash)
	r.items[pluginID] = manager
	return manager
}

func (r *testRuntimeRegistry) NewDetached() *pluginruntime.Manager {
	if r == nil {
		return nil
	}
	manager := pluginruntime.NewManager(r.logger, r.options)
	manager.SetOnCrash(r.onCrash)
	return manager
}

func (r *testRuntimeRegistry) Replace(pluginID string, manager *pluginruntime.Manager) *pluginruntime.Manager {
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

func (r *testRuntimeRegistry) Delete(pluginID string) *pluginruntime.Manager {
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
	events chan chatevent.Event
}

func (r *capturingRuntime) DeliverEvent(_ context.Context, event chatevent.Event) (plugins.Delivery, error) {
	select {
	case r.events <- event:
	default:
	}
	return plugins.Delivery{
		RequestID: "event_test_1",
		Result:    map[string]any{},
	}, nil
}

func (r *capturingRuntime) Snapshot() pluginruntime.Snapshot {
	return pluginruntime.Snapshot{State: pluginruntime.StateRunning}
}

var (
	testRenderPNGBytes, _  = base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO2W4n8AAAAASUVORK5CYII=")
	testRenderJPEGBytes, _ = base64.StdEncoding.DecodeString("/9j/4AAQSkZJRgABAQAAAQABAAD/2wCEAAkGBxAQEBAQEA8PDw8PDw8PDw8PDw8PDw8QFREWFhURFRUYHSggGBolGxUVITEhJSkrLi4uFx8zODMsNygtLisBCgoKDg0OGxAQGy0lICYtLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLf/AABEIAAEAAQMBEQACEQEDEQH/xAAXAAEBAQEAAAAAAAAAAAAAAAAAAQID/8QAFBABAAAAAAAAAAAAAAAAAAAAAP/aAAwDAQACEAMQAAAB6gD/xAAXEAEBAQEAAAAAAAAAAAAAAAABEQAh/9oACAEBAAEFAjQ2qf/EABQRAQAAAAAAAAAAAAAAAAAAABD/2gAIAQMBAT8BP//EABQRAQAAAAAAAAAAAAAAAAAAABD/2gAIAQIBAT8BP//EABYQAQEBAAAAAAAAAAAAAAAAAAERIf/aAAgBAQAGPwIhZ//EABgQAQEBAQEAAAAAAAAAAAAAAAERACEx/9oACAEBAAE/IZmBliTFkY2l/9oADAMBAAIAAwAAABAP/8QAFBEBAAAAAAAAAAAAAAAAAAAAEP/aAAgBAwEBPxA//8QAFBEBAAAAAAAAAAAAAAAAAAAAEP/aAAgBAgEBPxA//8QAGBABAAMBAAAAAAAAAAAAAAAAAQARITFR/9oACAEBAAE/EKQhNQIfY0x0KGLX/9k=")
)

type captureRenderRunner struct {
	mu   sync.Mutex
	docs []renderservice.Document
}

func (r *captureRenderRunner) Render(_ context.Context, doc renderservice.Document) ([]byte, error) {
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

func newRenderServiceForRepo(t *testing.T, repoRoot string, root string, runner renderservice.Runner) *renderservice.Service {
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
  "name": "测试模板",
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

type testAdapterIdentities struct{ shell *onebot11.Shell }

func (source testAdapterIdentities) BotIdentities() []chatevent.BotIdentity {
	if id := source.shell.CurrentBotID(); id != "" {
		return []chatevent.BotIdentity{{SourceAdapter: "onebot11", SourceProtocol: "onebot11", ID: id}}
	}
	return []chatevent.BotIdentity{}
}

// ReadyForEvents reports whether this target can accept a plugin event.
func (r *capturingRuntime) ReadyForEvents() bool {
	return r.Snapshot().State == pluginruntime.StateRunning
}

func newTestController(t *testing.T, deps Deps, repositories ...pluginstore.ConfigRepository) *Controller {
	t.Helper()
	if deps.Operations == nil {
		deps.Operations = NewOperationGate()
	}
	if deps.CurrentConfig == nil {
		deps.CurrentConfig = func() config.Config { return config.Config{} }
	}
	if deps.Plugins == nil {
		deps.Plugins = plugincatalog.New(nil)
	}
	if deps.Runtimes == nil {
		deps.Runtimes = newRuntimeRegistry(slog.Default(), pluginruntime.Options{})
	}
	if deps.Dispatcher == nil {
		deps.Dispatcher = dispatch.New(slog.Default(), nil, nil, 16)
	}
	if deps.Settings == nil {
		var repo pluginsettings.Repository = emptySettingsRepository{}
		if len(repositories) > 0 && repositories[0] != nil {
			repo = repositories[0]
		}
		service, err := pluginsettings.New(pluginsettings.Deps{Plugins: deps.Plugins, Config: repo, RefreshCommands: actions.RefreshCommands(deps.Plugins, deps.Dispatcher), Notify: actions.NotifyConfigChanged(deps.Dispatcher)})
		if err != nil {
			t.Fatal(err)
		}
		deps.Settings = service
	}
	controller, err := NewController(deps)
	if err != nil {
		t.Fatal(err)
	}
	controller.BindLifecycleContext(t.Context())
	t.Cleanup(deps.Dispatcher.Close)
	t.Cleanup(controller.Close)
	return controller
}

func (r *testRuntimeRegistry) ReleaseRetired(*pluginruntime.Manager) {}

type emptySettingsRepository struct{}

func (emptySettingsRepository) ReadAll(context.Context, string) (map[string]any, error) {
	return map[string]any{}, nil
}
func (emptySettingsRepository) Write(context.Context, string, map[string]any) ([]string, error) {
	panic("read-only lifecycle fixture settings")
}
