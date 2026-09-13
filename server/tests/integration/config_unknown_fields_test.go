package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/tests/testutil"
)

func TestConfigPutIgnoresUnknownFields(t *testing.T) {
	t.Parallel()
	application, configPath, _ := newTestAppWithConfigMutation(t, nil, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	fixture := loadWebAPIFixtureDocument(t, testutil.RepoPath(t, "fixtures", "web-api", "ok.config-update-response.yaml"))
	document := normalizeJSONMap(t, fixture.Request.Body)
	document["obsolete_section"] = "ignored-config-marker"
	document["web"].(map[string]any)["exposure_mode"] = "ignored-config-marker"
	document["adapters"].([]any)[0].(map[string]any)["obsolete_adapter_flag"] = "ignored-config-marker"
	testutil.ConfigDocumentOneBot(t, document)["reverse_ws"].(map[string]any)["obsolete_transport_option"] = "ignored-config-marker"
	payload, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPut, "/api/config", bytes.NewReader(payload))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	application.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("unknown config fields rejected: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if bytes.Contains(recorder.Body.Bytes(), []byte("ignored-config-marker")) {
		t.Fatal("config update response retained ignored fields")
	}
	persisted, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(persisted, []byte("ignored-config-marker")) {
		t.Fatal("config update persisted ignored fields")
	}
	if application.CurrentConfig().Log.Level != "debug" {
		t.Fatal("config update did not apply the declared log level")
	}
	getRequest := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	getRequest.Header.Set("Authorization", "Bearer "+token)
	getRecorder := httptest.NewRecorder()
	application.Handler().ServeHTTP(getRecorder, getRequest)
	if getRecorder.Code != http.StatusOK || bytes.Contains(getRecorder.Body.Bytes(), []byte("ignored-config-marker")) {
		t.Fatal("config snapshot did not omit ignored fields")
	}
}
