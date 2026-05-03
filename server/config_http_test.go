package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	internalapp "github.com/RayleaBot/RayleaBot/server/internal/app"
	"github.com/RayleaBot/RayleaBot/server/internal/auth"
	internalconfig "github.com/RayleaBot/RayleaBot/server/internal/config"
)

func TestConfigGetReturnsPlaintextOneBotTransportTokens(t *testing.T) {
	t.Parallel()

	application, _, _ := newTestAppWithConfigMutation(t, func(input map[string]any) {
		onebot := input["onebot"].(map[string]any)
		onebot["forward_ws"].(map[string]any)["access_token"] = "forward-secret"
		onebot["reverse_ws"].(map[string]any)["access_token"] = "reverse-secret"
		onebot["http_api"].(map[string]any)["access_token"] = "http-secret"
		onebot["webhook"].(map[string]any)["access_token"] = "webhook-secret"
	}, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	fixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "ok.config-get-response.yaml"))
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	request, err := http.NewRequest(http.MethodGet, server.URL+fixture.Request.Path, nil)
	if err != nil {
		t.Fatalf("create config get request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform config get request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != fixture.Response.Status {
		t.Fatalf("unexpected config get status: got %d want %d", response.StatusCode, fixture.Response.Status)
	}

	body := decodeBody(t, readAll(t, response))
	expected := normalizeJSONMap(t, fixture.Response.Body)
	if !reflect.DeepEqual(body, expected) {
		t.Fatalf("unexpected config get body: got %#v want %#v", body, expected)
	}

	if body["redacted_fields"] != nil {
		t.Fatalf("redacted_fields = %#v, want omitted for OneBot tokens", body["redacted_fields"])
	}
	if got := body["config"].(map[string]any)["onebot"].(map[string]any)["forward_ws"].(map[string]any)["access_token"]; got != "forward-secret" {
		t.Fatalf("config get forward_ws.access_token = %#v, want forward-secret", got)
	}
}

func TestConfigPutWritesValidatedDocumentAndPlaintextTransportTokens(t *testing.T) {
	t.Parallel()

	application, configPath, schemaPath := newTestAppWithConfigMutation(t, func(input map[string]any) {
		input["onebot"].(map[string]any)["forward_ws"].(map[string]any)["access_token"] = "old-forward-secret"
	}, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	fixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "ok.config-update-response.yaml"))
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	updateRequest := normalizeJSONMap(t, fixture.Request.Body)
	updateRequest["onebot"].(map[string]any)["access_token"] = "legacy-secret"
	payload, err := json.Marshal(updateRequest)
	if err != nil {
		t.Fatalf("marshal config update request: %v", err)
	}
	request, err := http.NewRequest(http.MethodPut, server.URL+fixture.Request.Path, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("create config update request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform config update request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != fixture.Response.Status {
		t.Fatalf("unexpected config update status: got %d want %d", response.StatusCode, fixture.Response.Status)
	}

	body := decodeBody(t, readAll(t, response))
	expected := normalizeJSONMap(t, fixture.Response.Body)
	if !reflect.DeepEqual(body, expected) {
		t.Fatalf("unexpected config update body: got %#v want %#v", body, expected)
	}
	if _, ok := body["config"].(map[string]any)["onebot"].(map[string]any)["access_token"]; ok {
		t.Fatal("legacy onebot.access_token should not be returned")
	}

	document, err := internalconfig.LoadDocument(configPath, schemaPath)
	if err != nil {
		t.Fatalf("load persisted config: %v", err)
	}
	if got := document["server"].(map[string]any)["port"]; got != float64(8081) {
		t.Fatalf("unexpected persisted server.port: got %#v want 8081", got)
	}
	if got := document["log"].(map[string]any)["level"]; got != "debug" {
		t.Fatalf("unexpected persisted log.level: got %#v want debug", got)
	}
	onebot := document["onebot"].(map[string]any)
	if _, ok := onebot["access_token"]; ok {
		t.Fatal("legacy onebot.access_token should not be persisted")
	}
	if got := onebot["forward_ws"].(map[string]any)["access_token"]; got != "forward-secret" {
		t.Fatalf("unexpected persisted forward_ws.access_token: got %#v want forward-secret", got)
	}

	if application.CurrentConfig().Server.Port != 8081 {
		t.Fatalf("expected live config server.port to reflect saved value 8081, got %d", application.CurrentConfig().Server.Port)
	}
	if application.CurrentConfig().Logging.Level != "debug" {
		t.Fatalf("expected live config log.level to be hot-reloaded to debug, got %q", application.CurrentConfig().Logging.Level)
	}
}

func TestConfigPutRejectsInvalidConfig(t *testing.T) {
	t.Parallel()

	application, configPath, schemaPath := newTestAppWithConfigMutation(t, func(input map[string]any) {
		input["onebot"].(map[string]any)["forward_ws"].(map[string]any)["access_token"] = "fixture-only-secret"
	}, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	fixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "invalid.config-update-invalid.yaml"))
	before, err := internalconfig.LoadDocument(configPath, schemaPath)
	if err != nil {
		t.Fatalf("load baseline config: %v", err)
	}

	payload, err := json.Marshal(fixture.Request.Body)
	if err != nil {
		t.Fatalf("marshal invalid config update request: %v", err)
	}
	request, err := http.NewRequest(http.MethodPut, "/api/config", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("create invalid config update request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	application.Handler().ServeHTTP(recorder, request)
	if recorder.Code != fixture.Response.Status {
		t.Fatalf("unexpected invalid config update status: got %d want %d", recorder.Code, fixture.Response.Status)
	}

	body := decodeBody(t, recorder.Body.Bytes())
	assertErrorEnvelopeMatchesFixture(t, body, fixture.Response.Body, "platform.invalid_config")
	if strings.Contains(recorder.Body.String(), "fixture-only-secret") {
		t.Fatalf("invalid config response leaked secret content: %s", recorder.Body.String())
	}

	after, err := internalconfig.LoadDocument(configPath, schemaPath)
	if err != nil {
		t.Fatalf("reload config after invalid update: %v", err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("config file changed after invalid update: got %#v want %#v", after, before)
	}
}

func TestConfigPutNormalizesShorthandOneBotURL(t *testing.T) {
	t.Parallel()

	application, configPath, schemaPath := newTestAppWithConfigMutation(t, nil, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	document, err := internalconfig.LoadDocument(configPath, schemaPath)
	if err != nil {
		t.Fatalf("load baseline config: %v", err)
	}
	document["onebot"].(map[string]any)["forward_ws"].(map[string]any)["url"] = "ws:127.0.0.1:2658"
	document["onebot"].(map[string]any)["forward_ws"].(map[string]any)["enabled"] = true

	payload, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("marshal config update request: %v", err)
	}
	request, err := http.NewRequest(http.MethodPut, "/api/config", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("create config update request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	application.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected config update status: got %d want 200", recorder.Code)
	}

	saved, err := internalconfig.LoadDocument(configPath, schemaPath)
	if err != nil {
		t.Fatalf("reload config after update: %v", err)
	}
	forwardWS := saved["onebot"].(map[string]any)["forward_ws"].(map[string]any)
	if got := forwardWS["url"]; got != "ws://127.0.0.1:2658" {
		t.Fatalf("saved onebot.forward_ws.url = %#v, want ws://127.0.0.1:2658", got)
	}
}

func TestConfigPutHotReloadsOneBotTransportStateWithoutRestart(t *testing.T) {
	t.Parallel()

	application, _, _ := newTestAppWithConfigMutation(t, nil, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	payload := map[string]any{
		"schema_version": "2",
		"server": map[string]any{
			"host": "127.0.0.1",
			"port": 8080,
		},
		"onebot": map[string]any{
			"provider": "standard",
			"reverse_ws": map[string]any{
				"enabled":      false,
				"url":          "wss://bot.example.com/reverse",
				"access_token": "reverse-secret",
			},
			"forward_ws": map[string]any{
				"enabled":      false,
				"url":          "ws://127.0.0.1:2658",
				"access_token": "forward-secret",
			},
			"http_api": map[string]any{
				"enabled":      false,
				"url":          "",
				"access_token": "http-secret",
			},
			"webhook": map[string]any{
				"enabled":      false,
				"url":          "https://bot.example.com/webhook",
				"access_token": "webhook-secret",
			},
		},
		"database": map[string]any{
			"engine": "sqlite",
			"path":   "data/rayleabot.db",
		},
		"command": map[string]any{
			"prefixes": []string{"/"},
		},
		"admin": map[string]any{
			"super_admins":              []any{},
			"session_ttl_days":          7,
			"sliding_renewal":           true,
			"max_sessions":              3,
			"login_fail_limit":          5,
			"login_fail_window_seconds": 300,
		},
		"permission": map[string]any{
			"default_level":           "everyone",
			"auto_grant_capabilities": []any{},
		},
		"render": map[string]any{
			"worker_count":               1,
			"browser_args":               []any{"--disable-gpu"},
			"browser_path":               "",
			"timeout_seconds":            30,
			"queue_wait_timeout_seconds": 15,
			"queue_max_length":           32,
		},
		"scheduler": map[string]any{
			"timezone": "",
		},
		"runtime": map[string]any{
			"plugin_init_timeout_seconds":           30,
			"plugin_init_max_total_seconds":         300,
			"plugin_event_timeout_seconds":          60,
			"max_pending_events_per_plugin":         16,
			"max_pending_control_events_per_plugin": 4,
			"nodejs_max_old_space_size_mb":          256,
			"dependency_install_timeout_seconds":    900,
			"max_concurrent_dependency_installs":    1,
			"ipc_pending_actions_max":               256,
			"ipc_action_burst_limit":                "100/1s",
			"stderr_rate_limit_bytes_per_second":    262144,
			"max_concurrent_tasks_per_plugin":       4,
			"crash_backoff_initial_seconds":         2,
			"crash_backoff_max_seconds":             60,
			"shutdown_grace_seconds":                10,
			"ipc_message_max_bytes":                 8388608,
		},
		"storage": map[string]any{
			"kv_value_max_bytes":           65536,
			"kv_total_limit_mb":            16,
			"file_max_bytes":               10485760,
			"plugin_workdir_soft_limit_mb": 256,
		},
		"data": map[string]any{
			"audit_logs_retention_days":     90,
			"event_records_retention_days":  7,
			"download_cache_retention_days": 15,
		},
		"log": map[string]any{
			"level":                 "info",
			"retention_days":        7,
			"rate_limit_per_plugin": "200/10s",
		},
		"message": map[string]any{
			"rate_limit_per_plugin":   "20/10s",
			"rate_limit_per_target":   "5/5s",
			"circuit_breaker_seconds": 30,
		},
		"user": map[string]any{
			"command_rate_limit": "10/60s",
			"cooldown_reply":     true,
		},
		"group": map[string]any{
			"command_rate_limit": "30/60s",
		},
		"adapter": map[string]any{
			"connect_timeout_seconds":   18,
			"reconnect_initial_seconds": 2,
			"reconnect_multiplier":      2,
			"reconnect_max_seconds":     120,
			"reconnect_jitter_ratio":    0.2,
		},
		"http": map[string]any{
			"timeout_seconds":     10,
			"max_retries":         2,
			"allow_private_hosts": []any{},
		},
		"web": map[string]any{
			"exposure_mode":    "localhost_only",
			"setup_local_only": true,
		},
		"backup": map[string]any{
			"default_consistency": "offline",
		},
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal config update request: %v", err)
	}
	request, err := http.NewRequest(http.MethodPut, server.URL+"/api/config", bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("create config update request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform config update request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("unexpected config update status: got %d want 200", response.StatusCode)
	}

	body := decodeBody(t, readAll(t, response))
	if body["restart_required"] != false {
		t.Fatalf("unexpected restart_required: %#v", body["restart_required"])
	}

	snapshotReq, err := http.NewRequest(http.MethodGet, server.URL+"/api/protocols/onebot11", nil)
	if err != nil {
		t.Fatalf("create protocol snapshot request: %v", err)
	}
	snapshotReq.Header.Set("Authorization", "Bearer "+token)
	snapshotResp, err := server.Client().Do(snapshotReq)
	if err != nil {
		t.Fatalf("perform protocol snapshot request: %v", err)
	}
	defer snapshotResp.Body.Close()

	snapshotBody := decodeBody(t, readAll(t, snapshotResp))
	transports, ok := snapshotBody["transport_status"].([]any)
	if !ok {
		t.Fatalf("unexpected transport_status: %#v", snapshotBody["transport_status"])
	}

	statusByTransport := make(map[string]map[string]any, len(transports))
	for _, item := range transports {
		statusItem, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("unexpected transport status item: %#v", item)
		}
		statusByTransport[statusItem["transport"].(string)] = statusItem
	}

	if statusByTransport["forward_ws"]["enabled"] != false || statusByTransport["forward_ws"]["configured"] != true {
		t.Fatalf("unexpected forward_ws snapshot: %#v", statusByTransport["forward_ws"])
	}
	if statusByTransport["reverse_ws"]["enabled"] != false || statusByTransport["reverse_ws"]["configured"] != true {
		t.Fatalf("unexpected reverse_ws snapshot: %#v", statusByTransport["reverse_ws"])
	}
	if statusByTransport["webhook"]["enabled"] != false || statusByTransport["webhook"]["configured"] != true {
		t.Fatalf("unexpected webhook snapshot: %#v", statusByTransport["webhook"])
	}

	reverseReq, err := http.NewRequest(http.MethodGet, server.URL+"/api/protocols/onebot11/reverse-ws", nil)
	if err != nil {
		t.Fatalf("create reverse websocket request: %v", err)
	}
	reverseReq.Header.Set("Authorization", "Bearer "+token)
	reverseResp, err := server.Client().Do(reverseReq)
	if err != nil {
		t.Fatalf("perform reverse websocket request: %v", err)
	}
	defer reverseResp.Body.Close()
	if reverseResp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("unexpected reverse websocket status: got %d want 503", reverseResp.StatusCode)
	}

	webhookReq, err := http.NewRequest(http.MethodPost, server.URL+"/api/protocols/onebot11/webhook", bytes.NewReader([]byte(`{}`)))
	if err != nil {
		t.Fatalf("create webhook request: %v", err)
	}
	webhookReq.Header.Set("Authorization", "Bearer "+token)
	webhookReq.Header.Set("Content-Type", "application/json")
	webhookResp, err := server.Client().Do(webhookReq)
	if err != nil {
		t.Fatalf("perform webhook request: %v", err)
	}
	defer webhookResp.Body.Close()
	if webhookResp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("unexpected webhook status: got %d want 503", webhookResp.StatusCode)
	}
}

func newTestAppWithConfigMutation(t *testing.T, mutate func(map[string]any), authOptions ...auth.Option) (*internalapp.App, string, string) {
	t.Helper()

	fixture := loadConfigFixture(t, filepath.Join("..", "fixtures", "config", "ok.minimal.json"))

	var input map[string]any
	if err := json.Unmarshal(fixture.Input, &input); err != nil {
		t.Fatalf("unmarshal config fixture input: %v", err)
	}
	if mutate != nil {
		mutate(input)
	}

	updated, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal config fixture input: %v", err)
	}

	configPath := writeYAMLConfig(t, updated)
	schemaPath := filepath.Join("..", "contracts", "config.user.schema.json")

	application, err := internalapp.New(internalapp.Options{
		ConfigPath:  configPath,
		SchemaPath:  schemaPath,
		AuthOptions: authOptions,
	})
	if err != nil {
		t.Fatalf("app.New failed: %v", err)
	}
	t.Cleanup(func() {
		if err := application.Close(); err != nil {
			t.Fatalf("close app resources: %v", err)
		}
	})

	return application, configPath, schemaPath
}

func responseBodyString(t *testing.T, body map[string]any) string {
	t.Helper()

	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal response body: %v", err)
	}
	return string(encoded)
}

func normalizeJSONMap(t *testing.T, body map[string]any) map[string]any {
	t.Helper()

	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal expected body: %v", err)
	}

	var normalized map[string]any
	if err := json.Unmarshal(encoded, &normalized); err != nil {
		t.Fatalf("unmarshal expected body: %v", err)
	}

	return normalized
}
