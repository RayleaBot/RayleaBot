package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"rayleabot/server/internal/app"
	"rayleabot/server/internal/auth"
	"rayleabot/server/internal/logging"
)

func TestLogsWebSocketReplaysBufferedSummaries(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	application.Logger.Warn(
		"authentication failed for reverse websocket",
		"component", "adapter.onebot11",
		"request_id", "req_adapter_0001",
	)

	token := issueLoginToken(t, application)
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	conn := dialProtectedWebSocket(t, server.URL, "/ws/logs", token)
	defer conn.Close(websocket.StatusNormalClosure, "")

	frame := readWebSocketFrameWhere(t, conn, func(frame map[string]any) bool {
		data, ok := frame["data"].(map[string]any)
		return ok && data["message"] == "authentication failed for reverse websocket"
	})
	if frame["channel"] != "logs" {
		t.Fatalf("unexpected channel: got %#v want %q", frame["channel"], "logs")
	}
	if frame["type"] != "logs.appended" {
		t.Fatalf("unexpected type: got %#v want %q", frame["type"], "logs.appended")
	}

	data := frame["data"].(map[string]any)
	if data["level"] != "warn" {
		t.Fatalf("unexpected level: got %#v want %q", data["level"], "warn")
	}
	if data["source"] != "adapter.onebot11" {
		t.Fatalf("unexpected source: got %#v want %q", data["source"], "adapter.onebot11")
	}
	if data["message"] != "authentication failed for reverse websocket" {
		t.Fatalf("unexpected message: got %#v", data["message"])
	}
	if data["request_id"] != "req_adapter_0001" {
		t.Fatalf("unexpected request_id: got %#v want %q", data["request_id"], "req_adapter_0001")
	}
}

func TestLogsWebSocketDeliversLiveWhitelistedSummaries(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	replayCount := len(application.Logs.Snapshot())
	token := issueLoginToken(t, application)
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	conn := dialProtectedWebSocket(t, server.URL, "/ws/logs", token)
	defer conn.Close(websocket.StatusNormalClosure, "")

	waitForLogSubscriber(t, application.Logs)
	for i := 0; i < replayCount; i++ {
		_ = readWebSocketJSON(t, conn)
	}

	application.Logger.Error(
		"plugin runtime stderr truncated",
		"component", "runtime",
		"plugin_id", "weather",
		"request_id", "req_plugin_0001",
		"secret", "fixture-only-secret",
		"token", "session-token-abc",
	)

	payload := readWebSocketPayloadWhere(t, conn, func(frame map[string]any) bool {
		data, ok := frame["data"].(map[string]any)
		return ok && data["message"] == "plugin runtime stderr truncated"
	})
	frame := decodeBody(t, payload)
	data := frame["data"].(map[string]any)
	allowed := map[string]bool{
		"timestamp":  true,
		"level":      true,
		"source":     true,
		"message":    true,
		"plugin_id":  true,
		"request_id": true,
	}
	for key := range data {
		if !allowed[key] {
			t.Fatalf("unexpected log summary field %q", key)
		}
	}
	if data["plugin_id"] != "weather" {
		t.Fatalf("unexpected plugin_id: got %#v want %q", data["plugin_id"], "weather")
	}
	if data["request_id"] != "req_plugin_0001" {
		t.Fatalf("unexpected request_id: got %#v want %q", data["request_id"], "req_plugin_0001")
	}

	raw := string(payload)
	for _, forbidden := range []string{
		"fixture-only-secret",
		"session-token-abc",
		"\"secret\"",
		"\"token\"",
	} {
		if strings.Contains(raw, forbidden) {
			t.Fatalf("websocket payload leaked forbidden log content %q: %s", forbidden, raw)
		}
	}
}

func TestLogsWebSocketRedactsSensitiveMessageContent(t *testing.T) {
	application := newTestAppWithOneBotAccessToken(t, "fixture-only-secret", deterministicAuthOptions()...)
	replayCount := len(application.Logs.Snapshot())
	token := issueLoginToken(t, application)
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	conn := dialProtectedWebSocket(t, server.URL, "/ws/logs", token)
	defer conn.Close(websocket.StatusNormalClosure, "")

	waitForLogSubscriber(t, application.Logs)
	for i := 0; i < replayCount; i++ {
		_ = readWebSocketJSON(t, conn)
	}

	application.Logger.Error(
		"downstream rejected fixture-only-secret during adapter handshake",
		"component", "runtime",
	)

	payload := readWebSocketPayloadWhere(t, conn, func(frame map[string]any) bool {
		data, ok := frame["data"].(map[string]any)
		if !ok {
			return false
		}
		message, _ := data["message"].(string)
		return strings.Contains(message, "adapter handshake")
	})

	raw := string(payload)
	if strings.Contains(raw, "fixture-only-secret") {
		t.Fatalf("websocket payload leaked sensitive message content: %s", raw)
	}
	if !strings.Contains(raw, "[REDACTED]") {
		t.Fatalf("expected redacted websocket payload, got %s", raw)
	}
}

func TestLogsWebSocketRejectsUnauthorizedSession(t *testing.T) {
	t.Parallel()

	application := newTestApp(t)
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	conn, response, err := websocket.Dial(ctx, websocketURL(server.URL)+"/ws/logs", nil)
	if conn != nil {
		_ = conn.Close(websocket.StatusNormalClosure, "")
	}
	if err == nil {
		t.Fatal("expected unauthorized websocket dial to fail")
	}
	if response == nil || response.StatusCode != http.StatusUnauthorized {
		if response == nil {
			t.Fatal("expected unauthorized response, got nil")
		}
		t.Fatalf("unexpected unauthorized status: got %d want %d", response.StatusCode, http.StatusUnauthorized)
	}
}

func waitForLogSubscriber(t *testing.T, stream *logging.Stream) {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if stream.SubscriberCount() > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatal("timed out waiting for log websocket subscriber")
}

func newTestAppWithOneBotAccessToken(t *testing.T, accessToken string, authOptions ...auth.Option) *app.App {
	t.Helper()

	fixture := loadConfigFixture(t, filepath.Join("..", "fixtures", "config", "ok.minimal.json"))

	var input map[string]any
	if err := json.Unmarshal(fixture.Input, &input); err != nil {
		t.Fatalf("unmarshal config fixture input: %v", err)
	}

	onebot := input["onebot"].(map[string]any)
	onebot["access_token"] = accessToken

	updated, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal config fixture input: %v", err)
	}

	configPath := writeYAMLConfig(t, updated)
	schemaPath := filepath.Join("..", "contracts", "config.user.schema.json")

	application, err := app.New(app.Options{
		ConfigPath:  configPath,
		SchemaPath:  schemaPath,
		AuthOptions: authOptions,
	})
	if err != nil {
		t.Fatalf("app.New failed: %v", err)
	}
	t.Cleanup(func() {
		if application.Storage != nil {
			if err := application.Storage.Close(); err != nil {
				t.Fatalf("close sqlite store: %v", err)
			}
			application.Storage = nil
		}
	})

	return application
}

func readWebSocketFrameWhere(t *testing.T, conn *websocket.Conn, match func(map[string]any) bool) map[string]any {
	t.Helper()
	return decodeBody(t, readWebSocketPayloadWhere(t, conn, match))
}

func readWebSocketPayloadWhere(t *testing.T, conn *websocket.Conn, match func(map[string]any) bool) []byte {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		readCtx, cancel := context.WithTimeout(context.Background(), time.Until(deadline))
		_, payload, err := conn.Read(readCtx)
		cancel()
		if err != nil {
			t.Fatalf("read websocket frame: %v", err)
		}

		frame := decodeBody(t, payload)
		if match(frame) {
			return payload
		}
	}

	t.Fatal("timed out waiting for matching websocket frame")
	return nil
}
