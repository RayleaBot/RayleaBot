package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"rayleabot/server/internal/adapter"
	"rayleabot/server/internal/bridge"
	"rayleabot/server/internal/runtime"
)

func TestEventsWebSocketDeliversBridgeRuntimeFrame(t *testing.T) {
	t.Parallel()

	application := newTestApp(t)
	application.Auth = newDeterministicAuthManager(t)
	application.Bridge = bridge.New(application.Logger, &eventsRuntimeStub{
		snapshot: runtime.Snapshot{State: runtime.StateRunning},
		deliverResult: runtime.Delivery{
			RequestID: "req_evt_1",
			Result: map[string]any{
				"handled": true,
			},
		},
	}, nil)

	token := issueLoginToken(t, application)
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	conn := dialEventsWebSocket(t, server.URL, token)
	defer conn.Close(websocket.StatusNormalClosure, "")

	waitForObservabilitySubscriber(t, application.Bridge)

	outcome := application.Bridge.HandleAdapterEvent(context.Background(), testBridgeEvent())
	if outcome != bridge.OutcomeDelivered {
		t.Fatalf("unexpected bridge outcome: got %q want %q", outcome, bridge.OutcomeDelivered)
	}

	readCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, payload, err := conn.Read(readCtx)
	if err != nil {
		t.Fatalf("read websocket frame: %v", err)
	}

	var frame map[string]any
	if err := json.Unmarshal(payload, &frame); err != nil {
		t.Fatalf("unmarshal websocket frame: %v", err)
	}

	if frame["channel"] != "events" {
		t.Fatalf("unexpected channel: got %#v want %q", frame["channel"], "events")
	}
	if frame["type"] != "events.received" {
		t.Fatalf("unexpected type: got %#v want %q", frame["type"], "events.received")
	}

	data, ok := frame["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data object, got %#v", frame["data"])
	}
	if data["observability_scope"] != "bridge_runtime" {
		t.Fatalf("unexpected observability_scope: got %#v", data["observability_scope"])
	}
	if data["summary"] == "" {
		t.Fatalf("expected non-empty summary")
	}
	if data["last_supported_event_kind"] != string(adapter.EventKindMessageText) {
		t.Fatalf("unexpected last_supported_event_kind: got %#v", data["last_supported_event_kind"])
	}
	if data["last_delivery_outcome"] != string(bridge.OutcomeDelivered) {
		t.Fatalf("unexpected last_delivery_outcome: got %#v", data["last_delivery_outcome"])
	}
	if data["delivered_count"] != float64(1) || data["result_count"] != float64(1) || data["error_count"] != float64(0) {
		t.Fatalf("unexpected aggregate counts: %#v", data)
	}

	raw := string(payload)
	for _, forbidden := range []string{
		"hello bridge",
		"onebot11-message-1001",
		"3001",
		"2001",
		"req_evt_1",
		"plain_text",
		"event_id",
		"request_id",
	} {
		if strings.Contains(raw, forbidden) {
			t.Fatalf("websocket payload leaked forbidden content %q: %s", forbidden, raw)
		}
	}
}

func TestEventsWebSocketIsLiveOnlyWithoutReplay(t *testing.T) {
	t.Parallel()

	application := newTestApp(t)
	application.Auth = newDeterministicAuthManager(t)
	application.Bridge = bridge.New(application.Logger, &eventsRuntimeStub{
		snapshot: runtime.Snapshot{State: runtime.StateRunning},
	}, nil)

	token := issueLoginToken(t, application)
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	conn := dialEventsWebSocket(t, server.URL, token)
	defer conn.Close(websocket.StatusNormalClosure, "")

	waitForObservabilitySubscriber(t, application.Bridge)

	readCtx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	_, _, err := conn.Read(readCtx)
	if err == nil {
		t.Fatalf("expected no replay/backfill frame on connect")
	}
	if !strings.Contains(err.Error(), context.DeadlineExceeded.Error()) {
		t.Fatalf("expected deadline exceeded for live-only idle connection, got %v", err)
	}
}

func TestEventsWebSocketRejectsUnauthorizedSession(t *testing.T) {
	t.Parallel()

	application := newTestApp(t)
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	conn, response, err := websocket.Dial(ctx, websocketURL(server.URL)+"/ws/events", nil)
	if conn != nil {
		_ = conn.Close(websocket.StatusNormalClosure, "")
	}
	if err == nil {
		t.Fatalf("expected unauthorized websocket dial to fail")
	}
	if response == nil || response.StatusCode != http.StatusUnauthorized {
		if response == nil {
			t.Fatalf("expected unauthorized response, got nil")
		}
		t.Fatalf("unexpected unauthorized status: got %d want %d", response.StatusCode, http.StatusUnauthorized)
	}
}

type eventsRuntimeStub struct {
	snapshot      runtime.Snapshot
	deliverResult runtime.Delivery
	deliverError  error
}

func (s *eventsRuntimeStub) Snapshot() runtime.Snapshot {
	return s.snapshot
}

func (s *eventsRuntimeStub) DeliverEvent(context.Context, runtime.Event) (runtime.Delivery, error) {
	return s.deliverResult, s.deliverError
}

func issueLoginToken(t *testing.T, application interface{ Handler() http.Handler }) string {
	t.Helper()

	setupFixture := loadWebAPIFixtureDocument(t, "..\\fixtures\\web-api\\ok.setup-admin.yaml")
	loginFixture := loadWebAPIFixtureDocument(t, "..\\fixtures\\web-api\\ok.session-login.yaml")

	setup := performJSONRequest(t, application, setupFixture.Request.Method, setupFixture.Request.Path, setupFixture.Request.Body)
	if setup.Code != setupFixture.Response.Status {
		t.Fatalf("unexpected bootstrap status: got %d want %d", setup.Code, setupFixture.Response.Status)
	}

	login := performJSONRequest(t, application, loginFixture.Request.Method, loginFixture.Request.Path, loginFixture.Request.Body)
	if login.Code != loginFixture.Response.Status {
		t.Fatalf("unexpected login status: got %d want %d", login.Code, loginFixture.Response.Status)
	}

	body := decodeBody(t, login.Body.Bytes())
	token, ok := body["session_token"].(string)
	if !ok || token == "" {
		t.Fatalf("expected opaque session_token, got %#v", body["session_token"])
	}

	return token
}

func dialEventsWebSocket(t *testing.T, baseURL, token string) *websocket.Conn {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	conn, response, err := websocket.Dial(ctx, websocketURL(baseURL)+"/ws/events?session_token="+token, nil)
	if err != nil {
		if response == nil {
			t.Fatalf("dial websocket: %v", err)
		}
		t.Fatalf("dial websocket returned status %d: %v", response.StatusCode, err)
	}

	return conn
}

func websocketURL(httpURL string) string {
	if strings.HasPrefix(httpURL, "https://") {
		return "wss://" + strings.TrimPrefix(httpURL, "https://")
	}
	return "ws://" + strings.TrimPrefix(httpURL, "http://")
}

func testBridgeEvent() adapter.NormalizedEvent {
	return adapter.NormalizedEvent{
		Kind:             adapter.EventKindMessageText,
		EventID:          "onebot11-message-1001",
		BotID:            "10001",
		SourceProtocol:   "onebot11",
		SourceAdapter:    "adapter.onebot11",
		EventType:        "message.group",
		Timestamp:        time.Unix(1_700_000_123, 0).Unix(),
		ConversationType: "group",
		ConversationID:   "2001",
		SenderID:         "3001",
		PlainText:        "hello bridge",
	}
}

func waitForObservabilitySubscriber(t *testing.T, eventBridge *bridge.Bridge) {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if eventBridge.ObservabilitySubscriberCount() > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for websocket subscriber")
}
