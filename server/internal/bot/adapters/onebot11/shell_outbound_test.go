package onebot11

import (
	"bufio"
	"context"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/adapters/reconnect"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestShellSendMessageWritesRichSegmentArray(t *testing.T) {

	t.Parallel()

	server, requests := newOneBotAPIServer(t, func(request map[string]any) map[string]any {
		return map[string]any{
			"status":  "ok",
			"retcode": 0,
			"data": map[string]any{
				"message_id": 11111,
			},
		}
	})

	shell := newTestShell(oneBotForwardWS(wsURL(server.URL)), shellDeps{
		sleep: blockingSleep,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shell.Start(ctx)
	waitForState(t, shell, StateConnected, 500*time.Millisecond)

	_, err := shell.SendMessage(context.Background(), chatevent.OutboundMessageSend{
		TargetType: "group",
		TargetID:   "2001",
		Segments: []chatevent.MessageSegment{
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

func newTestShell(cfg config.OneBotConfig, deps shellDeps) *Shell {
	deps.skipRuntimeInfo = true
	if deps.sleep == nil {
		deps.sleep = blockingSleep
	}
	if deps.backoff == nil {
		deps.backoff = reconnect.NewWithDurations(10*time.Millisecond, 1, 10*time.Millisecond, 0, func() float64 { return 0.5 })
	}

	return newShell("onebot11", cfg, defaultAdapterConfig(), slog.New(slog.NewJSONHandler(io.Discard, nil)), deps)
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

// newTestWebSocketDial keeps WebSocket I/O inside the synctest bubble so socket
// scheduling cannot consume the short handshake and read timeout budgets.
func newTestWebSocketDial(serve func(*websocket.Conn)) dialFunc {
	return func(ctx context.Context, url string, options *websocket.DialOptions) (*websocket.Conn, *http.Response, error) {
		opts := *options
		opts.HTTPClient = &http.Client{Transport: testWebSocketTransport{serve: serve}}
		return websocket.Dial(ctx, url, &opts)
	}
}

type testWebSocketTransport struct {
	serve func(*websocket.Conn)
}

func (transport testWebSocketTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	client, server := net.Pipe()
	writer := &testWebSocketResponseWriter{ResponseRecorder: httptest.NewRecorder(), conn: server}
	conn, err := websocket.Accept(writer, request, nil)
	if err != nil {
		_ = client.Close()
		_ = server.Close()
		return nil, err
	}
	go func() {
		defer func() { _ = conn.CloseNow() }()
		transport.serve(conn)
		<-conn.CloseRead(context.Background()).Done()
	}()
	response := writer.Result()
	response.Body = client
	return response, nil
}

type testWebSocketResponseWriter struct {
	*httptest.ResponseRecorder
	conn net.Conn
}

func (writer *testWebSocketResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return writer.conn, bufio.NewReadWriter(bufio.NewReader(writer.conn), bufio.NewWriter(writer.conn)), nil
}

func wsURL(raw string) string {
	return "ws" + strings.TrimPrefix(raw, "http")
}

// newOneBotAPIServer accepts forward WebSocket connections that report ready and
// answer each API request with the frame built by respond, carrying the
// request's echo so the shell can match it. Requests are also delivered to the
// returned channel while it has room.
func newOneBotAPIServer(t *testing.T, respond func(request map[string]any) map[string]any) (*httptest.Server, <-chan map[string]any) {
	t.Helper()

	requests := make(chan map[string]any, 8)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("Accept failed: %v", err)
			return
		}
		defer func() {
			_ = conn.CloseNow()
		}()

		if err := wsjson.Write(r.Context(), conn, map[string]any{
			"post_type":       "meta_event",
			"meta_event_type": "lifecycle",
			"sub_type":        "enable",
		}); err != nil {
			t.Errorf("wsjson.Write ready failed: %v", err)
			return
		}
		for {
			var request map[string]any
			if err := wsjson.Read(r.Context(), conn, &request); err != nil {
				return
			}
			select {
			case requests <- request:
			default:
			}
			response := respond(request)
			response["echo"] = request["echo"]
			if err := wsjson.Write(r.Context(), conn, response); err != nil {
				return
			}
		}
	}))
	t.Cleanup(server.Close)
	return server, requests
}
