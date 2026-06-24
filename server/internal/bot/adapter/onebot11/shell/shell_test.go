package shell

import (
	"context"
	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/intake"
	adapteroutbound "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestShellReachesConnectedAfterReadyFrame(t *testing.T) {

	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("unexpected Authorization header: %q", got)
		}
		if got := r.URL.Query().Get("access_token"); got != "" {
			t.Errorf("unexpected access_token query parameter: %q", got)
		}

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

	shell := newTestShell(oneBotForwardWSWithToken(wsURL(server.URL), "test-token"), shellDeps{
		connectTimeout: 75 * time.Millisecond,
		sleep:          blockingSleep,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shell.Start(ctx)
	waitForState(t, shell, StateConnected, 500*time.Millisecond)

	snapshot := shell.Snapshot()
	if !snapshot.ReadyFrameSeen {
		t.Fatal("expected ready frame to be seen")
	}
	if snapshot.ConnectedAt == nil {
		t.Fatal("expected ConnectedAt to be populated")
	}
	if snapshot.TotalReceivedFrames != 1 {
		t.Fatalf("unexpected total frame count: got %d want 1", snapshot.TotalReceivedFrames)
	}
	if snapshot.InvalidReceivedFrames != 0 {
		t.Fatalf("unexpected invalid frame count: got %d want 0", snapshot.InvalidReceivedFrames)
	}
	if snapshot.LastFrameCategory != adapterintake.FrameCategoryLifecycleReady {
		t.Fatalf("unexpected last frame category: got %s want %s", snapshot.LastFrameCategory, adapterintake.FrameCategoryLifecycleReady)
	}
	if snapshot.LastFrameType != "meta.lifecycle.enable" {
		t.Fatalf("unexpected last frame type: got %q want %q", snapshot.LastFrameType, "meta.lifecycle.enable")
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	if err := shell.Stop(stopCtx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestShellAuthFailureStopsAtAuthFailed(t *testing.T) {

	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	shell := newTestShell(oneBotForwardWSWithToken(wsURL(server.URL), "bad-token"), shellDeps{
		connectTimeout: 50 * time.Millisecond,
		sleep:          blockingSleep,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shell.Start(ctx)
	waitForState(t, shell, StateAuthFailed, 500*time.Millisecond)

	time.Sleep(50 * time.Millisecond)
	if shell.Snapshot().State != StateAuthFailed {
		t.Fatalf("expected auth_failed to remain stable, got %s", shell.Snapshot().State)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	if err := shell.Stop(stopCtx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestShellDoesNotConnectDisabledConfiguredForwardTransport(t *testing.T) {

	t.Parallel()

	var attempts atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	shell := newTestShell(config.OneBotConfig{
		ForwardWS: config.OneBotTransportConfig{
			Enabled: false,
			URL:     wsURL(server.URL),
		},
	}, shellDeps{
		connectTimeout: 50 * time.Millisecond,
		sleep:          blockingSleep,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shell.Start(ctx)
	time.Sleep(120 * time.Millisecond)

	snapshot := shell.Snapshot()
	if attempts.Load() != 0 {
		t.Fatalf("unexpected websocket dial attempts: got %d want 0", attempts.Load())
	}
	if snapshot.ForwardWS.Enabled {
		t.Fatal("forward transport enabled = true, want false")
	}
	if !snapshot.ForwardWS.Configured {
		t.Fatal("forward transport configured = false, want true")
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	if err := shell.Stop(stopCtx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestShellReloadReconnectsWithNewForwardTransportAndKeepsSendUsable(t *testing.T) {

	t.Parallel()

	var firstServerConnections atomic.Int64
	firstServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		firstServerConnections.Add(1)

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
	defer firstServer.Close()

	requests := make(chan map[string]any, 1)
	secondServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
				"message_id": 67890,
			},
			"echo": request["echo"],
		}); err != nil {
			t.Errorf("wsjson.Write response failed: %v", err)
			return
		}

		<-r.Context().Done()
	}))
	defer secondServer.Close()

	shell := newTestShell(oneBotForwardWS(wsURL(firstServer.URL)), shellDeps{
		connectTimeout: 75 * time.Millisecond,
		sleep:          blockingSleep,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shell.Start(ctx)
	waitForState(t, shell, StateConnected, 500*time.Millisecond)
	if firstServerConnections.Load() != 1 {
		t.Fatalf("unexpected initial connection count: got %d want 1", firstServerConnections.Load())
	}

	if err := shell.Reload(oneBotForwardWS(wsURL(secondServer.URL)), defaultAdapterConfig()); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	waitForState(t, shell, StateConnected, 500*time.Millisecond)

	result, err := shell.SendMessage(context.Background(), adapteroutbound.OutboundMessageSend{
		TargetType: "group",
		TargetID:   "2001",
		Segments: []adapteroutbound.OutboundMessageSegment{{
			Type: "text",
			Data: map[string]any{"text": "hello after reload"},
		}},
	})
	if err != nil {
		t.Fatalf("SendMessage failed after reload: %v", err)
	}
	if result.MessageID != "67890" {
		t.Fatalf("unexpected message id after reload: got %q want %q", result.MessageID, "67890")
	}

	var request map[string]any
	select {
	case request = <-requests:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for send_msg request after reload")
	}

	if request["action"] != "send_msg" {
		t.Fatalf("unexpected request action after reload: %#v", request["action"])
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	if err := shell.Stop(stopCtx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestShellWaitsForReadyFrameWhileTrafficContinues(t *testing.T) {

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

		frames := []struct {
			delay time.Duration
			body  map[string]any
		}{
			{
				body: map[string]any{
					"post_type": "message",
				},
			},
			{
				delay: 40 * time.Millisecond,
				body: map[string]any{
					"post_type": "notice",
				},
			},
			{
				delay: 40 * time.Millisecond,
				body: map[string]any{
					"post_type":       "meta_event",
					"meta_event_type": "lifecycle",
					"sub_type":        "enable",
				},
			},
		}

		for _, frame := range frames {
			if frame.delay > 0 {
				time.Sleep(frame.delay)
			}
			if err := wsjson.Write(context.Background(), conn, frame.body); err != nil {
				t.Errorf("wsjson.Write failed: %v", err)
				return
			}
		}

		<-r.Context().Done()
	}))
	defer server.Close()

	shell := newTestShell(oneBotForwardWS(wsURL(server.URL)), shellDeps{
		connectTimeout: 150 * time.Millisecond,
		sleep:          blockingSleep,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shell.Start(ctx)
	waitForState(t, shell, StateConnected, 500*time.Millisecond)
	snapshot := waitForSnapshot(t, shell, 500*time.Millisecond, func(snapshot Snapshot) bool {
		return snapshot.TotalReceivedFrames == 3
	})
	if snapshot.InvalidReceivedFrames != 0 {
		t.Fatalf("unexpected invalid frame count: got %d want 0", snapshot.InvalidReceivedFrames)
	}
	if snapshot.LastFrameCategory != adapterintake.FrameCategoryLifecycleReady {
		t.Fatalf("unexpected last frame category: got %s want %s", snapshot.LastFrameCategory, adapterintake.FrameCategoryLifecycleReady)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	if err := shell.Stop(stopCtx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestShellHeartbeatUpdatesIntakeObservability(t *testing.T) {

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
			"interval":        1000,
			"self_id":         30003,
			"time":            30004,
			"status": map[string]any{
				"good":   true,
				"online": true,
			},
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

	snapshot := waitForSnapshot(t, shell, 500*time.Millisecond, func(snapshot Snapshot) bool {
		return snapshot.HeartbeatSeen
	})
	if snapshot.TotalReceivedFrames != 1 {
		t.Fatalf("unexpected total frame count: got %d want 1", snapshot.TotalReceivedFrames)
	}
	if snapshot.LastHeartbeatAt == nil {
		t.Fatal("expected LastHeartbeatAt to be populated")
	}
	if snapshot.LastFrameCategory != adapterintake.FrameCategoryHeartbeat {
		t.Fatalf("unexpected last frame category: got %s want %s", snapshot.LastFrameCategory, adapterintake.FrameCategoryHeartbeat)
	}
	if snapshot.LastFrameType != "meta.heartbeat" {
		t.Fatalf("unexpected last frame type: got %q want %q", snapshot.LastFrameType, "meta.heartbeat")
	}
	if snapshot.State != StateConnected {
		t.Fatalf("unexpected state after structured heartbeat: got %s want %s", snapshot.State, StateConnected)
	}
	if snapshot.BotID != "30003" {
		t.Fatalf("unexpected bot id from heartbeat: got %q want %q", snapshot.BotID, "30003")
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	if err := shell.Stop(stopCtx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestShellTreatsLifecycleConnectAsReadyAndKeepsSessionOpen(t *testing.T) {

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
			"sub_type":        "connect",
			"self_id":         30003,
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
	time.Sleep(150 * time.Millisecond)

	snapshot := shell.Snapshot()
	if snapshot.State != StateConnected {
		t.Fatalf("unexpected state: got %s want %s", snapshot.State, StateConnected)
	}
	if snapshot.LastFrameCategory != adapterintake.FrameCategoryLifecycleReady {
		t.Fatalf("unexpected last frame category: got %s want %s", snapshot.LastFrameCategory, adapterintake.FrameCategoryLifecycleReady)
	}
	if snapshot.LastFrameType != "meta.lifecycle.connect" {
		t.Fatalf("unexpected last frame type: got %q want %q", snapshot.LastFrameType, "meta.lifecycle.connect")
	}
	if snapshot.BotID != "30003" {
		t.Fatalf("unexpected bot id: got %q want %q", snapshot.BotID, "30003")
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	if err := shell.Stop(stopCtx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestShellAcceptsBinaryReadyFrame(t *testing.T) {

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

		if err := conn.Write(context.Background(), websocket.MessageBinary, []byte(`{"post_type":"meta_event","meta_event_type":"heartbeat","interval":1000}`)); err != nil {
			t.Errorf("conn.Write failed: %v", err)
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

	snapshot := waitForSnapshot(t, shell, 500*time.Millisecond, func(snapshot Snapshot) bool {
		return snapshot.HeartbeatSeen
	})
	if snapshot.TotalReceivedFrames != 1 {
		t.Fatalf("unexpected total frame count: got %d want 1", snapshot.TotalReceivedFrames)
	}
	if snapshot.InvalidReceivedFrames != 0 {
		t.Fatalf("unexpected invalid frame count: got %d want 0", snapshot.InvalidReceivedFrames)
	}
	if snapshot.LastFrameCategory != adapterintake.FrameCategoryHeartbeat {
		t.Fatalf("unexpected last frame category: got %s want %s", snapshot.LastFrameCategory, adapterintake.FrameCategoryHeartbeat)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	if err := shell.Stop(stopCtx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestShellInvalidFrameIncrementsInvalidCounter(t *testing.T) {

	t.Parallel()

	invalidSent := make(chan struct{})
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
		if err := conn.Write(context.Background(), websocket.MessageText, []byte("{")); err != nil {
			t.Errorf("conn.Write failed: %v", err)
			return
		}
		close(invalidSent)
	}))
	defer server.Close()

	shell := newTestShell(oneBotForwardWS(wsURL(server.URL)), shellDeps{
		connectTimeout: 75 * time.Millisecond,
		sleep:          blockingSleep,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shell.Start(ctx)

	select {
	case <-invalidSent:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for invalid frame to be sent")
	}

	waitForState(t, shell, StateReconnecting, 500*time.Millisecond)
	snapshot := waitForSnapshot(t, shell, 500*time.Millisecond, func(snapshot Snapshot) bool {
		return snapshot.InvalidReceivedFrames == 1
	})
	if snapshot.TotalReceivedFrames != 2 {
		t.Fatalf("unexpected total frame count: got %d want 2", snapshot.TotalReceivedFrames)
	}
	if snapshot.LastFrameCategory != adapterintake.FrameCategoryInvalid {
		t.Fatalf("unexpected last frame category: got %s want %s", snapshot.LastFrameCategory, adapterintake.FrameCategoryInvalid)
	}
	if snapshot.LastFrameType != "invalid" {
		t.Fatalf("unexpected last frame type: got %q want %q", snapshot.LastFrameType, "invalid")
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	if err := shell.Stop(stopCtx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestShellUnknownFrameIsClassifiedConservatively(t *testing.T) {

	t.Parallel()

	unknownSent := make(chan struct{})
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
			"status": "ok",
		}); err != nil {
			t.Errorf("wsjson.Write failed: %v", err)
			return
		}
		close(unknownSent)

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
	case <-unknownSent:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for unknown frame to be sent")
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
	if snapshot.LastFrameCategory != adapterintake.FrameCategoryUnknown {
		t.Fatalf("unexpected last frame category: got %s want %s", snapshot.LastFrameCategory, adapterintake.FrameCategoryUnknown)
	}
	if snapshot.LastFrameType != "unknown" {
		t.Fatalf("unexpected last frame type: got %q want %q", snapshot.LastFrameType, "unknown")
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	if err := shell.Stop(stopCtx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestShellNonStringEchoDoesNotTriggerReconnect(t *testing.T) {

	t.Parallel()

	ignoredSent := make(chan struct{})
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
			"status":  "ok",
			"retcode": 0,
			"echo":    123,
		}); err != nil {
			t.Errorf("wsjson.Write failed: %v", err)
			return
		}
		close(ignoredSent)

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
	case <-ignoredSent:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for ignored api response")
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
	if snapshot.LastFrameCategory != adapterintake.FrameCategoryUnknown {
		t.Fatalf("unexpected last frame category: got %s want %s", snapshot.LastFrameCategory, adapterintake.FrameCategoryUnknown)
	}
	if snapshot.LastFrameType != "api.response.ignored" {
		t.Fatalf("unexpected last frame type: got %q want %q", snapshot.LastFrameType, "api.response.ignored")
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	if err := shell.Stop(stopCtx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestShellBlankEchoDoesNotTriggerReconnect(t *testing.T) {

	t.Parallel()

	ignoredSent := make(chan struct{})
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
			"status":  "ok",
			"retcode": 0,
			"echo":    "   ",
		}); err != nil {
			t.Errorf("wsjson.Write failed: %v", err)
			return
		}
		close(ignoredSent)

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
	case <-ignoredSent:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for ignored api response")
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
	if snapshot.LastFrameType != "api.response.ignored" {
		t.Fatalf("unexpected last frame type: got %q want %q", snapshot.LastFrameType, "api.response.ignored")
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	if err := shell.Stop(stopCtx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}
