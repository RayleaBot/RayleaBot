package testutil

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"gopkg.in/yaml.v3"
)

type WebAPIFixtureDocument struct {
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

type WebAPIFixture struct {
	Response struct {
		Body map[string]any `yaml:"body"`
	} `yaml:"response"`
}

func EncodeBodyReader(t testing.TB, body map[string]any) io.Reader {
	t.Helper()

	if body == nil {
		return httpNoBodyReader{}
	}

	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}
	return bytes.NewReader(encoded)
}

type httpNoBodyReader struct{}

func (httpNoBodyReader) Read(_ []byte) (int, error) { return 0, io.EOF }

func LoadWebAPIFixtureDocument(t testing.TB, path string) WebAPIFixtureDocument {
	t.Helper()

	bytes, err := ReadRepoPath(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}

	var fixture WebAPIFixtureDocument
	if err := yaml.Unmarshal(bytes, &fixture); err != nil {
		t.Fatalf("unmarshal fixture %s: %v", path, err)
	}

	return fixture
}

func LoadWebAPIFixture(t testing.TB, path string) WebAPIFixture {
	t.Helper()

	bytes, err := ReadRepoPath(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}

	var fixture WebAPIFixture
	if err := yaml.Unmarshal(bytes, &fixture); err != nil {
		t.Fatalf("unmarshal fixture %s: %v", path, err)
	}
	fixture.Response.Body = normalizeFixtureMap(fixture.Response.Body)

	return fixture
}

func PerformJSONRequest(t testing.TB, application interface{ Handler() http.Handler }, method, path string, body map[string]any) *httptest.ResponseRecorder {
	return PerformJSONRequestWithRemoteAddr(t, application, method, path, body, "127.0.0.1:0")
}

func PerformJSONRequestWithRemoteAddr(t testing.TB, application interface{ Handler() http.Handler }, method, path string, body map[string]any, remoteAddr string) *httptest.ResponseRecorder {
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

	return PerformJSONBytesRequestWithRemoteAddr(t, application, method, path, payload, remoteAddr)
}

func PerformJSONBytesRequest(t testing.TB, application interface{ Handler() http.Handler }, method, path string, payload []byte) *httptest.ResponseRecorder {
	return PerformJSONBytesRequestWithRemoteAddr(t, application, method, path, payload, "127.0.0.1:0")
}

func PerformJSONBytesRequestWithRemoteAddr(t testing.TB, application interface{ Handler() http.Handler }, method, path string, payload []byte, remoteAddr string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(method, path, bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = remoteAddr
	recorder := httptest.NewRecorder()
	application.Handler().ServeHTTP(recorder, request)
	return recorder
}

func AssertErrorEnvelopeMatchesFixture(t testing.TB, actual map[string]any, expected map[string]any, wantCode string) {
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

func CloneMap(input map[string]any) map[string]any {
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func DecodeBody(t testing.TB, raw []byte) map[string]any {
	t.Helper()

	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	return body
}

func ReadAll(t testing.TB, response *http.Response) []byte {
	t.Helper()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	return body
}

func IssueLoginToken(t testing.TB, application interface{ Handler() http.Handler }) string {
	t.Helper()

	setupFixture := LoadWebAPIFixtureDocument(t, "../fixtures/web-api/ok.setup-admin.yaml")
	loginFixture := LoadWebAPIFixtureDocument(t, "../fixtures/web-api/ok.session-login.yaml")

	setup := PerformJSONRequest(t, application, setupFixture.Request.Method, setupFixture.Request.Path, setupFixture.Request.Body)
	if setup.Code != setupFixture.Response.Status {
		t.Fatalf("unexpected bootstrap status: got %d want %d", setup.Code, setupFixture.Response.Status)
	}

	login := PerformJSONRequest(t, application, loginFixture.Request.Method, loginFixture.Request.Path, loginFixture.Request.Body)
	if login.Code != loginFixture.Response.Status {
		t.Fatalf("unexpected login status: got %d want %d", login.Code, loginFixture.Response.Status)
	}

	body := DecodeBody(t, login.Body.Bytes())
	token, ok := body["session_token"].(string)
	if !ok || token == "" {
		t.Fatalf("expected opaque session_token, got %#v", body["session_token"])
	}

	return token
}

func IssueExistingBootstrapLoginToken(t testing.TB, application interface{ Handler() http.Handler }) string {
	t.Helper()

	loginFixture := LoadWebAPIFixtureDocument(t, "../fixtures/web-api/ok.session-login.yaml")
	login := PerformJSONRequest(t, application, loginFixture.Request.Method, loginFixture.Request.Path, loginFixture.Request.Body)
	if login.Code != loginFixture.Response.Status {
		t.Fatalf("unexpected login status: got %d want %d", login.Code, loginFixture.Response.Status)
	}

	body := DecodeBody(t, login.Body.Bytes())
	token, ok := body["session_token"].(string)
	if !ok || token == "" {
		t.Fatalf("expected opaque session_token, got %#v", body["session_token"])
	}

	return token
}

func WebSocketURL(httpURL string) string {
	if strings.HasPrefix(httpURL, "https://") {
		return "wss://" + strings.TrimPrefix(httpURL, "https://")
	}
	return "ws://" + strings.TrimPrefix(httpURL, "http://")
}

func DialProtectedWebSocket(t testing.TB, baseURL, path, token string) *websocket.Conn {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	conn, response, err := websocket.Dial(ctx, WebSocketURL(baseURL)+path+"?session_token="+token, nil)
	if err != nil {
		if response != nil {
			t.Fatalf("dial websocket returned status %d: %v", response.StatusCode, err)
		}
		t.Fatalf("dial websocket: %v", err)
	}

	return conn
}

func DialEventsWebSocket(t testing.TB, baseURL, token string) *websocket.Conn {
	return DialProtectedWebSocket(t, baseURL, "/ws/events", token)
}

func ReadWebSocketJSON(t testing.TB, conn *websocket.Conn) map[string]any {
	t.Helper()

	readCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, payload, err := conn.Read(readCtx)
	if err != nil {
		t.Fatalf("read websocket frame: %v", err)
	}

	return DecodeBody(t, payload)
}

func normalizeFixtureMap(values map[string]any) map[string]any {
	result := make(map[string]any, len(values))
	for key, value := range values {
		result[key] = normalizeFixtureValue(value)
	}
	return result
}

func normalizeFixtureValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return normalizeFixtureMap(typed)
	case []any:
		items := make([]any, 0, len(typed))
		for _, item := range typed {
			items = append(items, normalizeFixtureValue(item))
		}
		return items
	case time.Time:
		return typed.UTC().Format(time.RFC3339)
	default:
		return value
	}
}
