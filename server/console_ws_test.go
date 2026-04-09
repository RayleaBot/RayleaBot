package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/RayleaBot/RayleaBot/server/internal/console"
)

func TestPluginConsoleWebSocketReplaysBufferedFrames(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	application.Console().Append(console.Entry{
		PluginID:  "raylea.help",
		Stream:    "stderr",
		Text:      "Traceback (most recent call last): ...",
		Timestamp: time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
	})

	token := issueLoginToken(t, application)
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	conn := dialProtectedWebSocket(t, server.URL, "/ws/plugins/raylea.help/console", token)
	defer conn.Close(websocket.StatusNormalClosure, "")

	frame := readWebSocketJSON(t, conn)
	if frame["channel"] != "plugin_console" {
		t.Fatalf("unexpected channel: got %#v want %q", frame["channel"], "plugin_console")
	}
	if frame["type"] != "plugins.console" {
		t.Fatalf("unexpected type: got %#v want %q", frame["type"], "plugins.console")
	}

	data := frame["data"].(map[string]any)
	if data["plugin_id"] != "raylea.help" {
		t.Fatalf("unexpected plugin_id: got %#v want %q", data["plugin_id"], "raylea.help")
	}
	if data["stream"] != "stderr" {
		t.Fatalf("unexpected stream: got %#v want %q", data["stream"], "stderr")
	}
	if data["text"] != "Traceback (most recent call last): ..." {
		t.Fatalf("unexpected text: got %#v", data["text"])
	}
	if data["timestamp"] != "2026-03-20T10:00:00Z" {
		t.Fatalf("unexpected data timestamp: got %#v want %q", data["timestamp"], "2026-03-20T10:00:00Z")
	}
}

func TestPluginConsoleWebSocketDeliversLiveFrames(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	conn := dialProtectedWebSocket(t, server.URL, "/ws/plugins/raylea.help/console", token)
	defer conn.Close(websocket.StatusNormalClosure, "")

	waitForConsoleSubscriber(t, application.Console(), "raylea.help")
	application.Console().Append(console.Entry{
		PluginID:  "raylea.help",
		Stream:    "system",
		Text:      "[System] stderr rate limit exceeded, output truncated",
		Timestamp: time.Date(2026, 3, 20, 10, 5, 0, 0, time.UTC),
	})

	frame := readWebSocketJSON(t, conn)
	data := frame["data"].(map[string]any)
	if data["plugin_id"] != "raylea.help" {
		t.Fatalf("unexpected plugin_id: got %#v want %q", data["plugin_id"], "raylea.help")
	}
	if data["stream"] != "system" {
		t.Fatalf("unexpected stream: got %#v want %q", data["stream"], "system")
	}
	if data["text"] != "[System] stderr rate limit exceeded, output truncated" {
		t.Fatalf("unexpected text: got %#v", data["text"])
	}
}

func TestPluginConsoleWebSocketRejectsUnauthorizedSession(t *testing.T) {
	t.Parallel()

	application := newTestApp(t)
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	conn, response, err := websocket.Dial(ctx, websocketURL(server.URL)+"/ws/plugins/raylea.help/console", nil)
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

func waitForConsoleSubscriber(t *testing.T, stream *console.Stream, pluginID string) {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if stream.SubscriberCount(pluginID) > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for console websocket subscriber for %s", pluginID)
}
