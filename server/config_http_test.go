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

	internalapp "rayleabot/server/internal/app"
	"rayleabot/server/internal/auth"
	internalconfig "rayleabot/server/internal/config"
)

func TestConfigGetReturnsRedactedSnapshot(t *testing.T) {
	t.Parallel()

	application, _, _ := newTestAppWithConfigMutation(t, func(input map[string]any) {
		input["onebot"].(map[string]any)["access_token"] = "fixture-only-secret"
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

	raw := responseBodyString(t, body)
	if strings.Contains(raw, "fixture-only-secret") {
		t.Fatalf("config get response leaked raw secret: %s", raw)
	}
}

func TestConfigPutWritesValidatedDocumentAndPreservesRedactedSecret(t *testing.T) {
	t.Parallel()

	application, configPath, schemaPath := newTestAppWithConfigMutation(t, func(input map[string]any) {
		input["onebot"].(map[string]any)["access_token"] = "fixture-only-secret"
	}, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	fixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "ok.config-update-response.yaml"))
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	payload, err := json.Marshal(fixture.Request.Body)
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
	if got := document["onebot"].(map[string]any)["access_token"]; got != "fixture-only-secret" {
		t.Fatalf("unexpected persisted access_token: got %#v want preserved secret", got)
	}

	if application.Config.Server.Port != 8081 {
		t.Fatalf("expected live config server.port to reflect saved value 8081, got %d", application.Config.Server.Port)
	}
	if application.Config.Logging.Level != "debug" {
		t.Fatalf("expected live config log.level to be hot-reloaded to debug, got %q", application.Config.Logging.Level)
	}
}

func TestConfigPutRejectsInvalidConfig(t *testing.T) {
	t.Parallel()

	application, configPath, schemaPath := newTestAppWithConfigMutation(t, func(input map[string]any) {
		input["onebot"].(map[string]any)["access_token"] = "fixture-only-secret"
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
