package pluginapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"pgregory.net/rapid"
)

func TestProperty_NonExistentPluginReturns404(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		pluginID := rapid.StringMatching("[a-z][a-z0-9_]{2,30}").Draw(t, "pluginID")

		// Empty catalog — no plugins exist.
		router, _, _, _ := setupRouter(nil)

		for _, action := range []string{"enable", "disable"} {
			path := "/api/plugins/" + pluginID + "/" + action
			req := httptest.NewRequest(http.MethodPost, path, nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusNotFound {
				t.Fatalf("%s %s: status = %d, want 404", action, pluginID, rec.Code)
			}

			env := decodeErrorEnvelope(t, rec.Body.Bytes())
			if env.Error.Code != codeResourceMissing {
				t.Fatalf("%s %s: error.code = %q, want %q", action, pluginID, env.Error.Code, codeResourceMissing)
			}
		}
	})
}
func TestEnableHandler_Success(t *testing.T) {
	router, _, _, repo := setupRouter([]plugins.Snapshot{{
		PluginID:          "weather",
		Name:              "Weather",
		Version:           "1.0.0",
		RegistrationState: "installed",
		DesiredState:      "disabled",
		RuntimeState:      "stopped",
		DisplayState:      "disabled",
		Valid:             true,
	}})

	req := httptest.NewRequest(http.MethodPost, "/api/plugins/weather/enable", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	var resp pluginDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Plugin.ID != "weather" {
		t.Fatalf("plugin.id = %q, want weather", resp.Plugin.ID)
	}
	if resp.Plugin.State != "enabled" {
		t.Fatalf("plugin.state = %q, want enabled", resp.Plugin.State)
	}
	if repo.saved["weather"] != "enabled" {
		t.Fatalf("persisted enable intent = %q, want enabled", repo.saved["weather"])
	}
}

// TestDisableHandler_RuntimeStillStopping: disable an enabled plugin returns 200.
// The runtime may still be stopping. Reproduces fixture edge.plugins-disable-response.yaml.
func TestDisableHandler_RuntimeStillStopping(t *testing.T) {
	router, _, _, repo := setupRouter([]plugins.Snapshot{{
		PluginID:          "weather",
		Name:              "Weather",
		Version:           "1.0.0",
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "stopping",
		DisplayState:      "disabling",
		Valid:             true,
	}})

	req := httptest.NewRequest(http.MethodPost, "/api/plugins/weather/disable", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	var resp pluginDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Plugin.ID != "weather" {
		t.Fatalf("plugin.id = %q, want weather", resp.Plugin.ID)
	}
	if resp.Plugin.State != "stopping" {
		t.Fatalf("plugin.state = %q, want stopping", resp.Plugin.State)
	}
	if repo.saved["weather"] != "disabled" {
		t.Fatalf("persisted enable intent = %q, want disabled", repo.saved["weather"])
	}
}

func TestEnableHandler_AlreadyEnabled_409(t *testing.T) {
	router, _, _, repo := setupRouter([]plugins.Snapshot{{
		PluginID:          "weather",
		Name:              "Weather",
		Version:           "1.0.0",
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
		Valid:             true,
	}})

	req := httptest.NewRequest(http.MethodPost, "/api/plugins/weather/enable", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body = %s", rec.Code, rec.Body.String())
	}

	env := decodeErrorEnvelope(t, rec.Body.Bytes())
	if env.Error.Code == "" {
		t.Fatal("error.code is empty")
	}
	if _, ok := repo.saved["weather"]; ok {
		t.Fatal("state conflict should not persist enable intent")
	}
}

// TestDisableHandler_AlreadyDisabled_409: disable already-disabled plugin returns 409.
func TestDisableHandler_AlreadyDisabled_409(t *testing.T) {
	router, _, _, repo := setupRouter([]plugins.Snapshot{{
		PluginID:          "weather",
		Name:              "Weather",
		Version:           "1.0.0",
		RegistrationState: "installed",
		DesiredState:      "disabled",
		RuntimeState:      "stopped",
		Valid:             true,
	}})

	req := httptest.NewRequest(http.MethodPost, "/api/plugins/weather/disable", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body = %s", rec.Code, rec.Body.String())
	}

	env := decodeErrorEnvelope(t, rec.Body.Bytes())
	if env.Error.Code == "" {
		t.Fatal("error.code is empty")
	}
	if _, ok := repo.saved["weather"]; ok {
		t.Fatal("state conflict should not persist enable intent")
	}
}

// TestEnableHandler_RemovedPlugin_409: enable a removed plugin returns 409.
func TestEnableHandler_RemovedPlugin_409(t *testing.T) {
	router, _, _, repo := setupRouter([]plugins.Snapshot{{
		PluginID:          "old_plugin",
		Name:              "Old Plugin",
		Version:           "1.0.0",
		RegistrationState: "removed",
		DesiredState:      "disabled",
		RuntimeState:      "stopped",
		Valid:             true,
	}})

	req := httptest.NewRequest(http.MethodPost, "/api/plugins/old_plugin/enable", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body = %s", rec.Code, rec.Body.String())
	}

	env := decodeErrorEnvelope(t, rec.Body.Bytes())
	if env.Error.Code == "" {
		t.Fatal("error.code is empty")
	}
	if _, ok := repo.saved["old_plugin"]; ok {
		t.Fatal("removed plugin should not persist enable intent")
	}
}
