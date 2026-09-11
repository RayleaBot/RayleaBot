package ws

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/app"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/tests/testutil"
	"github.com/coder/websocket"
)

func TestEventsWebSocketDeliversBridgeRuntimeFrame(t *testing.T) {
	t.Parallel()

	application, _, _ := newTestAppWithOptions(t, nil, func(options *app.Options, _ string) {
		options.BridgeDispatch = &eventsDispatchStub{
			deliverable: true,
			results: []dispatch.DeliveryResult{{
				PluginID: "weather",
				Outcome:  dispatch.OutcomeDelivered,
			}},
		}
	}, deterministicAuthOptions()...)
	eventBridge := application.Bridge()

	token := issueLoginToken(t, application)
	server := newManagementTestServer(t, application.Handler())
	defer server.Close()

	conn := dialEventsWebSocket(t, server.URL, token)
	defer func(release func(websocket.StatusCode, string) error) { _ = release(websocket.StatusNormalClosure, "") }(conn.Close)

	waitForObservabilitySubscriber(t, eventBridge)
	readProtocolReplayFrame(t, conn)

	outcome := eventBridge.HandleAdapterEvent(context.Background(), testBridgeEvent())
	if outcome != chatevent.DeliveryOutcomeDelivered {
		t.Fatalf("unexpected bridge outcome: got %q want %q", outcome, chatevent.DeliveryOutcomeDelivered)
	}

	frame := readEventsReplayFrameByKey(t, conn, "observability_scope")

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
	if data["last_supported_event_kind"] != string(chatevent.EventKindMessageText) {
		t.Fatalf("unexpected last_supported_event_kind: got %#v", data["last_supported_event_kind"])
	}
	if data["last_delivery_outcome"] != string(chatevent.DeliveryOutcomeDelivered) {
		t.Fatalf("unexpected last_delivery_outcome: got %#v", data["last_delivery_outcome"])
	}
	if data["delivered_count"] != float64(1) || data["result_count"] != float64(1) || data["error_count"] != float64(0) {
		t.Fatalf("unexpected aggregate counts: %#v", data)
	}

	rawBytes, err := json.Marshal(frame)
	if err != nil {
		t.Fatalf("marshal websocket frame: %v", err)
	}
	raw := string(rawBytes)
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

	application, _, _ := newTestAppWithOptions(t, nil, func(options *app.Options, _ string) {
		options.BridgeDispatch = &eventsDispatchStub{deliverable: true}
	}, deterministicAuthOptions()...)
	eventBridge := application.Bridge()

	token := issueLoginToken(t, application)
	server := newManagementTestServer(t, application.Handler())
	defer server.Close()

	conn := dialEventsWebSocket(t, server.URL, token)
	defer func(release func(websocket.StatusCode, string) error) { _ = release(websocket.StatusNormalClosure, "") }(conn.Close)

	waitForObservabilitySubscriber(t, eventBridge)
	firstStatus := readServiceStatusReplayFrame(t, conn)
	assertServiceStatusReplayFrame(t, firstStatus, "running")
	first := readProtocolReplayFrame(t, conn)
	assertProtocolReplayFrame(t, first, "adapters")
}

func TestEventsWebSocketReplaysServiceStatusOnConnect(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	server := newManagementTestServer(t, application.Handler())
	defer server.Close()

	conn := dialEventsWebSocket(t, server.URL, token)
	defer func(release func(websocket.StatusCode, string) error) { _ = release(websocket.StatusNormalClosure, "") }(conn.Close)

	frame := readServiceStatusReplayFrame(t, conn)
	assertServiceStatusReplayFrame(t, frame, "running")
}

func TestEventsWebSocketReplaysSameProtocolSnapshotAsHTTPHandler(t *testing.T) {
	t.Parallel()

	application, _, _ := newTestAppWithConfigMutation(t, func(input map[string]any) {
		// The instance switch is the master: a disabled instance runs no
		// transport, so the ingress needs both turned on.
		testutil.ConfigDocumentAdapterInstance(t, input, config.DefaultOneBot11AdapterID)["enabled"] = true
		onebot := testutil.ConfigDocumentOneBot(t, input)
		reverseWS := onebot["reverse_ws"].(map[string]any)
		reverseWS["enabled"] = true
		reverseWS["url"] = "ws://127.0.0.1:8080/onebot/reverse"
		reverseWS["access_token"] = "fixture-token"
	}, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	server := newManagementTestServer(t, application.Handler())
	defer server.Close()

	unauthorizedReq, err := http.NewRequest(http.MethodGet, server.URL+"/api/adapters/onebot11/reverse-ws", nil)
	if err != nil {
		t.Fatalf("create reverse websocket request: %v", err)
	}
	unauthorizedResp, err := server.Client().Do(unauthorizedReq)
	if err != nil {
		t.Fatalf("perform reverse websocket request: %v", err)
	}
	defer func(release func() error) { _ = release() }(unauthorizedResp.Body.Close)
	if unauthorizedResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unexpected reverse websocket status: got %d want %d", unauthorizedResp.StatusCode, http.StatusUnauthorized)
	}

	snapshotReq, err := http.NewRequest(http.MethodGet, server.URL+"/api/adapters", nil)
	if err != nil {
		t.Fatalf("create protocol snapshot request: %v", err)
	}
	snapshotReq.Header.Set("Authorization", "Bearer "+token)
	snapshotResp, err := server.Client().Do(snapshotReq)
	if err != nil {
		t.Fatalf("perform protocol snapshot request: %v", err)
	}
	defer func(release func() error) { _ = release() }(snapshotResp.Body.Close)
	if snapshotResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected protocol snapshot status: got %d want %d", snapshotResp.StatusCode, http.StatusOK)
	}
	httpSnapshot := decodeBody(t, readAll(t, snapshotResp))

	conn := dialEventsWebSocket(t, server.URL, token)
	defer func(release func(websocket.StatusCode, string) error) { _ = release(websocket.StatusNormalClosure, "") }(conn.Close)
	readServiceStatusReplayFrame(t, conn)
	first := readProtocolReplayFrame(t, conn)
	assertProtocolReplayFrame(t, first, "adapters")

	data, ok := first["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected websocket data object, got %#v", first["data"])
	}
	wsSnapshot, ok := data["adapters"].([]any)
	if !ok {
		t.Fatalf("expected websocket protocol snapshot object, got %#v", data["adapters"])
	}
	if !reflect.DeepEqual(wsSnapshot, httpSnapshot["adapters"]) {
		t.Fatalf("unexpected websocket protocol snapshot: got %#v want %#v", wsSnapshot, httpSnapshot)
	}
}

func TestEventsWebSocketDeliversPluginStateFrame(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	server := newManagementTestServer(t, application.Handler())
	defer server.Close()

	conn := dialEventsWebSocket(t, server.URL, token)
	defer func(release func(websocket.StatusCode, string) error) { _ = release(websocket.StatusNormalClosure, "") }(conn.Close)

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

	frame := readEventsReplayFrameByKey(t, conn, "plugin_id")

	data, ok := frame["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data object, got %#v", frame["data"])
	}
	if data["plugin_id"] != pluginID {
		t.Fatalf("unexpected plugin_id: got %#v want %q", data["plugin_id"], pluginID)
	}
	if data["state"] != "running" {
		t.Fatalf("unexpected state: got %#v want %q", data["state"], "running")
	}
}

func TestEventsWebSocketPublishesStoppingServiceStatusAfterShutdownRequest(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	server := newManagementTestServer(t, application.Handler())
	defer server.Close()

	conn := dialEventsWebSocket(t, server.URL, token)
	defer func(release func(websocket.StatusCode, string) error) { _ = release(websocket.StatusNormalClosure, "") }(conn.Close)

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
	defer func(release func() error) { _ = release() }(response.Body.Close)
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
	server := newManagementTestServer(t, application.Handler())
	defer server.Close()

	conn := dialEventsWebSocket(t, server.URL, token)
	defer func(release func(websocket.StatusCode, string) error) { _ = release(websocket.StatusNormalClosure, "") }(conn.Close)

	readServiceStatusReplayFrame(t, conn)
	readProtocolReplayFrame(t, conn)

	request, err := http.NewRequest(http.MethodPost, server.URL+"/api/governance/blacklist/entries", strings.NewReader(`{
  "entry_type": "user",
  "target_id": "1001",
  "reason": "spam",
  "scope": {
    "kind":"global",
    "source_protocol": "onebot11",
    "source_adapter": "",
    "bot_id": ""
  }
}`))
	if err != nil {
		t.Fatalf("create governance request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform governance request: %v", err)
	}
	defer func(release func() error) { _ = release() }(response.Body.Close)
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
	server := newManagementTestServer(t, application.Handler())
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	conn, response, err := websocket.Dial(ctx, websocketURL(server.URL)+"/ws/events", &websocket.DialOptions{
		Host:       testManagementAuthority,
		HTTPHeader: http.Header{"Origin": []string{testManagementOrigin}},
	})
	if conn != nil {
		_ = conn.Close(websocket.StatusNormalClosure, "")
	}
	if err == nil {
		t.Fatalf("expected unauthorized websocket dial to fail")
	}
	if response == nil || response.StatusCode != http.StatusUnauthorized {
		if response == nil {
			t.Fatalf("expected unauthorized response, got nil")
			return
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

func (s *eventsDispatchStub) Dispatch(context.Context, chatevent.Event, string) []dispatch.DeliveryResult {
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
	return dialProtectedWebSocket(t, baseURL, "/ws/events", token)
}

func websocketURL(httpURL string) string {
	if strings.HasPrefix(httpURL, "https://") {
		return "wss://" + strings.TrimPrefix(httpURL, "https://")
	}
	return "ws://" + strings.TrimPrefix(httpURL, "http://")
}

func testBridgeEvent() chatevent.NormalizedEvent {
	return chatevent.NormalizedEvent{
		Kind:             chatevent.EventKindMessageText,
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
	return readEventsReplayFrameByKey(t, conn, "adapters")
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

func TestEventsWebSocketApplicationCloseReleasesConnectionAndSubscriptions(t *testing.T) {
	application := newTestApp(t, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	server := newManagementTestServer(t, application.Handler())
	defer server.Close()
	initialBridgeSubscribers := application.Bridge().ObservabilitySubscriberCount()
	initialPluginSubscribers := application.Plugins().SubscriberCount()
	conn := dialEventsWebSocket(t, server.URL, token)
	defer func() { _ = conn.CloseNow() }()
	readProtocolReplayFrame(t, conn)
	if application.Bridge().ObservabilitySubscriberCount() != initialBridgeSubscribers+1 || application.Plugins().SubscriberCount() != initialPluginSubscribers+1 {
		t.Fatal("connection did not subscribe to runtime sources")
	}
	if err := application.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for {
		_, _, err := conn.Read(ctx)
		if err == nil {
			continue
		}
		if errors.Is(err, context.DeadlineExceeded) {
			t.Fatal("application close left the WebSocket connected")
		}
		break
	}
	if application.Bridge().ObservabilitySubscriberCount() != 0 || application.Plugins().SubscriberCount() != 0 {
		t.Fatal("closed connection retained runtime subscriptions")
	}
}
