package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"rayleabot/server/internal/auth"
)

func TestSetupAdminReturnsSessionToken(t *testing.T) {
	t.Parallel()

	application := newTestApp(t)
	application.Auth = newDeterministicAuthManager(t)
	fixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "ok.setup-admin.yaml"))

	recorder := performJSONRequest(t, application, fixture.Request.Method, fixture.Request.Path, fixture.Request.Body)
	if recorder.Code != fixture.Response.Status {
		t.Fatalf("unexpected status: got %d want %d", recorder.Code, fixture.Response.Status)
	}

	body := decodeBody(t, recorder.Body.Bytes())
	token, ok := body["session_token"].(string)
	if !ok || token == "" {
		t.Fatalf("expected opaque session_token, got %#v", body["session_token"])
	}
	if len(body) != 1 {
		t.Fatalf("unexpected success body shape: %#v", body)
	}

	expected := cloneMap(fixture.Response.Body)
	expected["session_token"] = token
	if !reflect.DeepEqual(body, expected) {
		t.Fatalf("unexpected success body: got %#v want %#v", body, expected)
	}

	raw := recorder.Body.String()
	if strings.Contains(raw, fixture.Request.Body["identifier"].(string)) || strings.Contains(raw, fixture.Request.Body["secret"].(string)) {
		t.Fatalf("response leaked request credential content: %s", raw)
	}
}

func TestSetupAdminRejectsMalformedRequest(t *testing.T) {
	t.Parallel()

	application := newTestApp(t)
	application.Auth = newDeterministicAuthManager(t)
	fixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "invalid.setup-admin-bad-request.yaml"))

	recorder := performJSONRequest(t, application, fixture.Request.Method, fixture.Request.Path, fixture.Request.Body)
	if recorder.Code != fixture.Response.Status {
		t.Fatalf("unexpected status: got %d want %d", recorder.Code, fixture.Response.Status)
	}

	body := decodeBody(t, recorder.Body.Bytes())
	assertErrorEnvelopeMatchesFixture(t, body, fixture.Response.Body, "platform.invalid_request")

	raw := recorder.Body.String()
	if strings.Contains(raw, "fixture-only-secret") || strings.Contains(raw, "identifier") && strings.Contains(raw, "admin") {
		t.Fatalf("malformed response leaked request content: %s", raw)
	}
}

func TestSetupAdminRejectsAlreadyInitialized(t *testing.T) {
	t.Parallel()

	application := newTestApp(t)
	application.Auth = newDeterministicAuthManager(t)
	okFixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "ok.setup-admin.yaml"))
	edgeFixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "edge.setup-admin-already-initialized.yaml"))

	first := performJSONRequest(t, application, okFixture.Request.Method, okFixture.Request.Path, okFixture.Request.Body)
	if first.Code != okFixture.Response.Status {
		t.Fatalf("unexpected first bootstrap status: got %d want %d", first.Code, okFixture.Response.Status)
	}

	second := performJSONRequest(t, application, edgeFixture.Request.Method, edgeFixture.Request.Path, edgeFixture.Request.Body)
	if second.Code != edgeFixture.Response.Status {
		t.Fatalf("unexpected second bootstrap status: got %d want %d", second.Code, edgeFixture.Response.Status)
	}

	body := decodeBody(t, second.Body.Bytes())
	assertErrorEnvelopeMatchesFixture(t, body, edgeFixture.Response.Body, "permission.denied")

	raw := second.Body.String()
	if strings.Contains(raw, edgeFixture.Request.Body["identifier"].(string)) || strings.Contains(raw, edgeFixture.Request.Body["secret"].(string)) {
		t.Fatalf("edge response leaked request content: %s", raw)
	}
}

func loadWebAPIFixtureDocument(t *testing.T, path string) webAPIFixtureDocument {
	t.Helper()

	normalizedPath := filepath.Clean(filepath.FromSlash(strings.ReplaceAll(path, "\\", "/")))
	bytes, err := os.ReadFile(normalizedPath)
	if err != nil {
		t.Fatalf("read fixture %s: %v", normalizedPath, err)
	}

	var fixture webAPIFixtureDocument
	if err := yaml.Unmarshal(bytes, &fixture); err != nil {
		t.Fatalf("unmarshal fixture %s: %v", normalizedPath, err)
	}

	return fixture
}

type webAPIFixtureDocument struct {
	Request struct {
		Method string         `yaml:"method"`
		Path   string         `yaml:"path"`
		Body   map[string]any `yaml:"body"`
	} `yaml:"request"`
	Response struct {
		Status  int               `yaml:"status"`
		Headers map[string]string `yaml:"headers"`
		Body    map[string]any    `yaml:"body"`
	} `yaml:"response"`
}

func performJSONRequest(t *testing.T, application interface{ Handler() http.Handler }, method, path string, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()

	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
	} else {
		payload = []byte("{}")
	}

	request := httptest.NewRequest(method, path, bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	application.Handler().ServeHTTP(recorder, request)
	return recorder
}

func assertErrorEnvelopeMatchesFixture(t *testing.T, actual map[string]any, expected map[string]any, wantCode string) {
	t.Helper()

	errorBody, ok := actual["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error envelope, got %#v", actual)
	}
	if errorBody["code"] != wantCode {
		t.Fatalf("unexpected error code: got %#v want %q", errorBody["code"], wantCode)
	}

	expectedError := expected["error"].(map[string]any)
	if errorBody["message"] != expectedError["message"] {
		t.Fatalf("unexpected error message: got %#v want %#v", errorBody["message"], expectedError["message"])
	}
	if errorBody["message_key"] != expectedError["message_key"] {
		t.Fatalf("unexpected error message_key: got %#v want %#v", errorBody["message_key"], expectedError["message_key"])
	}
	requestID, ok := errorBody["request_id"].(string)
	if !ok || !strings.HasPrefix(requestID, "req_") {
		t.Fatalf("unexpected request_id: %#v", errorBody["request_id"])
	}

	expectedDetails, hasExpectedDetails := expectedError["details"]
	actualDetails, hasActualDetails := errorBody["details"]
	if hasExpectedDetails != hasActualDetails {
		t.Fatalf("unexpected error details presence: got %#v want %#v", actualDetails, expectedDetails)
	}
	if hasExpectedDetails && !reflect.DeepEqual(actualDetails, expectedDetails) {
		t.Fatalf("unexpected error details: got %#v want %#v", actualDetails, expectedDetails)
	}

	wantLen := 4
	if hasExpectedDetails {
		wantLen = 5
	}
	if len(errorBody) != wantLen {
		t.Fatalf("unexpected error body shape: %#v", errorBody)
	}
}

func cloneMap(input map[string]any) map[string]any {
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func newDeterministicAuthManager(t *testing.T) *auth.Manager {
	t.Helper()

	current := time.Date(2026, 3, 19, 10, 0, 0, 0, time.UTC)
	sessionCounter := 0
	manager, err := auth.NewManager(
		auth.Config{
			SessionTTLDays: 1,
			SlidingRenewal: false,
			MaxSessions:    3,
		},
		auth.WithClock(func() time.Time {
			return current
		}),
		auth.WithSigningKey([]byte("0123456789abcdef0123456789abcdef")),
		auth.WithSessionIDGenerator(func() (string, error) {
			sessionCounter++
			return "session-test-" + string(rune('0'+sessionCounter)), nil
		}),
	)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	return manager
}

// deterministicAuthOptions returns auth.Option values that produce a
// deterministic auth.Manager when passed to app.New via Options.AuthOptions.
// Unlike newDeterministicAuthManager, these options are applied at router
// creation time so the RequireAuth middleware captures the correct manager.
func deterministicAuthOptions() []auth.Option {
	current := time.Date(2026, 3, 19, 10, 0, 0, 0, time.UTC)
	sessionCounter := 0
	return []auth.Option{
		auth.WithClock(func() time.Time {
			return current
		}),
		auth.WithSigningKey([]byte("0123456789abcdef0123456789abcdef")),
		auth.WithSessionIDGenerator(func() (string, error) {
			sessionCounter++
			return "session-test-" + string(rune('0'+sessionCounter)), nil
		}),
	}
}
