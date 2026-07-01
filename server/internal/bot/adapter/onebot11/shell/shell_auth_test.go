package shell

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/coder/websocket"
)

func TestShellDialUsesForwardWSQueryTokenCompatibility(t *testing.T) {
	t.Parallel()

	dialErr := errors.New("dial stopped")
	var gotURL string
	var gotAuth string
	cfg := oneBotForwardWSWithToken("ws://127.0.0.1:6700/ws", "test-token")
	cfg.ForwardWS.AccessTokenQueryCompat = true
	shell := newTestShell(cfg, shellDeps{
		dial: func(_ context.Context, rawURL string, opts *websocket.DialOptions) (*websocket.Conn, *http.Response, error) {
			gotURL = rawURL
			if opts != nil {
				gotAuth = opts.HTTPHeader.Get("Authorization")
			}
			return nil, nil, dialErr
		},
	})

	_, _, err := shell.dial(context.Background())
	if !errors.Is(err, dialErr) {
		t.Fatalf("dial error = %v, want %v", err, dialErr)
	}
	if gotURL != "ws://127.0.0.1:6700/ws?access_token=test-token" {
		t.Fatalf("dial URL = %q, want compatibility token query", gotURL)
	}
	if gotAuth != "Bearer test-token" {
		t.Fatalf("Authorization = %q, want bearer token header", gotAuth)
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
