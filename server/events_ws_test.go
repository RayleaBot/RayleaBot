package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

func TestEventsWebSocketDeliversBridgeRuntimeFrame(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	eventBridge := bridge.New(application.Logger(), &eventsDispatchStub{
		deliverable: true,
		results: []dispatch.DeliveryResult{{
			PluginID: "weather",
			Outcome:  dispatch.OutcomeDelivered,
		}},
	})
	application.SetBridge(eventBridge)

	token := issueLoginToken(t, application)
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	conn := dialEventsWebSocket(t, server.URL, token)
	defer conn.Close(websocket.StatusNormalClosure, "")

	waitForObservabilitySubscriber(t, eventBridge)
	readProtocolReplayFrame(t, conn)

	outcome := eventBridge.HandleAdapterEvent(context.Background(), testBridgeEvent())
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

func TestEventsWebSocketReplaysProtocolStateOnConnect(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	eventBridge := bridge.New(application.Logger(), &eventsDispatchStub{
		deliverable: true,
	})
	application.SetBridge(eventBridge)

	token := issueLoginToken(t, application)
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	conn := dialEventsWebSocket(t, server.URL, token)
	defer conn.Close(websocket.StatusNormalClosure, "")

	waitForObservabilitySubscriber(t, eventBridge)
	firstStatus := readServiceStatusReplayFrame(t, conn)
	assertServiceStatusReplayFrame(t, firstStatus, "running")
	first := readProtocolReplayFrame(t, conn)
	assertProtocolReplayFrame(t, first, "protocol_snapshot")
}

func TestEventsWebSocketReplaysServiceStatusOnConnect(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	conn := dialEventsWebSocket(t, server.URL, token)
	defer conn.Close(websocket.StatusNormalClosure, "")

	frame := readServiceStatusReplayFrame(t, conn)
	assertServiceStatusReplayFrame(t, frame, "running")
}

func TestEventsWebSocketReplaysSameProtocolSnapshotAsHTTPHandler(t *testing.T) {
	t.Parallel()

	application, _, _ := newTestAppWithConfigMutation(t, func(input map[string]any) {
		onebot := input["onebot"].(map[string]any)
		reverseWS := onebot["reverse_ws"].(map[string]any)
		reverseWS["enabled"] = true
		reverseWS["url"] = "ws://127.0.0.1:8080/onebot/reverse"
		reverseWS["access_token"] = "fixture-token"
	}, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	unauthorizedReq, err := http.NewRequest(http.MethodGet, server.URL+"/api/protocols/onebot11/reverse-ws", nil)
	if err != nil {
		t.Fatalf("create reverse websocket request: %v", err)
	}
	unauthorizedResp, err := server.Client().Do(unauthorizedReq)
	if err != nil {
		t.Fatalf("perform reverse websocket request: %v", err)
	}
	defer unauthorizedResp.Body.Close()
	if unauthorizedResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unexpected reverse websocket status: got %d want %d", unauthorizedResp.StatusCode, http.StatusUnauthorized)
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
	if snapshotResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected protocol snapshot status: got %d want %d", snapshotResp.StatusCode, http.StatusOK)
	}
	httpSnapshot := decodeBody(t, readAll(t, snapshotResp))

	conn := dialEventsWebSocket(t, server.URL, token)
	defer conn.Close(websocket.StatusNormalClosure, "")
	readServiceStatusReplayFrame(t, conn)
	first := readProtocolReplayFrame(t, conn)
	assertProtocolReplayFrame(t, first, "protocol_snapshot")

	data, ok := first["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected websocket data object, got %#v", first["data"])
	}
	wsSnapshot, ok := data["protocol_snapshot"].(map[string]any)
	if !ok {
		t.Fatalf("expected websocket protocol snapshot object, got %#v", data["protocol_snapshot"])
	}
	if !reflect.DeepEqual(wsSnapshot, httpSnapshot) {
		t.Fatalf("unexpected websocket protocol snapshot: got %#v want %#v", wsSnapshot, httpSnapshot)
	}
}

func TestEventsWebSocketDeliversPluginStateFrame(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	conn := dialEventsWebSocket(t, server.URL, token)
	defer conn.Close(websocket.StatusNormalClosure, "")

	waitForPluginSubscriber(t, application.Plugins())
	readServiceStatusReplayFrame(t, conn)
	readProtocolReplayFrame(t, conn)

	snapshots := application.Plugins().List()
	if len(snapshots) == 0 {
		t.Fatal("expected at least one plugin snapshot")
	}
	pluginID := snapshots[0].PluginID
	if _, err := application.Plugins().SetRuntimeState(pluginID, "running"); err != nil {
		t.Fatalf("SetRuntimeState returned error: %v", err)
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

	data, ok := frame["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data object, got %#v", frame["data"])
	}
	if data["plugin_id"] != pluginID {
		t.Fatalf("unexpected plugin_id: got %#v want %q", data["plugin_id"], pluginID)
	}
	if data["runtime_state"] != "running" {
		t.Fatalf("unexpected runtime_state: got %#v want %q", data["runtime_state"], "running")
	}
	if data["display_state"] != "running" {
		t.Fatalf("unexpected display_state: got %#v want %q", data["display_state"], "running")
	}
}

func TestEventsWebSocketPublishesStoppingServiceStatusAfterShutdownRequest(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	conn := dialEventsWebSocket(t, server.URL, token)
	defer conn.Close(websocket.StatusNormalClosure, "")

	readServiceStatusReplayFrame(t, conn)
	readProtocolReplayFrame(t, conn)

	request, err := http.NewRequest(http.MethodPost, server.URL+"/api/system/shutdown", nil)
	if err != nil {
		t.Fatalf("create shutdown request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform shutdown request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("unexpected shutdown status: got %d want %d", response.StatusCode, http.StatusAccepted)
	}

	frame := readServiceStatusReplayFrame(t, conn)
	assertServiceStatusReplayFrame(t, frame, "stopping")
}

func TestEventsWebSocketPublishesGovernanceChangedAfterGovernanceWrite(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	conn := dialEventsWebSocket(t, server.URL, token)
	defer conn.Close(websocket.StatusNormalClosure, "")

	readServiceStatusReplayFrame(t, conn)
	readProtocolReplayFrame(t, conn)

	request, err := http.NewRequest(http.MethodPost, server.URL+"/api/governance/blacklist/entries", strings.NewReader(`{"entry_type":"user","target_id":"1001","reason":"spam"}`))
	if err != nil {
		t.Fatalf("create governance request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform governance request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("unexpected governance status: got %d want %d", response.StatusCode, http.StatusOK)
	}

	frame := readEventsReplayFrameByKey(t, conn, "event_type")
	data, ok := frame["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data object, got %#v", frame["data"])
	}
	if data["event_type"] != "governance.changed" {
		t.Fatalf("unexpected governance event_type: %#v", data["event_type"])
	}
	if summary, ok := data["summary"].(string); !ok || summary == "" {
		t.Fatalf("expected governance summary, got %#v", data["summary"])
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

type eventsDispatchStub struct {
	deliverable bool
	results     []dispatch.DeliveryResult
}

func (s *eventsDispatchStub) HasDeliverablePlugins() bool {
	return s.deliverable
}

func (s *eventsDispatchStub) Dispatch(context.Context, runtime.Event, string) []dispatch.DeliveryResult {
	return append([]dispatch.DeliveryResult(nil), s.results...)
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

func readProtocolReplayFrame(t *testing.T, conn *websocket.Conn) map[string]any {
	return readEventsReplayFrameByKey(t, conn, "protocol_snapshot")
}

func readServiceStatusReplayFrame(t *testing.T, conn *websocket.Conn) map[string]any {
	return readEventsReplayFrameByKey(t, conn, "service_status")
}

func readEventsReplayFrameByKey(t *testing.T, conn *websocket.Conn, key string) map[string]any {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		readCtx, cancel := context.WithTimeout(context.Background(), time.Until(deadline))
		_, payload, err := conn.Read(readCtx)
		cancel()
		if err != nil {
			t.Fatalf("read websocket frame: %v", err)
		}

		var frame map[string]any
		if err := json.Unmarshal(payload, &frame); err != nil {
			t.Fatalf("unmarshal websocket frame: %v", err)
		}
		data, ok := frame["data"].(map[string]any)
		if !ok {
			t.Fatalf("expected data object, got %#v", frame["data"])
		}
		if _, ok := data[key]; ok {
			return frame
		}
	}

	t.Fatalf("timed out waiting for %s replay frame", key)
	return nil
}

func assertProtocolReplayFrame(t *testing.T, frame map[string]any, key string) {
	t.Helper()

	if frame["channel"] != "events" {
		t.Fatalf("unexpected channel: %#v", frame["channel"])
	}
	if frame["type"] != "events.received" {
		t.Fatalf("unexpected type: %#v", frame["type"])
	}
	data, ok := frame["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data object, got %#v", frame["data"])
	}
	if data["protocol"] != "onebot11" {
		t.Fatalf("unexpected protocol: %#v", data["protocol"])
	}
	if _, ok := data[key]; !ok {
		t.Fatalf("expected %s in replay payload: %#v", key, data)
	}
}

func assertServiceStatusReplayFrame(t *testing.T, frame map[string]any, wantStatus string) {
	t.Helper()

	if frame["channel"] != "events" {
		t.Fatalf("unexpected channel: %#v", frame["channel"])
	}
	if frame["type"] != "events.received" {
		t.Fatalf("unexpected type: %#v", frame["type"])
	}
	data, ok := frame["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data object, got %#v", frame["data"])
	}
	if data["service_status"] != wantStatus {
		t.Fatalf("unexpected service_status: got %#v want %q", data["service_status"], wantStatus)
	}
	if summary, ok := data["summary"].(string); !ok || summary == "" {
		t.Fatalf("expected non-empty summary, got %#v", data["summary"])
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

func waitForPluginSubscriber(t *testing.T, catalog interface{ SubscriberCount() int }) {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if catalog.SubscriberCount() > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for plugin subscriber")
}
