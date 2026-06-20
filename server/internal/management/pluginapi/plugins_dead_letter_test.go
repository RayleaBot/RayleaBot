package pluginapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/go-chi/chi/v5"
)

// TestRecoverFromDeadLetterHandler_Success verifies the recover endpoint
// returns the plugin detail snapshot when the controller succeeds.
// Reproduces fixture ok.plugins-recover-response.yaml.
func TestRecoverFromDeadLetterHandler_Success(t *testing.T) {
	t.Parallel()

	catalog := newTestCatalog([]plugins.Snapshot{{
		PluginID:          "weather",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "dead_letter",
		DisplayState:      "dead_letter",
	}})
	controller := &stubDesiredStateController{
		recoverResult: plugins.Snapshot{
			PluginID:          "weather",
			Valid:             true,
			RegistrationState: "installed",
			DesiredState:      "enabled",
			RuntimeState:      "starting",
			DisplayState:      "enabling",
		},
	}
	router := chi.NewRouter()
	RegisterPluginRoutes(router, catalog, nil, nil, nil, controller, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/plugins/weather/recover", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	var resp pluginDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Plugin.State != "starting" {
		t.Fatalf("state = %q, want starting", resp.Plugin.State)
	}
	if resp.Plugin.StateDiagnosis != nil {
		t.Fatalf("state_diagnosis should be cleared, got %+v", resp.Plugin.StateDiagnosis)
	}
}

// TestRecoverFromDeadLetterHandler_NotRecoverable verifies the recover endpoint
// returns 409 plugin.not_recoverable when the runtime is not recoverable.
func TestRecoverFromDeadLetterHandler_NotRecoverable(t *testing.T) {
	t.Parallel()

	catalog := newTestCatalog([]plugins.Snapshot{{
		PluginID:          "weather",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
		DisplayState:      "running",
	}})
	controller := &stubDesiredStateController{
		recoverErr: plugins.ErrPluginNotInDeadLetter,
	}
	router := chi.NewRouter()
	RegisterPluginRoutes(router, catalog, nil, nil, nil, controller, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/plugins/weather/recover", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body = %s", rec.Code, rec.Body.String())
	}
	env := decodeErrorEnvelope(t, rec.Body.Bytes())
	if env.Error.Code != "plugin.not_recoverable" {
		t.Fatalf("error.code = %q, want plugin.not_recoverable", env.Error.Code)
	}
	if env.Error.MessageKey != "errors.plugin.not_recoverable" {
		t.Fatalf("error.message_key = %q, want errors.plugin.not_recoverable", env.Error.MessageKey)
	}
	if env.Error.Details["plugin_id"] != "weather" {
		t.Fatalf("details.plugin_id = %#v, want weather", env.Error.Details["plugin_id"])
	}
}

// TestRecoverFromDeadLetterHandler_NotFound verifies 404 when the plugin
// does not exist.
func TestRecoverFromDeadLetterHandler_NotFound(t *testing.T) {
	t.Parallel()

	catalog := newTestCatalog(nil)
	controller := &stubDesiredStateController{
		recoverErr: plugins.ErrPluginNotFound,
	}
	router := chi.NewRouter()
	RegisterPluginRoutes(router, catalog, nil, nil, nil, controller, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/plugins/missing/recover", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body = %s", rec.Code, rec.Body.String())
	}
}
