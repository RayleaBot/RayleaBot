package shell

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

func TestShellCanSendForwardWSQueryTokenInCompatibilityMode(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("unexpected Authorization header: %q", got)
		}
		if got := r.URL.Query().Get("access_token"); got != "test-token" {
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

	cfg := oneBotForwardWSWithToken(wsURL(server.URL), "test-token")
	cfg.ForwardWS.AccessTokenQueryCompat = true
	shell := newTestShell(cfg, shellDeps{
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
}

func TestDialURLKeepsAccessTokenOutOfQueryByDefault(t *testing.T) {
	t.Parallel()

	if got := dialURL("ws://127.0.0.1:6700/ws", "test-token", false); got != "ws://127.0.0.1:6700/ws" {
		t.Fatalf("dialURL() = %q, want original URL", got)
	}
	if got := dialURL("ws://127.0.0.1:6700/ws", "test-token", true); got != "ws://127.0.0.1:6700/ws?access_token=test-token" {
		t.Fatalf("dialURL() = %q, want query compatibility URL", got)
	}
}
