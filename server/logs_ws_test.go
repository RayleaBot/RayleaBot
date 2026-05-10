package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/RayleaBot/RayleaBot/server/internal/app"
	"github.com/RayleaBot/RayleaBot/server/internal/auth"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
)

func TestLogsWebSocketReplaysBufferedSummaries(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	application.Logger().Warn(
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
	if data["log_id"] == "" {
		t.Fatalf("expected log_id to be populated, got %#v", data["log_id"])
	}
	if data["level"] != "warn" {
		t.Fatalf("unexpected level: got %#v want %q", data["level"], "warn")
	}
	if data["source"] != "adapter.onebot11" {
		t.Fatalf("unexpected source: got %#v want %q", data["source"], "adapter.onebot11")
	}
	if data["protocol"] != "onebot11" {
		t.Fatalf("unexpected protocol: got %#v want %q", data["protocol"], "onebot11")
	}
	if data["message"] != "authentication failed for reverse websocket" {
		t.Fatalf("unexpected message: got %#v", data["message"])
	}
	if data["request_id"] != "req_adapter_0001" {
		t.Fatalf("unexpected request_id: got %#v want %q", data["request_id"], "req_adapter_0001")
	}
}

func TestLogsWebSocketReplaysOutboundDeliverySummary(t *testing.T) {
	t.Parallel()

	rawFixture, err := os.ReadFile(filepath.Join("..", "fixtures", "websocket", "ok.logs-appended.outbound-onebot11.json"))
	if err != nil {
		t.Fatalf("read websocket outbound fixture: %v", err)
	}

	var fixture map[string]any
	if err := json.Unmarshal(rawFixture, &fixture); err != nil {
		t.Fatalf("decode websocket outbound fixture: %v", err)
	}

	application := newTestApp(t, deterministicAuthOptions()...)
	application.Logs().Append(logging.Summary{
		LogID:     "log_outbound_delivered_0001",
		Timestamp: "2026-04-10T09:18:00Z",
		Level:     "info",
		Source:    "adapter.onebot11",
		Message:   "weather/echo -> [测试群(2001)]：hello world",
		PluginID:  "weather",
		RequestID: "req_runtime_delivery_0001",
	})

	token := issueLoginToken(t, application)
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	conn := dialProtectedWebSocket(t, server.URL, "/ws/logs", token)
	defer conn.Close(websocket.StatusNormalClosure, "")

	frame := readWebSocketFrameWhere(t, conn, func(frame map[string]any) bool {
		data, ok := frame["data"].(map[string]any)
		return ok && data["request_id"] == "req_runtime_delivery_0001"
	})

	expectedFrame, ok := fixture["frame"].(map[string]any)
	if !ok {
		t.Fatalf("fixture frame has unexpected shape: %#v", fixture["frame"])
	}
	if frame["channel"] != expectedFrame["channel"] {
		t.Fatalf("unexpected channel: got %#v want %#v", frame["channel"], expectedFrame["channel"])
	}
	if frame["type"] != expectedFrame["type"] {
		t.Fatalf("unexpected frame type: got %#v want %#v", frame["type"], expectedFrame["type"])
	}

	timestamp, ok := frame["timestamp"].(string)
	if !ok || strings.TrimSpace(timestamp) == "" {
		t.Fatalf("expected websocket frame timestamp, got %#v", frame["timestamp"])
	}
	if _, err := time.Parse(time.RFC3339, timestamp); err != nil {
		t.Fatalf("unexpected websocket frame timestamp: %v", err)
	}

	data, ok := frame["data"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected websocket frame data: %#v", frame["data"])
	}
	expectedData, ok := expectedFrame["data"].(map[string]any)
	if !ok {
		t.Fatalf("fixture frame data has unexpected shape: %#v", expectedFrame["data"])
	}
	if !reflect.DeepEqual(data, expectedData) {
		t.Fatalf("unexpected outbound websocket data: got %#v want %#v", data, expectedData)
	}
}

func TestLogsWebSocketAppendsCommandPolicyRejectionSummary(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	putWhitelistState(t, server.URL, token, true)

	conn := dialProtectedWebSocket(t, server.URL, "/ws/logs", token)
	defer conn.Close(websocket.StatusNormalClosure, "")

	waitForLogSubscriber(t, application.Logs())
	application.HandleAdapterEvent(context.Background(), commandRejectionEvent())

	frame := readWebSocketFrameWhere(t, conn, func(frame map[string]any) bool {
		data, ok := frame["data"].(map[string]any)
		return ok && data["message"] == "plugin raylea.echo command echo rejected by command policy: sender is not whitelisted"
	})

	data := frame["data"].(map[string]any)
	if data["source"] != "bridge" || data["protocol"] != "onebot11" {
		t.Fatalf("unexpected command rejection websocket summary: %#v", data)
	}
	if data["plugin_id"] != "raylea.echo" {
		t.Fatalf("unexpected command rejection websocket plugin_id: %#v", data["plugin_id"])
	}
	allowed := map[string]bool{
		"log_id":     true,
		"timestamp":  true,
		"level":      true,
		"source":     true,
		"message":    true,
		"protocol":   true,
		"plugin_id":  true,
		"request_id": true,
	}
	for key := range data {
		if !allowed[key] {
			t.Fatalf("unexpected websocket summary field %q", key)
		}
	}
}

func TestLogsWebSocketDeliversLiveWhitelistedSummaries(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	replayCount := len(application.Logs().Snapshot())
	token := issueLoginToken(t, application)
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	conn := dialProtectedWebSocket(t, server.URL, "/ws/logs", token)
	defer conn.Close(websocket.StatusNormalClosure, "")

	waitForLogSubscriber(t, application.Logs())
	for i := 0; i < replayCount; i++ {
		_ = readWebSocketJSON(t, conn)
	}

	application.Logger().Error(
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
		"log_id":     true,
		"timestamp":  true,
		"level":      true,
		"source":     true,
		"message":    true,
		"protocol":   true,
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
	replayCount := len(application.Logs().Snapshot())
	token := issueLoginToken(t, application)
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	conn := dialProtectedWebSocket(t, server.URL, "/ws/logs", token)
	defer conn.Close(websocket.StatusNormalClosure, "")

	waitForLogSubscriber(t, application.Logs())
	for i := 0; i < replayCount; i++ {
		_ = readWebSocketJSON(t, conn)
	}

	application.Logger().Error(
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

func TestLogsWebSocketReplaysCurrentBootOnlyAcrossRestart(t *testing.T) {
	t.Parallel()

	configPath := writePersistentYAMLConfig(t, filepath.Join(t.TempDir(), "state.db"))
	appA := newPersistentTestApp(t, configPath, func() time.Time { return time.Date(2026, 3, 20, 9, 0, 0, 0, time.UTC) }, "logs-ws-a")
	_ = issueLoginToken(t, appA)
	appA.Logger().Warn(
		"persisted websocket replay",
		"component", "adapter.onebot11",
		"request_id", "req_ws_persist_1",
	)
	closePersistentTestApp(t, appA)

	appB := newPersistentTestApp(t, configPath, func() time.Time { return time.Date(2026, 3, 20, 9, 10, 0, 0, time.UTC) }, "logs-ws-b")
	defer closePersistentTestApp(t, appB)
	appB.Logger().Warn(
		"current boot websocket replay",
		"component", "adapter.onebot11",
		"request_id", "req_ws_current_1",
	)
	server := httptest.NewServer(appB.Handler())
	defer server.Close()

	token := issueExistingBootstrapLoginToken(t, appB)
	conn := dialProtectedWebSocket(t, server.URL, "/ws/logs", token)
	defer conn.Close(websocket.StatusNormalClosure, "")

	frame := readWebSocketFrameWhere(t, conn, func(frame map[string]any) bool {
		data, ok := frame["data"].(map[string]any)
		if ok && data["request_id"] == "req_ws_persist_1" {
			t.Fatalf("old boot websocket replay leaked into current session: %#v", frame)
		}
		return ok && data["request_id"] == "req_ws_current_1"
	})
	data := frame["data"].(map[string]any)
	if data["message"] != "current boot websocket replay" {
		t.Fatalf("unexpected websocket replay message: %#v", data["message"])
	}
	if data["log_id"] == "" {
		t.Fatalf("expected websocket replay log_id, got %#v", data["log_id"])
	}
	if data["source"] != "adapter.onebot11" {
		t.Fatalf("unexpected websocket replay source: %#v", data["source"])
	}
	if data["protocol"] != "onebot11" {
		t.Fatalf("unexpected websocket replay protocol: %#v", data["protocol"])
	}

	assertNoWebSocketFrameWhere(t, conn, 200*time.Millisecond, func(frame map[string]any) bool {
		data, ok := frame["data"].(map[string]any)
		return ok && data["request_id"] == "req_ws_persist_1"
	})
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
	onebot["forward_ws"].(map[string]any)["access_token"] = accessToken

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
		if err := application.Close(); err != nil {
			t.Fatalf("close app resources: %v", err)
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

func assertNoWebSocketFrameWhere(t *testing.T, conn *websocket.Conn, window time.Duration, match func(map[string]any) bool) {
	t.Helper()

	deadline := time.Now().Add(window)
	for time.Now().Before(deadline) {
		readCtx, cancel := context.WithTimeout(context.Background(), time.Until(deadline))
		_, payload, err := conn.Read(readCtx)
		cancel()
		if err != nil {
			return
		}

		frame := decodeBody(t, payload)
		if match(frame) {
			t.Fatalf("unexpected websocket frame: %#v", frame)
		}
	}
}
