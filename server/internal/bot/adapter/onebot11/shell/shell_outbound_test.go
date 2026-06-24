package shell

import (
	"context"
	"errors"
	adapteroutbound "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/outbound"
	adapterbackoff "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/shell/backoff"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
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

func TestShellSendMessageWritesRichSegmentArray(t *testing.T) {

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
				"message_id": 11111,
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

	_, err := shell.SendMessage(context.Background(), adapteroutbound.OutboundMessageSend{
		TargetType: "group",
		TargetID:   "2001",
		Segments: []adapteroutbound.OutboundMessageSegment{
			{Type: "at", Data: map[string]any{"user_id": "3001"}},
			{Type: "text", Data: map[string]any{"text": " rich outbound"}},
			{Type: "image", Data: map[string]any{"url": "https://example.test/rich.png"}},
		},
	})
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	var request map[string]any
	select {
	case request = <-requests:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for rich send_msg request")
	}

	params := request["params"].(map[string]any)
	message := params["message"].([]any)
	if len(message) != 3 {
		t.Fatalf("unexpected message segment count: %#v", params["message"])
	}
	first := message[0].(map[string]any)
	if first["type"] != "at" {
		t.Fatalf("unexpected first rich segment: %#v", first)
	}
	second := message[1].(map[string]any)
	if second["type"] != "text" {
		t.Fatalf("unexpected second rich segment: %#v", second)
	}
	third := message[2].(map[string]any)
	if third["type"] != "image" {
		t.Fatalf("unexpected third rich segment: %#v", third)
	}
	thirdData := third["data"].(map[string]any)
	if thirdData["file"] != "https://example.test/rich.png" {
		t.Fatalf("unexpected rich image data: %#v", thirdData)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	if err := shell.Stop(stopCtx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestShellSendReplyMapsReplyTargetMissing(t *testing.T) {

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
			"retcode": 1404,
			"wording": "reply target missing",
			"echo":    request["echo"],
		}); err != nil {
			t.Errorf("wsjson.Write response failed: %v", err)
			return
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

	_, err := shell.SendReply(context.Background(), adapteroutbound.OutboundMessageReply{
		TargetType:       "group",
		TargetID:         "2001",
		ReplyToMessageID: "98765",
		Segments: []adapteroutbound.OutboundMessageSegment{{
			Type: "text",
			Data: map[string]any{"text": "reply text"},
		}},
	})
	if err == nil {
		t.Fatal("expected SendReply to fail")
	}

	var adapterErr *adapteroutbound.Error
	if !errors.As(err, &adapterErr) {
		t.Fatalf("expected *adapteroutbound.Error, got %T", err)
	}
	if adapterErr.Code != adapteroutbound.ErrorCodeReplyTargetMissing {
		t.Fatalf("unexpected adapter error code: got %q want %q", adapterErr.Code, adapteroutbound.ErrorCodeReplyTargetMissing)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	if err := shell.Stop(stopCtx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func newTestShell(cfg config.OneBotConfig, deps shellDeps) *Shell {
	deps.skipRuntimeInfo = true
	if deps.sleep == nil {
		deps.sleep = blockingSleep
	}
	if deps.connectTimeout <= 0 {
		deps.connectTimeout = 50 * time.Millisecond
	}
	if deps.connectTimeout < 500*time.Millisecond {
		deps.connectTimeout = 500 * time.Millisecond
	}
	if deps.backoff == nil {
		deps.backoff = adapterbackoff.NewWithDurations(10*time.Millisecond, 1, 10*time.Millisecond, 0, func() float64 { return 0.5 })
	}

	return newShell(cfg, defaultAdapterConfig(), slog.New(slog.NewJSONHandler(io.Discard, nil)), deps)
}

func oneBotForwardWS(url string) config.OneBotConfig {
	return oneBotForwardWSWithToken(url, "")
}

func oneBotForwardWSWithToken(url, accessToken string) config.OneBotConfig {
	return config.OneBotConfig{
		ForwardWS: config.OneBotTransportConfig{
			Enabled:     true,
			URL:         url,
			AccessToken: accessToken,
		},
	}
}

func defaultAdapterConfig() config.AdapterConfig {
	return config.AdapterConfig{
		ConnectTimeoutSeconds:   15,
		ReconnectInitialSeconds: 2,
		ReconnectMultiplier:     2,
		ReconnectMaxSeconds:     120,
		ReconnectJitterRatio:    0.2,
	}
}

func waitForState(t *testing.T, shell *Shell, want State, timeout time.Duration) {
	t.Helper()

	if timeout < 2*time.Second {
		timeout = 2 * time.Second
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if shell.Snapshot().State == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for state %s, got %s", want, shell.Snapshot().State)
}

func waitForSnapshot(t *testing.T, shell *Shell, timeout time.Duration, predicate func(Snapshot) bool) Snapshot {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		snapshot := shell.Snapshot()
		if predicate(snapshot) {
			return snapshot
		}
		time.Sleep(10 * time.Millisecond)
	}

	snapshot := shell.Snapshot()
	t.Fatalf("timed out waiting for snapshot predicate, last snapshot: %#v", snapshot)
	return Snapshot{}
}

func blockingSleep(ctx context.Context, _ time.Duration) error {
	<-ctx.Done()
	return ctx.Err()
}

func wsURL(raw string) string {
	return "ws" + strings.TrimPrefix(raw, "http")
}
