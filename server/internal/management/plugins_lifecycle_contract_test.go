package management

import (
	"net/http/httptest"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
)

func TestReloadPluginReturnsUpdatedSnapshot(t *testing.T) {
	t.Parallel()

	catalog := plugincatalog.New([]plugins.Snapshot{
		{PluginID: "weather", Valid: true, RegistrationState: "installed", DesiredState: "enabled", RuntimeState: "running"},
	})
	controller := &stubReloadController{
		reloadResult: plugins.Snapshot{
			PluginID: "weather", Valid: true, RegistrationState: "installed", DesiredState: "enabled",
			RuntimeState: "starting", DisplayState: "enabling",
		},
	}
	router := pluginRouterWithController(t, catalog, controller, nil)

	request := httptest.NewRequest("POST", "/api/plugins/weather/reload", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != 200 {
		t.Fatalf("unexpected status: got %d want 200", recorder.Code)
	}

	body := decodeBody(t, recorder.Body.Bytes())
	plugin := body["plugin"].(map[string]any)
	if plugin["state"] != "starting" {
		t.Fatalf("state should be starting, got %v", plugin["state"])
	}
}

func TestReloadPluginRejectsDisabledPlugin(t *testing.T) {
	t.Parallel()

	catalog := plugincatalog.New([]plugins.Snapshot{
		{PluginID: "weather", Valid: true, RegistrationState: "installed", DesiredState: "disabled", RuntimeState: "stopped"},
	})
	controller := &stubReloadController{
		reloadErr: plugins.ErrStateConflict,
	}
	router := pluginRouterWithController(t, catalog, controller, nil)

	request := httptest.NewRequest("POST", "/api/plugins/weather/reload", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != 409 {
		t.Fatalf("unexpected status: got %d want 409", recorder.Code)
	}
}

func TestReloadPluginRejectsNotFound(t *testing.T) {
	t.Parallel()

	catalog := plugincatalog.New(nil)
	controller := &stubReloadController{
		reloadErr: plugins.ErrPluginNotFound,
	}
	router := pluginRouterWithController(t, catalog, controller, nil)

	request := httptest.NewRequest("POST", "/api/plugins/nonexistent/reload", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != 404 {
		t.Fatalf("unexpected status: got %d want 404", recorder.Code)
	}
}

func TestUninstallPluginReturnsTaskAccepted(t *testing.T) {
	t.Parallel()

	catalog := plugincatalog.New([]plugins.Snapshot{
		{PluginID: "weather", Valid: true, RegistrationState: "installed", DesiredState: "disabled", RuntimeState: "stopped"},
	})
	uninstaller := &stubUninstallCoordinator{taskID: "task_plugin_uninstall_0001"}
	router := pluginRouterWithController(t, catalog, nil, uninstaller)

	request := httptest.NewRequest("DELETE", "/api/plugins/weather", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != 202 {
		t.Fatalf("unexpected status: got %d want 202", recorder.Code)
	}

	body := decodeBody(t, recorder.Body.Bytes())
	if body["task_id"] != "task_plugin_uninstall_0001" {
		t.Fatalf("unexpected task_id: %v", body["task_id"])
	}
}

func TestUninstallPluginRejectsNotFound(t *testing.T) {
	t.Parallel()

	catalog := plugincatalog.New(nil)
	uninstaller := &stubUninstallCoordinator{taskID: "should-not-reach"}
	router := pluginRouterWithController(t, catalog, nil, uninstaller)

	request := httptest.NewRequest("DELETE", "/api/plugins/nonexistent", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != 404 {
		t.Fatalf("unexpected status: got %d want 404", recorder.Code)
	}

	body := decodeBody(t, recorder.Body.Bytes())
	errorBody := body["error"].(map[string]any)
	if errorBody["code"] != "platform.resource_missing" {
		t.Fatalf("unexpected error code: %v", errorBody["code"])
	}
}

func TestUninstallInstalledPluginAcceptedRegardlessOfFormerDistribution(t *testing.T) {
	t.Parallel()

	catalog := plugincatalog.New([]plugins.Snapshot{
		{
			PluginID:          "raylea.echo",
			Valid:             true,
			RegistrationState: "installed",
			DesiredState:      "enabled",
			RuntimeState:      "stopped",
			SourceRoot:        "plugins/installed",
		},
	})
	uninstaller := &stubUninstallCoordinator{taskID: "should-not-run"}
	router := pluginRouterWithController(t, catalog, nil, uninstaller)

	request := httptest.NewRequest("DELETE", "/api/plugins/raylea.echo", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != 202 {
		t.Fatalf("unexpected status: got %d want 202", recorder.Code)
	}
	if uninstaller.pluginID != "raylea.echo" {
		t.Fatalf("uninstaller plugin id = %q", uninstaller.pluginID)
	}
}
