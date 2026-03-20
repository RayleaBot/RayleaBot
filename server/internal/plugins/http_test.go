package plugins

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"pgregory.net/rapid"
	"rayleabot/server/internal/tasks"
)

// --- helpers ---

func setupRouter(entries []Snapshot) (chi.Router, *Catalog, *tasks.Registry) {
	catalog := NewCatalog(entries)
	taskRegistry := tasks.NewRegistry()
	router := chi.NewRouter()
	router.Post("/api/plugins/install", newInstallHandler(catalog, taskRegistry))
	router.Post("/api/plugins/{plugin_id}/enable", newEnableHandler(catalog))
	router.Post("/api/plugins/{plugin_id}/disable", newDisableHandler(catalog))
	return router, catalog, taskRegistry
}

type fataler interface {
	Fatalf(format string, args ...any)
}

func decodeErrorEnvelope(t fataler, body []byte) errorEnvelope {
	var env errorEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("failed to decode error envelope: %v\nbody: %s", err, body)
	}
	return env
}

// --- Property-Based Tests ---

// Feature: plugin-write-api, Property 1: 安装任务创建 round-trip
// Validates: Requirements 1.1, 1.2, 6.1, 6.2, 6.3
func TestProperty_InstallCreatesQueryableTask(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		sourceType := rapid.SampledFrom([]string{"local_zip", "local_directory"}).Draw(t, "sourceType")
		source := rapid.StringMatching("[a-zA-Z0-9/_\\\\.:]{1,100}").Draw(t, "source")

		router, _, taskRegistry := setupRouter(nil)

		reqBody, _ := json.Marshal(pluginInstallRequest{SourceType: sourceType, Source: source})
		req := httptest.NewRequest(http.MethodPost, "/api/plugins/install", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusAccepted {
			t.Fatalf("status = %d, want 202; body = %s", rec.Code, rec.Body.String())
		}

		var resp taskAcceptedResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if resp.TaskID == "" {
			t.Fatal("task_id is empty")
		}

		snap, ok := taskRegistry.Get(resp.TaskID)
		if !ok {
			t.Fatalf("task %q not found in registry", resp.TaskID)
		}
		if snap.TaskType != "plugin.install" {
			t.Fatalf("task_type = %q, want %q", snap.TaskType, "plugin.install")
		}
		if snap.Status != tasks.StatusPending {
			t.Fatalf("status = %q, want %q", snap.Status, tasks.StatusPending)
		}
	})
}

// Feature: plugin-write-api, Property 2: 无效安装请求被拒绝
// Validates: Requirements 1.3, 1.4, 1.5
func TestProperty_InvalidInstallRequestRejected(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		router, _, taskRegistry := setupRouter(nil)
		tasksBefore := len(taskRegistry.List())

		// Generate one of several invalid request variants.
		variant := rapid.IntRange(0, 3).Draw(t, "variant")
		var body string
		switch variant {
		case 0: // missing source_type
			src := rapid.StringMatching("[a-zA-Z0-9/_]{1,50}").Draw(t, "source")
			body = `{"source":"` + src + `"}`
		case 1: // missing source
			st := rapid.SampledFrom([]string{"local_zip", "local_directory"}).Draw(t, "sourceType")
			body = `{"source_type":"` + st + `"}`
		case 2: // invalid source_type
			badType := rapid.StringMatching("[a-z]{3,15}").
				Filter(func(s string) bool { return s != "local_zip" && s != "local_directory" }).
				Draw(t, "badType")
			src := rapid.StringMatching("[a-zA-Z0-9/_]{1,50}").Draw(t, "source")
			b, _ := json.Marshal(pluginInstallRequest{SourceType: badType, Source: src})
			body = string(b)
		case 3: // empty source
			st := rapid.SampledFrom([]string{"local_zip", "local_directory"}).Draw(t, "sourceType")
			b, _ := json.Marshal(pluginInstallRequest{SourceType: st, Source: ""})
			body = string(b)
		}

		req := httptest.NewRequest(http.MethodPost, "/api/plugins/install", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("variant=%d status = %d, want 400; body = %s", variant, rec.Code, rec.Body.String())
		}

		env := decodeErrorEnvelope(t, rec.Body.Bytes())
		if env.Error.Code != codeInvalidRequest {
			t.Fatalf("error.code = %q, want %q", env.Error.Code, codeInvalidRequest)
		}

		tasksAfter := len(taskRegistry.List())
		if tasksAfter != tasksBefore {
			t.Fatalf("tasks count changed from %d to %d; no task should be created for invalid request", tasksBefore, tasksAfter)
		}
	})
}

// Feature: plugin-write-api, Property 5: 不存在的插件返回 404
// Validates: Requirements 2.4, 3.4
func TestProperty_NonExistentPluginReturns404(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		pluginID := rapid.StringMatching("[a-z][a-z0-9_]{2,30}").Draw(t, "pluginID")

		// Empty catalog — no plugins exist.
		router, _, _ := setupRouter(nil)

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

// Feature: plugin-write-api, Property 8: 错误响应 schema 一致性
// Validates: Requirements 4.1, 4.4
func TestProperty_ErrorResponseSchemaConsistency(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Build a catalog with one installed+enabled plugin for 409 scenarios.
		catalog := NewCatalog([]Snapshot{{
			PluginID:          "existing",
			Name:              "Existing Plugin",
			Version:           "1.0.0",
			RegistrationState: "installed",
			DesiredState:      "enabled",
			RuntimeState:      "running",
			Valid:             true,
		}})
		taskRegistry := tasks.NewRegistry()
		router := chi.NewRouter()
		router.Post("/api/plugins/install", newInstallHandler(catalog, taskRegistry))
		router.Post("/api/plugins/{plugin_id}/enable", newEnableHandler(catalog))
		router.Post("/api/plugins/{plugin_id}/disable", newDisableHandler(catalog))

		// Pick one of several error-triggering scenarios.
		scenario := rapid.IntRange(0, 3).Draw(t, "scenario")
		var req *http.Request
		switch scenario {
		case 0: // 400 — malformed install body
			req = httptest.NewRequest(http.MethodPost, "/api/plugins/install", strings.NewReader(`{invalid`))
			req.Header.Set("Content-Type", "application/json")
		case 1: // 400 — empty source
			b, _ := json.Marshal(pluginInstallRequest{SourceType: "local_zip", Source: ""})
			req = httptest.NewRequest(http.MethodPost, "/api/plugins/install", bytes.NewReader(b))
			req.Header.Set("Content-Type", "application/json")
		case 2: // 404 — non-existent plugin enable
			fakeID := rapid.StringMatching("[a-z]{5,15}").
				Filter(func(s string) bool { return s != "existing" }).
				Draw(t, "fakeID")
			req = httptest.NewRequest(http.MethodPost, "/api/plugins/"+fakeID+"/enable", nil)
		case 3: // 409 — already enabled
			req = httptest.NewRequest(http.MethodPost, "/api/plugins/existing/enable", nil)
		}

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		// Verify Content-Type.
		ct := rec.Header().Get("Content-Type")
		if !strings.HasPrefix(ct, "application/json") {
			t.Fatalf("scenario=%d Content-Type = %q, want application/json", scenario, ct)
		}

		// Verify ErrorEnvelope schema.
		env := decodeErrorEnvelope(t, rec.Body.Bytes())
		if env.Error.Code == "" {
			t.Fatalf("scenario=%d error.code is empty", scenario)
		}
		if env.Error.Message == "" {
			t.Fatalf("scenario=%d error.message is empty", scenario)
		}
		if env.Error.MessageKey == "" {
			t.Fatalf("scenario=%d error.message_key is empty", scenario)
		}
		if env.Error.RequestID == "" {
			t.Fatalf("scenario=%d error.request_id is empty", scenario)
		}
	})
}

// --- Unit Tests ---

// TestInstallHandler_ValidLocalZip: valid install request returns 202 with task_id.
// Reproduces fixture ok.plugins-install-accepted.yaml.
func TestInstallHandler_ValidLocalZip(t *testing.T) {
	router, _, taskRegistry := setupRouter(nil)

	body, _ := json.Marshal(pluginInstallRequest{SourceType: "local_zip", Source: "C:/plugins/weather.zip"})
	req := httptest.NewRequest(http.MethodPost, "/api/plugins/install", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body = %s", rec.Code, rec.Body.String())
	}

	var resp taskAcceptedResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.TaskID == "" {
		t.Fatal("task_id is empty")
	}

	snap, ok := taskRegistry.Get(resp.TaskID)
	if !ok {
		t.Fatalf("task %q not in registry", resp.TaskID)
	}
	if snap.TaskType != "plugin.install" {
		t.Fatalf("task_type = %q, want plugin.install", snap.TaskType)
	}
	if snap.Status != tasks.StatusPending {
		t.Fatalf("status = %q, want pending", snap.Status)
	}
}

// TestEnableHandler_Success: enable a disabled+installed plugin returns 200.
// Reproduces fixture ok.plugins-enable-response.yaml.
func TestEnableHandler_Success(t *testing.T) {
	router, _, _ := setupRouter([]Snapshot{{
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
	if resp.Plugin.DesiredState != "enabled" {
		t.Fatalf("plugin.desired_state = %q, want enabled", resp.Plugin.DesiredState)
	}
}

// TestDisableHandler_RuntimeStillStopping: disable an enabled plugin returns 200.
// runtime_state may still be "stopping". Reproduces fixture edge.plugins-disable-response.yaml.
func TestDisableHandler_RuntimeStillStopping(t *testing.T) {
	router, _, _ := setupRouter([]Snapshot{{
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
	if resp.Plugin.DesiredState != "disabled" {
		t.Fatalf("plugin.desired_state = %q, want disabled", resp.Plugin.DesiredState)
	}
	if resp.Plugin.RegistrationState != "installed" {
		t.Fatalf("plugin.registration_state = %q, want installed", resp.Plugin.RegistrationState)
	}
	// runtime_state may still be "stopping" — that's allowed by the contract.
	if resp.Plugin.RuntimeState != "stopping" {
		t.Fatalf("plugin.runtime_state = %q, want stopping", resp.Plugin.RuntimeState)
	}
}

// TestEnableHandler_AlreadyEnabled_409: enable already-enabled plugin returns 409.
func TestEnableHandler_AlreadyEnabled_409(t *testing.T) {
	router, _, _ := setupRouter([]Snapshot{{
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
}

// TestDisableHandler_AlreadyDisabled_409: disable already-disabled plugin returns 409.
func TestDisableHandler_AlreadyDisabled_409(t *testing.T) {
	router, _, _ := setupRouter([]Snapshot{{
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
}

// TestEnableHandler_RemovedPlugin_409: enable plugin with registration_state=removed returns 409.
func TestEnableHandler_RemovedPlugin_409(t *testing.T) {
	router, _, _ := setupRouter([]Snapshot{{
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
}

// TestInstallHandler_EmptySource_400: source="" returns 400.
func TestInstallHandler_EmptySource_400(t *testing.T) {
	router, _, _ := setupRouter(nil)

	body, _ := json.Marshal(pluginInstallRequest{SourceType: "local_zip", Source: ""})
	req := httptest.NewRequest(http.MethodPost, "/api/plugins/install", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
	}

	env := decodeErrorEnvelope(t, rec.Body.Bytes())
	if env.Error.Code != codeInvalidRequest {
		t.Fatalf("error.code = %q, want %q", env.Error.Code, codeInvalidRequest)
	}
}

// TestInstallHandler_MalformedJSON_400: invalid JSON body returns 400.
func TestInstallHandler_MalformedJSON_400(t *testing.T) {
	router, _, _ := setupRouter(nil)

	req := httptest.NewRequest(http.MethodPost, "/api/plugins/install", strings.NewReader(`{not valid json`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
	}

	env := decodeErrorEnvelope(t, rec.Body.Bytes())
	if env.Error.Code != codeInvalidRequest {
		t.Fatalf("error.code = %q, want %q", env.Error.Code, codeInvalidRequest)
	}
}
