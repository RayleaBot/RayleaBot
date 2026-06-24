package shell

import (
	"context"
	"encoding/json"
	"errors"
	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/intake"
	adapteroutbound "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestShellEventFrameIsConsumedWithoutSideEffects(t *testing.T) {

	t.Parallel()

	eventSent := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("Accept failed: %v", err)
			return
		}
		defer func() {
			_ = conn.CloseNow()
		}()

		if err := wsjson.Write(context.Background(), conn, map[string]any{
			"post_type":       "meta_event",
			"meta_event_type": "lifecycle",
			"sub_type":        "enable",
		}); err != nil {
			t.Errorf("wsjson.Write failed: %v", err)
			return
		}
		if err := wsjson.Write(context.Background(), conn, map[string]any{
			"post_type": "message",
		}); err != nil {
			t.Errorf("wsjson.Write failed: %v", err)
			return
		}
		close(eventSent)

		<-r.Context().Done()
	}))
	defer server.Close()

	shell := newTestShell(oneBotForwardWS(wsURL(server.URL)), shellDeps{
		connectTimeout: 500 * time.Millisecond,
		sleep:          blockingSleep,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shell.Start(ctx)
	waitForState(t, shell, StateConnected, 500*time.Millisecond)

	select {
	case <-eventSent:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event frame to be sent")
	}

	snapshot := waitForSnapshot(t, shell, 500*time.Millisecond, func(snapshot Snapshot) bool {
		return snapshot.TotalReceivedFrames == 2
	})
	if snapshot.State != StateConnected {
		t.Fatalf("unexpected state: got %s want %s", snapshot.State, StateConnected)
	}
	if snapshot.InvalidReceivedFrames != 0 {
		t.Fatalf("unexpected invalid frame count: got %d want 0", snapshot.InvalidReceivedFrames)
	}
	if snapshot.LastFrameCategory != adapterintake.FrameCategoryEvent {
		t.Fatalf("unexpected last frame category: got %s want %s", snapshot.LastFrameCategory, adapterintake.FrameCategoryEvent)
	}
	if snapshot.LastFrameType != "message" {
		t.Fatalf("unexpected last frame type: got %q want %q", snapshot.LastFrameType, "message")
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	if err := shell.Stop(stopCtx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestShellReconnectsWhenReadyFrameTimesOut(t *testing.T) {

	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("Accept failed: %v", err)
			return
		}
		defer func() {
			_ = conn.CloseNow()
		}()

		<-r.Context().Done()
	}))
	defer server.Close()

	shell := newTestShell(oneBotForwardWS(wsURL(server.URL)), shellDeps{
		connectTimeout: 40 * time.Millisecond,
		sleep:          blockingSleep,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shell.Start(ctx)
	waitForState(t, shell, StateReconnecting, 500*time.Millisecond)

	snapshot := shell.Snapshot()
	if snapshot.LastErrorCode != errorCodeForwardWSConnectFail {
		t.Fatalf("unexpected error code: got %q want %q", snapshot.LastErrorCode, errorCodeForwardWSConnectFail)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	if err := shell.Stop(stopCtx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestShellReconnectsAfterConnectionLoss(t *testing.T) {

	t.Parallel()

	closeConn := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("Accept failed: %v", err)
			return
		}
		defer func() {
			_ = conn.CloseNow()
		}()

		if err := wsjson.Write(context.Background(), conn, map[string]any{
			"post_type":       "meta_event",
			"meta_event_type": "lifecycle",
			"sub_type":        "enable",
		}); err != nil {
			t.Errorf("wsjson.Write failed: %v", err)
			return
		}

		select {
		case <-closeConn:
			_ = conn.Close(websocket.StatusNormalClosure, "")
		case <-r.Context().Done():
		}
	}))
	defer server.Close()

	shell := newTestShell(oneBotForwardWS(wsURL(server.URL)), shellDeps{
		connectTimeout: 75 * time.Millisecond,
		sleep:          blockingSleep,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shell.Start(ctx)
	waitForState(t, shell, StateConnected, 500*time.Millisecond)
	close(closeConn)
	waitForState(t, shell, StateReconnecting, 500*time.Millisecond)

	snapshot := shell.Snapshot()
	if snapshot.LastErrorCode != errorCodeForwardWSSessionLost {
		t.Fatalf("unexpected error code: got %q want %q", snapshot.LastErrorCode, errorCodeForwardWSSessionLost)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	if err := shell.Stop(stopCtx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestShellKeepsConnectionOpenWhenHeartbeatHasNotStartedAfterLifecycleEnable(t *testing.T) {

	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("Accept failed: %v", err)
			return
		}
		defer func() {
			_ = conn.CloseNow()
		}()

		if err := wsjson.Write(context.Background(), conn, map[string]any{
			"post_type":       "meta_event",
			"meta_event_type": "lifecycle",
			"sub_type":        "enable",
		}); err != nil {
			t.Errorf("wsjson.Write failed: %v", err)
			return
		}

		<-r.Context().Done()
	}))
	defer server.Close()

	shell := newTestShell(oneBotForwardWS(wsURL(server.URL)), shellDeps{
		connectTimeout: 40 * time.Millisecond,
		sleep:          blockingSleep,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shell.Start(ctx)
	waitForState(t, shell, StateConnected, 500*time.Millisecond)
	time.Sleep(120 * time.Millisecond)

	snapshot := shell.Snapshot()
	if snapshot.State != StateConnected {
		t.Fatalf("unexpected state: got %s want %s", snapshot.State, StateConnected)
	}
	if snapshot.LastErrorCode != "" {
		t.Fatalf("unexpected error code: got %q want empty", snapshot.LastErrorCode)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	if err := shell.Stop(stopCtx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestShellReconnectsAfterHeartbeatTimeout(t *testing.T) {

	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("Accept failed: %v", err)
			return
		}
		defer func() {
			_ = conn.CloseNow()
		}()

		if err := wsjson.Write(context.Background(), conn, map[string]any{
			"post_type":       "meta_event",
			"meta_event_type": "heartbeat",
			"interval":        20,
		}); err != nil {
			t.Errorf("wsjson.Write failed: %v", err)
			return
		}

		<-r.Context().Done()
	}))
	defer server.Close()

	shell := newTestShell(oneBotForwardWS(wsURL(server.URL)), shellDeps{
		connectTimeout: 75 * time.Millisecond,
		sleep:          blockingSleep,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shell.Start(ctx)
	waitForState(t, shell, StateConnected, 500*time.Millisecond)
	waitForState(t, shell, StateReconnecting, 500*time.Millisecond)

	snapshot := shell.Snapshot()
	if snapshot.LastErrorCode != errorCodeForwardWSSessionLost {
		t.Fatalf("unexpected error code: got %q want %q", snapshot.LastErrorCode, errorCodeForwardWSSessionLost)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	if err := shell.Stop(stopCtx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestShellStopTransitionsToStopped(t *testing.T) {

	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("Accept failed: %v", err)
			return
		}
		defer func() {
			_ = conn.CloseNow()
		}()

		if err := wsjson.Write(context.Background(), conn, map[string]any{
			"post_type":       "meta_event",
			"meta_event_type": "lifecycle",
			"sub_type":        "enable",
		}); err != nil {
			t.Errorf("wsjson.Write failed: %v", err)
			return
		}

		<-r.Context().Done()
	}))
	defer server.Close()

	shell := newTestShell(oneBotForwardWS(wsURL(server.URL)), shellDeps{
		connectTimeout: 75 * time.Millisecond,
		sleep:          blockingSleep,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shell.Start(ctx)
	waitForState(t, shell, StateConnected, 500*time.Millisecond)

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	if err := shell.Stop(stopCtx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	if shell.Snapshot().State != StateStopped {
		t.Fatalf("expected stopped state, got %s", shell.Snapshot().State)
	}
}

func TestShellStopWaitsForReverseWebSocketAndStoppedLog(t *testing.T) {

	t.Parallel()

	logStream := logging.NewStream(16)
	logger := slog.New(slog.NewJSONHandler(logging.NewSummaryWriter(io.Discard, logStream, nil), &slog.HandlerOptions{
		ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
			switch attr.Key {
			case slog.TimeKey:
				attr.Key = "ts"
			case slog.MessageKey:
				attr.Key = "msg"
			}
			return attr
		},
	}))
	shell := newShell(config.OneBotConfig{
		ReverseWS: config.OneBotTransportConfig{
			Enabled: true,
			URL:     "ws://127.0.0.1:8080/onebot/reverse",
		},
	}, defaultAdapterConfig(), logger, shellDeps{
		connectTimeout: 75 * time.Millisecond,
		sleep:          blockingSleep,
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("Accept failed: %v", err)
			return
		}
		shell.AttachReverseWS(conn)
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	shell.Start(ctx)
	waitForSnapshot(t, shell, 500*time.Millisecond, func(snapshot Snapshot) bool {
		return snapshot.ReverseWS.State == TransportStateListening
	})

	client, _, err := websocket.Dial(context.Background(), wsURL(server.URL), nil)
	if err != nil {
		t.Fatalf("Dial reverse websocket failed: %v", err)
	}
	defer func() {
		_ = client.CloseNow()
	}()
	if err := wsjson.Write(context.Background(), client, map[string]any{
		"post_type":       "meta_event",
		"meta_event_type": "lifecycle",
		"sub_type":        "enable",
	}); err != nil {
		t.Fatalf("write reverse ready frame: %v", err)
	}
	waitForState(t, shell, StateConnected, 500*time.Millisecond)

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	if err := shell.Stop(stopCtx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	snapshot := shell.Snapshot()
	if snapshot.State != StateStopped {
		t.Fatalf("expected stopped state, got %s", snapshot.State)
	}
	if snapshot.ReverseWS.State != TransportStateStopped {
		t.Fatalf("expected stopped reverse websocket, got %s", snapshot.ReverseWS.State)
	}
	for _, transport := range snapshot.ActiveTransports {
		if transport == TransportReverseWS {
			t.Fatalf("reverse websocket remained active after stop: %#v", snapshot.ActiveTransports)
		}
	}

	for _, summary := range logStream.Snapshot() {
		if summary.Source == "adapter" && summary.Message == "adapter shell stopped" {
			return
		}
	}
	t.Fatalf("expected stopped adapter log, got %#v", logStream.Snapshot())
}

func TestShellRestartWithoutConfiguredForwardTransportReturnsToIdle(t *testing.T) {

	t.Parallel()

	shell := newTestShell(config.OneBotConfig{}, shellDeps{
		connectTimeout: 75 * time.Millisecond,
		sleep:          blockingSleep,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shell.Start(ctx)
	waitForState(t, shell, StateIdle, 200*time.Millisecond)

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	if err := shell.Stop(stopCtx); err != nil {
		t.Fatalf("first Stop failed: %v", err)
	}
	if shell.Snapshot().State != StateStopped {
		t.Fatalf("expected stopped state after first stop, got %s", shell.Snapshot().State)
	}

	shell.Start(ctx)
	snapshot := waitForSnapshot(t, shell, 200*time.Millisecond, func(snapshot Snapshot) bool {
		return snapshot.State == StateIdle
	})
	if snapshot.ForwardWS.State != TransportStateIdle {
		t.Fatalf("expected idle forward transport after restart, got %s", snapshot.ForwardWS.State)
	}
	if snapshot.LastErrorCode != "" {
		t.Fatalf("expected cleared adapter error after restart, got %q", snapshot.LastErrorCode)
	}

	secondStopCtx, secondStopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer secondStopCancel()
	if err := shell.Stop(secondStopCtx); err != nil {
		t.Fatalf("second Stop failed: %v", err)
	}
}

func TestShellSendMessageWritesSendMsgRequestAndReturnsMessageID(t *testing.T) {

	t.Parallel()

	requests := make(chan map[string]any, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("Accept failed: %v", err)
			return
		}
		defer func() {
			_ = conn.CloseNow()
		}()

		if err := wsjson.Write(context.Background(), conn, map[string]any{
			"post_type":       "meta_event",
			"meta_event_type": "lifecycle",
			"sub_type":        "enable",
		}); err != nil {
			t.Errorf("wsjson.Write ready failed: %v", err)
			return
		}

		var request map[string]any
		if err := wsjson.Read(context.Background(), conn, &request); err != nil {
			t.Errorf("wsjson.Read request failed: %v", err)
			return
		}
		requests <- request

		if err := wsjson.Write(context.Background(), conn, map[string]any{
			"status":  "ok",
			"retcode": 0,
			"data": map[string]any{
				"message_id": 12345,
			},
			"echo": request["echo"],
		}); err != nil {
			t.Errorf("wsjson.Write response failed: %v", err)
			return
		}

		<-r.Context().Done()
	}))
	defer server.Close()

	shell := newTestShell(oneBotForwardWS(wsURL(server.URL)), shellDeps{
		connectTimeout: 75 * time.Millisecond,
		sleep:          blockingSleep,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shell.Start(ctx)
	waitForState(t, shell, StateConnected, 500*time.Millisecond)

	result, err := shell.SendMessage(context.Background(), adapteroutbound.OutboundMessageSend{
		TargetType: "group",
		TargetID:   "2001",
		Segments: []adapteroutbound.OutboundMessageSegment{{
			Type: "text",
			Data: map[string]any{"text": "hello outbound"},
		}},
	})
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if result.MessageID != "12345" {
		t.Fatalf("unexpected message id: got %q want %q", result.MessageID, "12345")
	}

	var request map[string]any
	select {
	case request = <-requests:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for send_msg request")
	}

	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	if request["action"] != "send_msg" {
		t.Fatalf("unexpected request action: %#v", request["action"])
	}
	if _, ok := request["echo"].(string); !ok {
		t.Fatalf("expected string echo, got %#v", request["echo"])
	}
	params, ok := request["params"].(map[string]any)
	if !ok {
		t.Fatalf("expected params object, got %#v", request["params"])
	}
	if params["message_type"] != "group" {
		t.Fatalf("unexpected message_type: %#v", params["message_type"])
	}
	if params["group_id"] != float64(2001) {
		t.Fatalf("unexpected group_id: %#v", params["group_id"])
	}
	message, ok := params["message"].([]any)
	if !ok || len(message) != 1 {
		t.Fatalf("unexpected message payload: %#v", params["message"])
	}
	firstSegment, ok := message[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected first message segment: %#v", message[0])
	}
	if firstSegment["type"] != "text" {
		t.Fatalf("unexpected first segment type: %#v", firstSegment["type"])
	}
	firstData, ok := firstSegment["data"].(map[string]any)
	if !ok || firstData["text"] != "hello outbound" {
		t.Fatalf("unexpected first segment data: %#v", firstSegment["data"])
	}
	for _, forbidden := range []string{"plain_text", "event_id", "request_id", "target_type", "target_id"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("send_msg request leaked forbidden field %q: %s", forbidden, raw)
		}
	}
	if len(params) != 3 {
		t.Fatalf("unexpected params shape: %#v", params)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	if err := shell.Stop(stopCtx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestShellSendMessageReturnsAdapterSendFailed(t *testing.T) {

	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("Accept failed: %v", err)
			return
		}
		defer func() {
			_ = conn.CloseNow()
		}()

		if err := wsjson.Write(context.Background(), conn, map[string]any{
			"post_type":       "meta_event",
			"meta_event_type": "lifecycle",
			"sub_type":        "enable",
		}); err != nil {
			t.Errorf("wsjson.Write ready failed: %v", err)
			return
		}

		var request map[string]any
		if err := wsjson.Read(context.Background(), conn, &request); err != nil {
			t.Errorf("wsjson.Read request failed: %v", err)
			return
		}
		if err := wsjson.Write(context.Background(), conn, map[string]any{
			"status":  "failed",
			"retcode": 1200,
			"wording": "send failed",
			"echo":    request["echo"],
		}); err != nil {
			t.Errorf("wsjson.Write response failed: %v", err)
			return
		}

		<-r.Context().Done()
	}))
	defer server.Close()

	shell := newTestShell(oneBotForwardWS(wsURL(server.URL)), shellDeps{
		connectTimeout: 75 * time.Millisecond,
		sleep:          blockingSleep,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shell.Start(ctx)
	waitForState(t, shell, StateConnected, 500*time.Millisecond)

	_, err := shell.SendMessage(context.Background(), adapteroutbound.OutboundMessageSend{
		TargetType: "private",
		TargetID:   "3001",
		Segments: []adapteroutbound.OutboundMessageSegment{{
			Type: "text",
			Data: map[string]any{"text": "hello outbound"},
		}},
	})
	if err == nil {
		t.Fatal("expected SendMessage to fail")
	}
	var adapterErr *adapteroutbound.Error
	if !errors.As(err, &adapterErr) {
		t.Fatalf("expected *adapteroutbound.Error, got %T", err)
	}
	if adapterErr.Code != adapteroutbound.ErrorCodeSendFailed {
		t.Fatalf("unexpected adapter error code: got %q want %q", adapterErr.Code, adapteroutbound.ErrorCodeSendFailed)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	if err := shell.Stop(stopCtx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestShellSendReplyWritesReplySegmentRequestAndReturnsMessageID(t *testing.T) {

	t.Parallel()

	requests := make(chan map[string]any, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("Accept failed: %v", err)
			return
		}
		defer func() {
			_ = conn.CloseNow()
		}()

		if err := wsjson.Write(context.Background(), conn, map[string]any{
			"post_type":       "meta_event",
			"meta_event_type": "lifecycle",
			"sub_type":        "enable",
		}); err != nil {
			t.Errorf("wsjson.Write ready failed: %v", err)
			return
		}

		var request map[string]any
		if err := wsjson.Read(context.Background(), conn, &request); err != nil {
			t.Errorf("wsjson.Read request failed: %v", err)
			return
		}
		requests <- request

		if err := wsjson.Write(context.Background(), conn, map[string]any{
			"status":  "ok",
			"retcode": 0,
			"data": map[string]any{
				"message_id": 98765,
			},
			"echo": request["echo"],
		}); err != nil {
			t.Errorf("wsjson.Write response failed: %v", err)
			return
		}

		<-r.Context().Done()
	}))
	defer server.Close()

	shell := newTestShell(oneBotForwardWS(wsURL(server.URL)), shellDeps{
		connectTimeout: 75 * time.Millisecond,
		sleep:          blockingSleep,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shell.Start(ctx)
	waitForState(t, shell, StateConnected, 500*time.Millisecond)

	result, err := shell.SendReply(context.Background(), adapteroutbound.OutboundMessageReply{
		TargetType:       "group",
		TargetID:         "2001",
		ReplyToMessageID: "98765",
		Segments: []adapteroutbound.OutboundMessageSegment{{
			Type: "text",
			Data: map[string]any{"text": "reply text"},
		}},
	})
	if err != nil {
		t.Fatalf("SendReply failed: %v", err)
	}
	if result.MessageID != "98765" {
		t.Fatalf("unexpected message id: got %q want %q", result.MessageID, "98765")
	}

	var request map[string]any
	select {
	case request = <-requests:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for send_msg request")
	}

	if request["action"] != "send_msg" {
		t.Fatalf("unexpected request action: %#v", request["action"])
	}
	params, ok := request["params"].(map[string]any)
	if !ok {
		t.Fatalf("expected params object, got %#v", request["params"])
	}
	if params["message_type"] != "group" {
		t.Fatalf("unexpected message_type: %#v", params["message_type"])
	}
	if params["group_id"] != float64(2001) {
		t.Fatalf("unexpected group_id: %#v", params["group_id"])
	}
	message, ok := params["message"].([]any)
	if !ok || len(message) != 2 {
		t.Fatalf("unexpected message payload: %#v", params["message"])
	}
	replySegment, ok := message[0].(map[string]any)
	if !ok || replySegment["type"] != "reply" {
		t.Fatalf("unexpected reply segment: %#v", message[0])
	}
	replyData, ok := replySegment["data"].(map[string]any)
	if !ok || replyData["id"] != "98765" {
		t.Fatalf("unexpected reply segment data: %#v", replySegment["data"])
	}
	textSegment, ok := message[1].(map[string]any)
	if !ok || textSegment["type"] != "text" {
		t.Fatalf("unexpected text segment: %#v", message[1])
	}
	textData, ok := textSegment["data"].(map[string]any)
	if !ok || textData["text"] != "reply text" {
		t.Fatalf("unexpected text segment data: %#v", textSegment["data"])
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	if err := shell.Stop(stopCtx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}
