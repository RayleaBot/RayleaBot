package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
	"github.com/go-chi/chi/v5"
	"pgregory.net/rapid"
)

// --- helpers ---

type stubDesiredStateRepository struct {
	saved map[string]string
}

func (r *stubDesiredStateRepository) LoadDesiredStates(context.Context) (map[string]string, error) {
	if r == nil {
		return nil, nil
	}
	return r.saved, nil
}

func (r *stubDesiredStateRepository) SaveDesiredState(_ context.Context, pluginID string, desiredState string, _ time.Time) error {
	if r.saved == nil {
		r.saved = make(map[string]string)
	}
	r.saved[pluginID] = desiredState
	return nil
}

func (r *stubDesiredStateRepository) DeleteDesiredState(_ context.Context, _ string) error {
	return nil
}

func setupRouter(entries []Snapshot) (chi.Router, *Catalog, *tasks.Registry, *stubDesiredStateRepository) {
	catalog := NewCatalog(entries)
	taskRegistry := tasks.NewRegistry()
	repo := &stubDesiredStateRepository{}
	router := chi.NewRouter()
	router.Post("/api/plugins/install", newInstallHandler(catalog, taskRegistry, nil))
	router.Post("/api/plugins/{plugin_id}/enable", newEnableHandler(catalog, repo, nil, nil, nil))
	router.Post("/api/plugins/{plugin_id}/disable", newDisableHandler(catalog, repo, nil, nil, nil))
	return router, catalog, taskRegistry, repo
}

type stubDesiredStateController struct {
	enableResult  Snapshot
	enableErr     error
	disableResult Snapshot
	disableErr    error
	reloadResult  Snapshot
	reloadErr     error
	recoverResult Snapshot
	recoverErr    error
}

func (s *stubDesiredStateController) Enable(_ context.Context, _ string) (Snapshot, error) {
	return s.enableResult, s.enableErr
}

func (s *stubDesiredStateController) Disable(_ context.Context, _ string) (Snapshot, error) {
	return s.disableResult, s.disableErr
}

func (s *stubDesiredStateController) Reload(_ context.Context, _ string) (Snapshot, error) {
	return s.reloadResult, s.reloadErr
}

func (s *stubDesiredStateController) RecoverFromDeadLetter(_ context.Context, _ string) (Snapshot, error) {
	return s.recoverResult, s.recoverErr
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

func TestListHandler_ReturnsPluginMetadata(t *testing.T) {
	t.Parallel()

	catalog := NewCatalog([]Snapshot{{
		PluginID:          "weather",
		Name:              "Weather",
		Version:           "1.2.3",
		Description:       "Weather query plugin",
		Author:            "raylea",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
	}})
	router := chi.NewRouter()
	router.Get("/api/plugins", newListHandler(catalog))

	req := httptest.NewRequest(http.MethodGet, "/api/plugins", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	var resp pluginListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(resp.Items))
	}
	if got := resp.Items[0].Version; got != "1.2.3" {
		t.Fatalf("version = %q, want 1.2.3", got)
	}
	if got := resp.Items[0].Description; got != "Weather query plugin" {
		t.Fatalf("description = %q, want Weather query plugin", got)
	}
	if got := resp.Items[0].Author; got != "raylea" {
		t.Fatalf("author = %q, want raylea", got)
	}
}

// --- Property-Based Tests ---

// Feature: plugin-write-api, Property 1: 安装任务创建 round-trip
// Validates: Requirements 1.1, 1.2, 6.1, 6.2, 6.3
func TestProperty_InstallCreatesQueryableTask(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		sourceType := rapid.SampledFrom([]string{"local_zip", "local_directory"}).Draw(t, "sourceType")
		source := rapid.StringMatching("[a-zA-Z0-9/_\\\\.:]{1,100}").Draw(t, "source")

		router, _, taskRegistry, _ := setupRouter(nil)

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
		router, _, taskRegistry, _ := setupRouter(nil)
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
		repo := &stubDesiredStateRepository{}
		router := chi.NewRouter()
		router.Post("/api/plugins/install", newInstallHandler(catalog, taskRegistry, nil))
		router.Post("/api/plugins/{plugin_id}/enable", newEnableHandler(catalog, repo, nil, nil, nil))
		router.Post("/api/plugins/{plugin_id}/disable", newDisableHandler(catalog, repo, nil, nil, nil))

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
	router, _, taskRegistry, _ := setupRouter(nil)

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

func TestInstallHandler_AllowsExplicitInstallScriptAuthorization(t *testing.T) {
	router, _, taskRegistry, _ := setupRouter(nil)

	body, _ := json.Marshal(pluginInstallRequest{
		SourceType:          "local_directory",
		Source:              "C:/plugins/weather",
		AllowInstallScripts: true,
	})
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

	snap, ok := taskRegistry.Get(resp.TaskID)
	if !ok {
		t.Fatalf("task %q not in registry", resp.TaskID)
	}
	if snap.TaskType != "plugin.install" {
		t.Fatalf("task_type = %q, want plugin.install", snap.TaskType)
	}
}

// TestEnableHandler_Success: enable a disabled+installed plugin returns 200.
// Reproduces fixture ok.plugins-enable-response.yaml.
func TestEnableHandler_Success(t *testing.T) {
	router, _, _, repo := setupRouter([]Snapshot{{
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
	if repo.saved["weather"] != "enabled" {
		t.Fatalf("persisted desired_state = %q, want enabled", repo.saved["weather"])
	}
}

// TestDisableHandler_RuntimeStillStopping: disable an enabled plugin returns 200.
// runtime_state may still be "stopping". Reproduces fixture edge.plugins-disable-response.yaml.
func TestDisableHandler_RuntimeStillStopping(t *testing.T) {
	router, _, _, repo := setupRouter([]Snapshot{{
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
	if repo.saved["weather"] != "disabled" {
		t.Fatalf("persisted desired_state = %q, want disabled", repo.saved["weather"])
	}
}

func TestEnableHandler_ReturnsPermissionPendingForScopeChange(t *testing.T) {
	t.Parallel()

	catalog := NewCatalog([]Snapshot{{
		PluginID:          "weather",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "disabled",
		RuntimeState:      "stopped",
	}})
	controller := &stubDesiredStateController{
		enableErr: &PermissionPendingError{
			PluginID:     "weather",
			ScopeChanged: true,
		},
	}
	router := chi.NewRouter()
	RegisterRoutes(router, catalog, nil, nil, nil, controller, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/plugins/weather/enable", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body = %s", rec.Code, rec.Body.String())
	}

	env := decodeErrorEnvelope(t, rec.Body.Bytes())
	if env.Error.Code != "plugin.permission_pending" {
		t.Fatalf("error.code = %q, want plugin.permission_pending", env.Error.Code)
	}
	if env.Error.MessageKey != "errors.plugin.permission_pending" {
		t.Fatalf("error.message_key = %q, want errors.plugin.permission_pending", env.Error.MessageKey)
	}
	details := env.Error.Details
	if details["plugin_id"] != "weather" {
		t.Fatalf("details.plugin_id = %#v, want weather", details["plugin_id"])
	}
	if details["scope_changed"] != true {
		t.Fatalf("details.scope_changed = %#v, want true", details["scope_changed"])
	}
	if _, ok := details["missing_capabilities"]; ok {
		t.Fatalf("unexpected missing_capabilities: %#v", details["missing_capabilities"])
	}
}

func TestDetailHandler_ReturnsPermissionSummaries(t *testing.T) {
	t.Parallel()

	repo := &stubGrantRepository{
		grants: map[string][]PluginGrant{
			"weather": {{
				PluginID:   "weather",
				Capability: "logger.write",
				GrantedAt:  time.Now().UTC(),
			}},
		},
	}
	catalog := NewCatalog([]Snapshot{{
		PluginID:            "weather",
		Name:                "Weather",
		Valid:               true,
		RegistrationState:   "installed",
		DesiredState:        "enabled",
		RuntimeState:        "running",
		OptionalPermissions: []string{"logger.write"},
		RequiredPermissions: []string{"http.request"},
	}})
	router := chi.NewRouter()
	router.Get("/api/plugins/{plugin_id}", newDetailHandler(catalog, repo, func() []string {
		return []string{"http.request"}
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/plugins/weather", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	var resp pluginDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Plugin.Permissions) != 2 {
		t.Fatalf("len(permissions) = %d, want 2", len(resp.Plugin.Permissions))
	}
	if resp.Plugin.Permissions[0].Capability != "http.request" || resp.Plugin.Permissions[0].Source != string(PermissionSourceConfigAuto) {
		t.Fatalf("unexpected first permission: %#v", resp.Plugin.Permissions[0])
	}
	if resp.Plugin.Permissions[1].Capability != "logger.write" || resp.Plugin.Permissions[1].Source != string(PermissionSourcePersisted) {
		t.Fatalf("unexpected second permission: %#v", resp.Plugin.Permissions[1])
	}
}

func TestDetailHandler_ReturnsBuiltinAutoPermissions(t *testing.T) {
	t.Parallel()

	catalog := NewCatalog([]Snapshot{{
		PluginID:            "raylea.echo",
		Name:                "Echo",
		Valid:               true,
		SourceRoot:          "plugins/builtin",
		RegistrationState:   "installed",
		DesiredState:        "enabled",
		RuntimeState:        "running",
		RequiredPermissions: []string{"message.send"},
	}})
	router := chi.NewRouter()
	router.Get("/api/plugins/{plugin_id}", newDetailHandler(catalog, nil, nil))

	req := httptest.NewRequest(http.MethodGet, "/api/plugins/raylea.echo", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	var resp pluginDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Plugin.Permissions) != 1 {
		t.Fatalf("len(permissions) = %d, want 1", len(resp.Plugin.Permissions))
	}
	for _, permission := range resp.Plugin.Permissions {
		if permission.Source != string(PermissionSourceBuiltinAuto) {
			t.Fatalf("permission source = %q, want %q", permission.Source, PermissionSourceBuiltinAuto)
		}
		if permission.Status != string(PermissionStatusGranted) {
			t.Fatalf("permission status = %q, want %q", permission.Status, PermissionStatusGranted)
		}
	}
}

func TestDetailHandlerReturnsHelpProjection(t *testing.T) {
	t.Parallel()

	catalog := NewCatalog([]Snapshot{{
		PluginID:          "weather",
		Name:              "Weather",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
		Help: &Help{
			Title:   "Weather",
			Summary: "天气菜单",
			Groups: []HelpGroup{{
				Title: "查询",
				Items: []HelpItem{{
					Title:       "城市天气",
					Description: "查询城市天气",
					Usage:       "/weather 上海",
					Command:     "weather",
					Permission:  "everyone",
				}},
			}},
		},
	}})
	router := chi.NewRouter()
	router.Get("/api/plugins/{plugin_id}", newDetailHandler(catalog, nil, nil))

	req := httptest.NewRequest(http.MethodGet, "/api/plugins/weather", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	var resp pluginDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Plugin.Help.Title != "Weather" {
		t.Fatalf("unexpected help projection: %#v", resp.Plugin.Help)
	}
	if len(resp.Plugin.Help.Groups) != 1 || resp.Plugin.Help.Groups[0].Title != "查询" {
		t.Fatalf("unexpected help groups: %#v", resp.Plugin.Help.Groups)
	}
	if got := resp.Plugin.Help.Groups[0].Items[0]; got.Command != "weather" || got.Title != "城市天气" {
		t.Fatalf("unexpected help item: %#v", got)
	}
}

func TestDetailHandler_ReturnsManagementUI(t *testing.T) {
	t.Parallel()

	catalog := NewCatalog([]Snapshot{{
		PluginID:          "example-config-panel",
		Name:              "Example Config Panel",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "disabled",
		RuntimeState:      "stopped",
		DisplayState:      "disabled",
		ManagementUI: &ManagementUI{
			Pages: []ManagementUIPage{
				{ID: "config", Label: "配置页面", Entry: "web/index.html"},
				{ID: "secrets", Label: "密钥设置", Entry: "web/secrets.html"},
			},
		},
	}})
	router := chi.NewRouter()
	router.Get("/api/plugins/{plugin_id}", newDetailHandler(catalog, nil, nil))

	req := httptest.NewRequest(http.MethodGet, "/api/plugins/example-config-panel", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	var resp pluginDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Plugin.ManagementUI == nil {
		t.Fatal("expected management_ui in detail response")
	}
	if len(resp.Plugin.ManagementUI.Pages) != 2 {
		t.Fatalf("management_ui.pages length = %d, want 2", len(resp.Plugin.ManagementUI.Pages))
	}
	if got := resp.Plugin.ManagementUI.Pages[1]; got.ID != "secrets" || got.Label != "密钥设置" || got.Entry != "web/secrets.html" {
		t.Fatalf("unexpected management_ui page: %#v", got)
	}
}

func TestDetailHandler_ReturnsRenderTemplates(t *testing.T) {
	t.Parallel()

	catalog := NewCatalog([]Snapshot{{
		PluginID:          "weather-card",
		Name:              "Weather Card",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "disabled",
		RuntimeState:      "stopped",
		DisplayState:      "disabled",
		RenderTemplates:   []RenderTemplate{{Path: "templates/card"}},
	}})
	router := chi.NewRouter()
	router.Get("/api/plugins/{plugin_id}", newDetailHandler(catalog, nil, nil))

	req := httptest.NewRequest(http.MethodGet, "/api/plugins/weather-card", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	var resp pluginDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Plugin.RenderTemplates) != 1 || resp.Plugin.RenderTemplates[0].Path != "templates/card" {
		t.Fatalf("render_templates = %#v, want templates/card", resp.Plugin.RenderTemplates)
	}
}

// TestEnableHandler_AlreadyEnabled_409: enable already-enabled plugin returns 409.
func TestEnableHandler_AlreadyEnabled_409(t *testing.T) {
	router, _, _, repo := setupRouter([]Snapshot{{
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
		t.Fatal("state conflict should not persist desired_state")
	}
}

// TestDisableHandler_AlreadyDisabled_409: disable already-disabled plugin returns 409.
func TestDisableHandler_AlreadyDisabled_409(t *testing.T) {
	router, _, _, repo := setupRouter([]Snapshot{{
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
		t.Fatal("state conflict should not persist desired_state")
	}
}

// TestEnableHandler_RemovedPlugin_409: enable plugin with registration_state=removed returns 409.
func TestEnableHandler_RemovedPlugin_409(t *testing.T) {
	router, _, _, repo := setupRouter([]Snapshot{{
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
		t.Fatal("removed plugin should not persist desired_state")
	}
}

// TestInstallHandler_EmptySource_400: source="" returns 400.
func TestInstallHandler_EmptySource_400(t *testing.T) {
	router, _, _, _ := setupRouter(nil)

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
	router, _, _, _ := setupRouter(nil)

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

// --- Grants scope validation tests ---

type stubGrantRepository struct {
	grants map[string][]PluginGrant
}

func (r *stubGrantRepository) LoadGrants(_ context.Context, pluginID string) ([]PluginGrant, error) {
	now := time.Now().UTC()
	var active []PluginGrant
	for _, grant := range r.grants[pluginID] {
		if grant.ExpiresAt != nil && !grant.ExpiresAt.After(now) {
			continue
		}
		active = append(active, grant)
	}
	return active, nil
}

func (r *stubGrantRepository) LoadAllGrants(_ context.Context) (map[string][]string, error) {
	result := make(map[string][]string)
	for pid := range r.grants {
		gs, _ := r.LoadGrants(context.Background(), pid)
		for _, g := range gs {
			result[pid] = append(result[pid], g.Capability)
		}
	}
	return result, nil
}

func (r *stubGrantRepository) SaveGrant(_ context.Context, grant PluginGrant) error {
	if r.grants == nil {
		r.grants = make(map[string][]PluginGrant)
	}
	items := r.grants[grant.PluginID]
	for i, existing := range items {
		if existing.Capability == grant.Capability {
			items[i] = grant
			r.grants[grant.PluginID] = items
			return nil
		}
	}
	r.grants[grant.PluginID] = append(items, grant)
	return nil
}

func (r *stubGrantRepository) DeleteGrant(_ context.Context, pluginID, capability string) error {
	gs := r.grants[pluginID]
	for i, g := range gs {
		if g.Capability == capability {
			r.grants[pluginID] = append(gs[:i], gs[i+1:]...)
			break
		}
	}
	return nil
}

func (r *stubGrantRepository) DeleteAllGrants(_ context.Context, pluginID string) error {
	delete(r.grants, pluginID)
	return nil
}

func grantsRouter(entries []Snapshot, grantRepo GrantRepository) chi.Router {
	return grantsRouterWithAutoGrants(entries, grantRepo, nil)
}

func grantsRouterWithAutoGrants(entries []Snapshot, grantRepo GrantRepository, autoGrants []string) chi.Router {
	catalog := NewCatalog(entries)
	router := chi.NewRouter()
	RegisterRoutes(router, catalog, nil, nil, nil, nil, nil, grantRepo, func() []string {
		return append([]string(nil), autoGrants...)
	})
	return router
}

func TestGrantHandler_ValidCapability(t *testing.T) {
	t.Parallel()

	repo := &stubGrantRepository{}
	router := grantsRouter([]Snapshot{{
		PluginID:             "weather",
		Valid:                true,
		RegistrationState:    "installed",
		DesiredState:         "disabled",
		RuntimeState:         "stopped",
		DeclaredCapabilities: []string{"event.subscribe", "http.request"},
		RequiredPermissions:  []string{"http.request"},
		OptionalPermissions:  []string{"event.subscribe"},
	}}, repo)

	body, _ := json.Marshal(grantRequest{Capability: "http.request"})
	req := httptest.NewRequest(http.MethodPost, "/api/plugins/weather/grants", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	var resp grantResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Capability != "http.request" {
		t.Fatalf("capability = %q, want http.request", resp.Capability)
	}
	if resp.Source != string(GrantSourcePersisted) {
		t.Fatalf("source = %q, want %q", resp.Source, GrantSourcePersisted)
	}
	if resp.GrantedAt == nil || *resp.GrantedAt == "" {
		t.Fatalf("granted_at = %#v, want populated timestamp", resp.GrantedAt)
	}
	if resp.ExpiresAt != nil {
		t.Fatalf("expires_at = %v, want nil for permanent grant", *resp.ExpiresAt)
	}
}

func TestGrantHandler_AcceptsMultiSegmentCapability(t *testing.T) {
	t.Parallel()

	repo := &stubGrantRepository{}
	router := grantsRouter([]Snapshot{{
		PluginID:             "weather",
		Valid:                true,
		RegistrationState:    "installed",
		DesiredState:         "disabled",
		RuntimeState:         "stopped",
		DeclaredCapabilities: []string{"message.history.get"},
		RequiredPermissions:  []string{"message.history.get"},
	}}, repo)

	body, _ := json.Marshal(grantRequest{Capability: "message.history.get"})
	req := httptest.NewRequest(http.MethodPost, "/api/plugins/weather/grants", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
}

func TestGrantHandler_AcceptsProviderCapability(t *testing.T) {
	t.Parallel()

	repo := &stubGrantRepository{}
	router := grantsRouter([]Snapshot{{
		PluginID:            "weather",
		Valid:               true,
		RegistrationState:   "installed",
		DesiredState:        "disabled",
		RuntimeState:        "stopped",
		OptionalPermissions: []string{"provider.napcat.group.sign.set"},
	}}, repo)

	body, _ := json.Marshal(grantRequest{Capability: "provider.napcat.group.sign.set"})
	req := httptest.NewRequest(http.MethodPost, "/api/plugins/weather/grants", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
}

func TestGrantHandler_RejectsInvalidCapabilityFormat(t *testing.T) {
	t.Parallel()

	repo := &stubGrantRepository{}
	router := grantsRouter([]Snapshot{{
		PluginID:             "weather",
		Valid:                true,
		RegistrationState:    "installed",
		DesiredState:         "disabled",
		RuntimeState:         "stopped",
		DeclaredCapabilities: []string{"event.subscribe"},
	}}, repo)

	for _, badCap := range []string{"INVALID", "no-dot", "has.Upper", "has.num3rs", "123.abc"} {
		body, _ := json.Marshal(grantRequest{Capability: badCap})
		req := httptest.NewRequest(http.MethodPost, "/api/plugins/weather/grants", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("capability %q: status = %d, want 400", badCap, rec.Code)
		}
	}
}

func TestGrantHandler_RejectsUndeclaredCapability(t *testing.T) {
	t.Parallel()

	repo := &stubGrantRepository{}
	router := grantsRouter([]Snapshot{{
		PluginID:             "weather",
		Valid:                true,
		RegistrationState:    "installed",
		DesiredState:         "disabled",
		RuntimeState:         "stopped",
		DeclaredCapabilities: []string{"event.subscribe"},
		RequiredPermissions:  []string{"event.subscribe"},
	}}, repo)

	// http.request is valid format but not declared in this plugin's manifest.
	body, _ := json.Marshal(grantRequest{Capability: "http.request"})
	req := httptest.NewRequest(http.MethodPost, "/api/plugins/weather/grants", bytes.NewReader(body))
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

func TestGrantHandler_AcceptsOptionalPermission(t *testing.T) {
	t.Parallel()

	repo := &stubGrantRepository{}
	router := grantsRouter([]Snapshot{{
		PluginID:            "weather",
		Valid:               true,
		RegistrationState:   "installed",
		DesiredState:        "disabled",
		RuntimeState:        "stopped",
		OptionalPermissions: []string{"logger.write"},
	}}, repo)

	body, _ := json.Marshal(grantRequest{Capability: "logger.write"})
	req := httptest.NewRequest(http.MethodPost, "/api/plugins/weather/grants", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
}

func TestGrantHandler_PluginNotFound(t *testing.T) {
	t.Parallel()

	repo := &stubGrantRepository{}
	router := grantsRouter(nil, repo)

	body, _ := json.Marshal(grantRequest{Capability: "http.request"})
	req := httptest.NewRequest(http.MethodPost, "/api/plugins/nonexistent/grants", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestGrantHandler_AcceptsFutureExpiry(t *testing.T) {
	t.Parallel()

	repo := &stubGrantRepository{}
	router := grantsRouter([]Snapshot{{
		PluginID:             "weather",
		Valid:                true,
		RegistrationState:    "installed",
		DesiredState:         "disabled",
		RuntimeState:         "stopped",
		DeclaredCapabilities: []string{"logger.write"},
		OptionalPermissions:  []string{"logger.write"},
	}}, repo)

	expiresAt := time.Now().UTC().Add(2 * time.Hour).Format(time.RFC3339)
	body, _ := json.Marshal(grantRequest{Capability: "logger.write", ExpiresAt: &expiresAt})
	req := httptest.NewRequest(http.MethodPost, "/api/plugins/weather/grants", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	var resp grantResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ExpiresAt == nil || *resp.ExpiresAt != expiresAt {
		t.Fatalf("expires_at = %#v, want %q", resp.ExpiresAt, expiresAt)
	}
	if resp.Source != string(GrantSourcePersisted) {
		t.Fatalf("source = %q, want %q", resp.Source, GrantSourcePersisted)
	}
}

func TestGrantHandler_RejectsInvalidExpiry(t *testing.T) {
	t.Parallel()

	repo := &stubGrantRepository{}
	router := grantsRouter([]Snapshot{{
		PluginID:             "weather",
		Valid:                true,
		RegistrationState:    "installed",
		DesiredState:         "disabled",
		RuntimeState:         "stopped",
		DeclaredCapabilities: []string{"http.request"},
		RequiredPermissions:  []string{"http.request"},
	}}, repo)

	expiresAt := "2026-03-22T10:00:00+08:00"
	body, _ := json.Marshal(grantRequest{Capability: "http.request", ExpiresAt: &expiresAt})
	req := httptest.NewRequest(http.MethodPost, "/api/plugins/weather/grants", bytes.NewReader(body))
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

func TestListGrantsHandler_OmitsExpiredGrant(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)
	repo := &stubGrantRepository{
		grants: map[string][]PluginGrant{
			"weather": {
				{
					PluginID:   "weather",
					Capability: "http.request",
					GrantedAt:  now,
				},
				{
					PluginID:   "weather",
					Capability: "logger.write",
					GrantedAt:  now,
					ExpiresAt:  &future,
				},
				{
					PluginID:   "weather",
					Capability: "storage.file",
					GrantedAt:  now,
					ExpiresAt:  &past,
				},
			},
		},
	}
	router := grantsRouter([]Snapshot{{
		PluginID:          "weather",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "disabled",
		RuntimeState:      "stopped",
	}}, repo)

	req := httptest.NewRequest(http.MethodGet, "/api/plugins/weather/grants", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	var resp grantsListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(resp.Items))
	}
	if resp.Items[0].Source != string(GrantSourcePersisted) || resp.Items[0].GrantedAt == nil {
		t.Fatalf("unexpected first grant: %#v", resp.Items[0])
	}
	if resp.Items[1].ExpiresAt == nil {
		t.Fatalf("expires_at = nil, want populated expiry")
	}
}

func TestListGrantsHandler_ReturnsEffectiveGrantSources(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	repo := &stubGrantRepository{
		grants: map[string][]PluginGrant{
			"weather": {{
				PluginID:   "weather",
				Capability: "logger.write",
				GrantedAt:  now,
			}},
		},
	}
	router := grantsRouterWithAutoGrants([]Snapshot{{
		PluginID:            "weather",
		Valid:               true,
		RegistrationState:   "installed",
		DesiredState:        "enabled",
		RuntimeState:        "running",
		RequiredPermissions: []string{"scheduler.create"},
		OptionalPermissions: []string{"logger.write"},
	}}, repo, []string{"scheduler.create"})

	req := httptest.NewRequest(http.MethodGet, "/api/plugins/weather/grants", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	var resp grantsListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(resp.Items))
	}
	if resp.Items[0].Capability != "logger.write" || resp.Items[0].Source != string(GrantSourcePersisted) {
		t.Fatalf("unexpected persisted grant: %#v", resp.Items[0])
	}
	if resp.Items[1].Capability != "scheduler.create" || resp.Items[1].Source != string(GrantSourceConfigAuto) {
		t.Fatalf("unexpected config auto grant: %#v", resp.Items[1])
	}
	if resp.Items[1].GrantedAt != nil {
		t.Fatalf("config auto granted_at = %#v, want nil", resp.Items[1].GrantedAt)
	}
}

func TestListGrantsHandler_ReturnsBuiltinAutoGrant(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name                string
		pluginID            string
		requiredPermissions []string
		wantCapabilities    []string
	}{
		{
			name:                "echo",
			pluginID:            "raylea.echo",
			requiredPermissions: []string{"message.send"},
			wantCapabilities:    []string{"message.send"},
		},
		{
			name:                "fortune",
			pluginID:            "raylea.fortune",
			requiredPermissions: []string{"message.send", "render.image", "storage.kv"},
			wantCapabilities:    []string{"message.send", "render.image", "storage.kv"},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			router := grantsRouter([]Snapshot{{
				PluginID:            tc.pluginID,
				Valid:               true,
				SourceRoot:          "plugins/builtin",
				RegistrationState:   "installed",
				DesiredState:        "enabled",
				RuntimeState:        "running",
				RequiredPermissions: tc.requiredPermissions,
			}}, nil)

			req := httptest.NewRequest(http.MethodGet, "/api/plugins/"+tc.pluginID+"/grants", nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
			}

			var resp grantsListResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode: %v", err)
			}

			gotCapabilities := make([]string, 0, len(resp.Items))
			for _, item := range resp.Items {
				if item.Source != string(GrantSourceBuiltinAuto) {
					t.Fatalf("unexpected builtin auto grant source: %#v", item)
				}
				if item.GrantedAt != nil || item.ExpiresAt != nil {
					t.Fatalf("builtin auto timestamps should be nil: %#v", item)
				}
				gotCapabilities = append(gotCapabilities, item.Capability)
			}
			if !reflect.DeepEqual(gotCapabilities, tc.wantCapabilities) {
				t.Fatalf("builtin auto capabilities = %#v, want %#v", gotCapabilities, tc.wantCapabilities)
			}
		})
	}
}

// TestRecoverFromDeadLetterHandler_Success verifies the recover endpoint
// returns the plugin detail snapshot when the controller succeeds.
// Reproduces fixture ok.plugins-dead-letter-recover-response.yaml.
func TestRecoverFromDeadLetterHandler_Success(t *testing.T) {
	t.Parallel()

	catalog := NewCatalog([]Snapshot{{
		PluginID:          "weather",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "dead_letter",
		DisplayState:      "dead_letter",
	}})
	controller := &stubDesiredStateController{
		recoverResult: Snapshot{
			PluginID:          "weather",
			Valid:             true,
			RegistrationState: "installed",
			DesiredState:      "enabled",
			RuntimeState:      "starting",
			DisplayState:      "enabling",
		},
	}
	router := chi.NewRouter()
	RegisterRoutes(router, catalog, nil, nil, nil, controller, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/plugins/weather/dead_letter/recover", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	var resp pluginDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Plugin.RuntimeState != "starting" {
		t.Fatalf("runtime_state = %q, want starting", resp.Plugin.RuntimeState)
	}
	if resp.Plugin.DesiredState != "enabled" {
		t.Fatalf("desired_state = %q, want enabled", resp.Plugin.DesiredState)
	}
	if resp.Plugin.DeadLetter != nil {
		t.Fatalf("dead_letter should be cleared, got %+v", resp.Plugin.DeadLetter)
	}
}

// TestRecoverFromDeadLetterHandler_NotInDeadLetter verifies the recover
// endpoint returns 409 plugin.not_in_dead_letter when the runtime is not
// currently in dead_letter. Reproduces fixture
// invalid.plugins-dead-letter-recover-not-in-dead-letter.yaml.
func TestRecoverFromDeadLetterHandler_NotInDeadLetter(t *testing.T) {
	t.Parallel()

	catalog := NewCatalog([]Snapshot{{
		PluginID:          "weather",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
		DisplayState:      "running",
	}})
	controller := &stubDesiredStateController{
		recoverErr: ErrPluginNotInDeadLetter,
	}
	router := chi.NewRouter()
	RegisterRoutes(router, catalog, nil, nil, nil, controller, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/plugins/weather/dead_letter/recover", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body = %s", rec.Code, rec.Body.String())
	}
	env := decodeErrorEnvelope(t, rec.Body.Bytes())
	if env.Error.Code != "plugin.not_in_dead_letter" {
		t.Fatalf("error.code = %q, want plugin.not_in_dead_letter", env.Error.Code)
	}
	if env.Error.MessageKey != "errors.plugin.not_in_dead_letter" {
		t.Fatalf("error.message_key = %q, want errors.plugin.not_in_dead_letter", env.Error.MessageKey)
	}
	if env.Error.Details["plugin_id"] != "weather" {
		t.Fatalf("details.plugin_id = %#v, want weather", env.Error.Details["plugin_id"])
	}
}

// TestRecoverFromDeadLetterHandler_NotFound verifies 404 when the plugin
// does not exist.
func TestRecoverFromDeadLetterHandler_NotFound(t *testing.T) {
	t.Parallel()

	catalog := NewCatalog(nil)
	controller := &stubDesiredStateController{
		recoverErr: ErrPluginNotFound,
	}
	router := chi.NewRouter()
	RegisterRoutes(router, catalog, nil, nil, nil, controller, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/plugins/missing/dead_letter/recover", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body = %s", rec.Code, rec.Body.String())
	}
}
