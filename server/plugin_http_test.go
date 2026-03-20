package server

import (
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/go-chi/chi/v5"

	"rayleabot/server/internal/plugins"
)

func TestListPluginsReturnsContractShape(t *testing.T) {
	t.Parallel()

	router := pluginRouter(t, plugins.NewCatalog([]plugins.Snapshot{
		{
			PluginID:          "alpha",
			Valid:             true,
			RegistrationState: "installed",
			DesiredState:      "disabled",
			RuntimeState:      "stopped",
			DisplayState:      "discovered",
			Name:              "Alpha",
			Version:           "0.1.0",
			Runtime:           "python",
			Description:       "should not leak into public response",
			ManifestPath:      "examples/plugins/alpha/info.json",
		},
		{
			PluginID:          "broken",
			Valid:             false,
			RegistrationState: "installed",
			DesiredState:      "disabled",
			RuntimeState:      "stopped",
			DisplayState:      "invalid_manifest",
			ValidationSummary: "invalid runtime",
		},
		{
			PluginID:          "weather",
			Valid:             false,
			RegistrationState: "installed",
			DesiredState:      "disabled",
			RuntimeState:      "stopped",
			DisplayState:      "conflict",
			ConflictPaths: []string{
				"examples/plugins/weather/info.json",
				"plugins/installed/weather/info.json",
			},
			SourceRoots: []string{"examples/plugins", "plugins/installed"},
		},
	}))

	request := httptest.NewRequest("GET", "/api/plugins", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != 200 {
		t.Fatalf("unexpected status: got %d want 200", recorder.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	items, ok := body["items"].([]any)
	if !ok {
		t.Fatalf("expected items array, got %#v", body["items"])
	}
	if len(items) != 3 {
		t.Fatalf("unexpected item count: got %d want 3", len(items))
	}

	for _, item := range items {
		itemMap, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("expected item object, got %#v", item)
		}
		allowed := map[string]bool{
			"id":                 true,
			"registration_state": true,
			"desired_state":      true,
			"runtime_state":      true,
			"display_state":      true,
		}
		for key := range itemMap {
			if !allowed[key] {
				t.Fatalf("unexpected public field %q in list response", key)
			}
		}
	}
}

func TestGetPluginReturnsValidSnapshot(t *testing.T) {
	t.Parallel()

	router := pluginRouter(t, plugins.NewCatalog([]plugins.Snapshot{
		{
			PluginID:          "hello-python",
			Valid:             true,
			RegistrationState: "installed",
			DesiredState:      "disabled",
			RuntimeState:      "stopped",
			DisplayState:      "discovered",
		},
	}))

	request := httptest.NewRequest("GET", "/api/plugins/hello-python", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != 200 {
		t.Fatalf("unexpected status: got %d want 200", recorder.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	want := map[string]any{
		"plugin": map[string]any{
			"id":                 "hello-python",
			"registration_state": "installed",
			"desired_state":      "disabled",
			"runtime_state":      "stopped",
			"display_state":      "discovered",
		},
	}
	if !reflect.DeepEqual(body, want) {
		t.Fatalf("unexpected body: got %#v want %#v", body, want)
	}
}

func TestGetPluginReturns404WhenMissing(t *testing.T) {
	t.Parallel()

	router := pluginRouter(t, plugins.NewCatalog(nil))

	request := httptest.NewRequest("GET", "/api/plugins/missing-plugin", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != 404 {
		t.Fatalf("unexpected status: got %d want 404", recorder.Code)
	}

	body := decodeBody(t, recorder.Body.Bytes())
	errorBody := body["error"].(map[string]any)
	if errorBody["code"] != "platform.resource_missing" {
		t.Fatalf("unexpected error code: %#v", errorBody["code"])
	}
	details := errorBody["details"].(map[string]any)
	if details["resource_type"] != "plugin" {
		t.Fatalf("unexpected resource_type: %#v", details["resource_type"])
	}
}

func TestInvalidPluginAppearsInListButDetailReturns409(t *testing.T) {
	t.Parallel()

	snapshot := plugins.Snapshot{
		PluginID:          "legacy-binary-tool",
		Valid:             false,
		RegistrationState: "installed",
		DesiredState:      "disabled",
		RuntimeState:      "stopped",
		DisplayState:      "invalid_manifest",
		ManifestPath:      "plugins/installed/legacy-binary-tool/info.json",
		ValidationSummary: "runtime must be one of python or nodejs",
	}
	router := pluginRouter(t, plugins.NewCatalog([]plugins.Snapshot{snapshot}))

	listRequest := httptest.NewRequest("GET", "/api/plugins", nil)
	listRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != 200 {
		t.Fatalf("unexpected list status: got %d want 200", listRecorder.Code)
	}
	listBody := decodeBody(t, listRecorder.Body.Bytes())
	items := listBody["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("unexpected list count: got %d want 1", len(items))
	}

	detailRequest := httptest.NewRequest("GET", "/api/plugins/legacy-binary-tool", nil)
	detailRecorder := httptest.NewRecorder()
	router.ServeHTTP(detailRecorder, detailRequest)
	if detailRecorder.Code != 409 {
		t.Fatalf("unexpected detail status: got %d want 409", detailRecorder.Code)
	}

	detailBody := decodeBody(t, detailRecorder.Body.Bytes())
	errorBody := detailBody["error"].(map[string]any)
	if errorBody["code"] != "platform.invalid_request" {
		t.Fatalf("unexpected error code: %#v", errorBody["code"])
	}
	details := errorBody["details"].(map[string]any)
	if details["kind"] != "invalid_manifest" {
		t.Fatalf("unexpected error kind: %#v", details["kind"])
	}
}

func TestConflictPluginDetailReturns409(t *testing.T) {
	t.Parallel()

	router := pluginRouter(t, plugins.NewCatalog([]plugins.Snapshot{
		{
			PluginID:          "weather",
			Valid:             false,
			RegistrationState: "installed",
			DesiredState:      "disabled",
			RuntimeState:      "stopped",
			DisplayState:      "conflict",
			ValidationSummary: "duplicate plugin_id discovered across multiple directories",
			ConflictPaths: []string{
				"examples/plugins/weather/info.json",
				"plugins/installed/weather/info.json",
			},
			SourceRoots: []string{"examples/plugins", "plugins/installed"},
		},
	}))

	request := httptest.NewRequest("GET", "/api/plugins/weather", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != 409 {
		t.Fatalf("unexpected status: got %d want 409", recorder.Code)
	}

	body := decodeBody(t, recorder.Body.Bytes())
	errorBody := body["error"].(map[string]any)
	if errorBody["code"] != "platform.invalid_request" {
		t.Fatalf("unexpected error code: %#v", errorBody["code"])
	}
	details := errorBody["details"].(map[string]any)
	if details["kind"] != "plugin_id_conflict" {
		t.Fatalf("unexpected conflict kind: %#v", details["kind"])
	}
	if len(details["manifest_paths"].([]any)) != 2 {
		t.Fatalf("unexpected manifest_paths length: %#v", details["manifest_paths"])
	}
}

func pluginRouter(t *testing.T, catalog *plugins.Catalog) *chi.Mux {
	t.Helper()

	router := chi.NewRouter()
	plugins.RegisterRoutes(router, catalog, nil, nil, nil)
	return router
}

func decodeBody(t *testing.T, raw []byte) map[string]any {
	t.Helper()

	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("unmarshal response body: %v", err)
	}

	return body
}
