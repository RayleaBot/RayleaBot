package testutil

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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

const (
	TestSetupToken           = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	TestLauncherControlToken = "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"
	TestManagementAuthority  = "127.0.0.1:8080"
	TestManagementOrigin     = "http://" + TestManagementAuthority
)

func NewManagementTestServer(t testing.TB, handler http.Handler) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Host = TestManagementAuthority
		handler.ServeHTTP(w, r)
	}))
}

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

func PerformJSONBytesRequestWithRemoteAddr(t testing.TB, application interface{ Handler() http.Handler }, method, path string, payload []byte, remoteAddr string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(method, path, bytes.NewReader(payload))
	request.Host = "127.0.0.1:8080"
	request.Header.Set("Content-Type", "application/json")
	if path == "/api/setup/admin" {
		request.Header.Set("Origin", "http://127.0.0.1:8080")
		request.Header.Set("X-Raylea-Setup-Token", TestSetupToken)
	}
	if strings.HasPrefix(path, "/api/launcher/") {
		request.Header.Set("X-Raylea-Launcher-Control", TestLauncherControlToken)
	}
	request.RemoteAddr = remoteAddr
	recorder := httptest.NewRecorder()
	application.Handler().ServeHTTP(recorder, request)
	return recorder
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

	conn, response, err := websocket.Dial(ctx, WebSocketURL(baseURL)+path, &websocket.DialOptions{
		Host: TestManagementAuthority,
		HTTPHeader: http.Header{
			"Authorization": []string{"Bearer " + token},
			"Origin":        []string{TestManagementOrigin},
		},
	})
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
