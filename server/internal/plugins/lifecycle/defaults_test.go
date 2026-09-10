package lifecycle_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/management"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/artifact"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/lifecycle"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/settings"
	pluginstore "github.com/RayleaBot/RayleaBot/server/internal/plugins/storage"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
	"github.com/RayleaBot/RayleaBot/server/tests/testutil"
	"github.com/go-chi/chi/v5"
)

// Only the copied test executable in this fixture directory acts as a plugin.
func init() {
	if _, err := os.Stat(".settings-init-probe"); err != nil {
		return
	}
	decoder, encoder := json.NewDecoder(os.Stdin), json.NewEncoder(os.Stdout)
	current := map[string]any{}
	for {
		var frame map[string]any
		if decoder.Decode(&frame) != nil {
			os.Exit(0)
		}
		switch frame["type"] {
		case "init":
			current, _ = frame["config"].(map[string]any)
			if encoder.Encode(map[string]any{"type": "init_ack", "request_id": frame["request_id"], "status": "ready"}) != nil {
				os.Exit(2)
			}
		case "event":
			event, _ := frame["event"].(map[string]any)
			if event["event_type"] == "config.changed" {
				payload, _ := event["payload"].(map[string]any)
				current, _ = payload["config"].(map[string]any)
			}
			if encoder.Encode(map[string]any{"type": "result", "request_id": frame["request_id"], "status": "success", "data": current}) != nil {
				os.Exit(2)
			}
		case "shutdown":
			os.Exit(0)
		}
	}
}

func TestInitializationDefaultsAndExplicitOverridesMatchHTTPAndActions(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	root := t.TempDir()
	pluginRoot := filepath.Join(root, "plugins", "installed", "settings-defaults")
	if err := os.MkdirAll(pluginRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginRoot, ".settings-init-probe"), []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	entry := "probe" + filepath.Ext(executable)
	binary, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginRoot, entry), binary, 0o755); err != nil {
		t.Fatal(err)
	}
	platform, err := artifact.CurrentPlatform()
	if err != nil {
		t.Fatal(err)
	}
	artifactDoc, _ := json.Marshal(map[string]any{"artifact_version": "2", "target_platform": platform, "entry": entry})
	if err := os.WriteFile(filepath.Join(pluginRoot, "artifact.json"), artifactDoc, 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := map[string]any{"id": "settings-defaults", "name": "Settings", "version": "1.0.0", "manifest_version": "3", "min_core_version": "0.4.0", "license": "MIT", "metadata": map[string]any{"author": "fixture", "description": "Settings initialization probe"}, "events": []string{"config.changed", "management.action"}, "default_config": map[string]any{"city": "Beijing", "unit": "celsius", "fallback": "old"}}
	writeManifest := func() {
		data, err := json.Marshal(manifest)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(pluginRoot, "info.json"), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeManifest()
	validator, err := config.Compile(testutil.RepoPath(t, "contracts", "plugin-info.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	discover := func() []plugins.Snapshot {
		entries, _, err := catalog.Discover(catalog.DiscoverOptions{Validator: validator, RepoRoot: root, Roots: []catalog.ScanRoot{{Label: "plugins/installed", Path: filepath.Dir(pluginRoot)}}})
		if err != nil || len(entries) != 1 || !entries[0].Valid {
			t.Fatalf("discover: %#v %v", entries, err)
		}
		entries[0].DesiredState = "enabled"
		return entries
	}
	cat := catalog.New(discover())
	db, err := storage.Open(filepath.Join(root, "settings.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repo, err := pluginstore.NewConfigSQLiteRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dispatcher := dispatch.New(logger, nil, nil, 8)
	t.Cleanup(dispatcher.Close)
	svc, err := settings.New(settings.Deps{Plugins: cat, Config: repo, RefreshCommands: actions.RefreshCommands(cat, dispatcher), Notify: actions.NotifyConfigChanged(dispatcher)})
	if err != nil {
		t.Fatal(err)
	}
	actionService := actions.New(actions.Deps{Settings: svc})
	runtimes := pluginruntime.NewRegistry(logger, pluginruntime.Options{ExecuteLocalAction: actionService.Execute})
	controller, err := lifecycle.NewController(lifecycle.Deps{CurrentConfig: func() config.Config {
		return config.Config{Scheduler: config.SchedulerConfig{Timezone: "Asia/Shanghai"}, Runtime: config.RuntimeConfig{ShutdownGraceSeconds: 3}}
	}, RepoRoot: root, Logger: logger, Plugins: cat, Runtimes: runtimes, Dispatcher: dispatcher, Settings: svc, Operations: lifecycle.NewOperationGate()})
	if err != nil {
		t.Fatal(err)
	}
	controller.BindLifecycleContext(ctx)
	t.Cleanup(func() {
		controller.Close()
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = runtimes.StopAll(stopCtx)
	})
	router := chi.NewRouter()
	management.NewPluginManagementUIHandlers(management.PluginManagementUIDeps{Plugins: cat, Settings: svc}).RegisterProtectedRoutes(router)
	request := func(method, body string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(method, "/api/plugins/settings-defaults/settings", strings.NewReader(body)))
		return recorder
	}
	if err := controller.StartInstalled(ctx, "settings-defaults"); err != nil {
		t.Fatal(err)
	}
	initial, err := controller.InvokeManagementAction(ctx, "settings-defaults", "snapshot", nil)
	if err != nil || initial["fallback"] != "old" {
		t.Fatalf("first init: %#v %v", initial, err)
	}
	persisted, err := repo.ReadAll(ctx, "settings-defaults")
	if err != nil || len(persisted) != 0 {
		t.Fatalf("initialization persisted defaults: %#v %v", persisted, err)
	}
	saved := request(http.MethodPut, `{"values":{"unit":"celsius"}}`)
	if saved.Code != 200 {
		t.Fatal(saved.Body.String())
	}
	result, err := actionService.Execute(ctx, "settings-defaults", "write-city", plugins.Action{Kind: "config.write", ConfigValues: map[string]any{"city": "Shanghai"}}, chatevent.Event{})
	if err != nil || !reflect.DeepEqual(result["changed_keys"], []string{"city"}) {
		t.Fatalf("action override: %#v %v", result, err)
	}
	stopCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	err = controller.StopAndResetPluginWithContext(stopCtx, "settings-defaults")
	cancel()
	if err != nil {
		t.Fatal(err)
	}
	manifest["default_config"] = map[string]any{"city": "Wuhan", "unit": "fahrenheit", "fallback": "new", "timeout": 30}
	writeManifest()
	cat.Replace(discover())
	if err := controller.StartInstalled(ctx, "settings-defaults"); err != nil {
		t.Fatal(err)
	}
	restarted, err := controller.InvokeManagementAction(ctx, "settings-defaults", "snapshot", nil)
	if err != nil {
		t.Fatal(err)
	}
	response := request(http.MethodGet, "")
	var fromHTTP management.PluginSettingsResponse
	if err := json.Unmarshal(response.Body.Bytes(), &fromHTTP); err != nil || response.Code != 200 {
		t.Fatalf("HTTP read: %s %v", response.Body.String(), err)
	}
	expected := map[string]any{"city": "Shanghai", "unit": "celsius", "fallback": "new", "timeout": float64(30)}
	if !reflect.DeepEqual(restarted, expected) || !reflect.DeepEqual(fromHTTP.Values, expected) {
		t.Fatalf("init/HTTP diverged: init=%#v HTTP=%#v", restarted, fromHTTP.Values)
	}
	persisted, err = repo.ReadAll(ctx, "settings-defaults")
	if err != nil || len(persisted) != 2 {
		t.Fatalf("restart wrote unsaved defaults: %#v %v", persisted, err)
	}
}
