package adapter

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"rayleabot/server/internal/config"
)

func TestShellReachesConnectedAfterReadyFrame(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer secret-token" {
			t.Errorf("unexpected Authorization header: %q", got)
		}
		if got := r.URL.Query().Get("access_token"); got != "secret-token" {
			t.Errorf("unexpected access_token query parameter: %q", got)
		}

		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("Accept failed: %v", err)
			return
		}
		defer conn.CloseNow()

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

	shell := newTestShell(config.OneBotConfig{
		WSURL:       wsURL(server.URL),
		AccessToken: "secret-token",
	}, shellDeps{
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

	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
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

	shell := newTestShell(config.OneBotConfig{
		WSURL:       wsURL(server.URL),
		AccessToken: "bad-token",
	}, shellDeps{
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

	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
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
		defer conn.CloseNow()

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

	shell := newTestShell(config.OneBotConfig{
		WSURL: wsURL(server.URL),
	}, shellDeps{
		connectTimeout: 50 * time.Millisecond,
		sleep:          blockingSleep,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shell.Start(ctx)
	waitForState(t, shell, StateConnected, 500*time.Millisecond)

	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
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
		defer conn.CloseNow()

		<-r.Context().Done()
	}))
	defer server.Close()

	shell := newTestShell(config.OneBotConfig{
		WSURL: wsURL(server.URL),
	}, shellDeps{
		connectTimeout: 40 * time.Millisecond,
		sleep:          blockingSleep,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shell.Start(ctx)
	waitForState(t, shell, StateReconnecting, 500*time.Millisecond)

	snapshot := shell.Snapshot()
	if snapshot.LastErrorCode != errorCodeConnectionFail {
		t.Fatalf("unexpected error code: got %q want %q", snapshot.LastErrorCode, errorCodeConnectionFail)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
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
		defer conn.CloseNow()

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

	shell := newTestShell(config.OneBotConfig{
		WSURL: wsURL(server.URL),
	}, shellDeps{
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
	if snapshot.LastErrorCode != errorCodeConnectionLost {
		t.Fatalf("unexpected error code: got %q want %q", snapshot.LastErrorCode, errorCodeConnectionLost)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
	defer stopCancel()
	if err := shell.Stop(stopCtx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestShellReconnectsWhenHeartbeatHasNotStartedAfterLifecycleEnable(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("Accept failed: %v", err)
			return
		}
		defer conn.CloseNow()

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

	shell := newTestShell(config.OneBotConfig{
		WSURL: wsURL(server.URL),
	}, shellDeps{
		connectTimeout: 40 * time.Millisecond,
		sleep:          blockingSleep,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shell.Start(ctx)
	waitForState(t, shell, StateConnected, 500*time.Millisecond)
	waitForState(t, shell, StateReconnecting, 500*time.Millisecond)

	snapshot := shell.Snapshot()
	if snapshot.LastErrorCode != errorCodeConnectionLost {
		t.Fatalf("unexpected error code: got %q want %q", snapshot.LastErrorCode, errorCodeConnectionLost)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
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
		defer conn.CloseNow()

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

	shell := newTestShell(config.OneBotConfig{
		WSURL: wsURL(server.URL),
	}, shellDeps{
		connectTimeout: 75 * time.Millisecond,
		sleep:          blockingSleep,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shell.Start(ctx)
	waitForState(t, shell, StateConnected, 500*time.Millisecond)
	waitForState(t, shell, StateReconnecting, 500*time.Millisecond)

	snapshot := shell.Snapshot()
	if snapshot.LastErrorCode != errorCodeConnectionLost {
		t.Fatalf("unexpected error code: got %q want %q", snapshot.LastErrorCode, errorCodeConnectionLost)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
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
		defer conn.CloseNow()

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

	shell := newTestShell(config.OneBotConfig{
		WSURL: wsURL(server.URL),
	}, shellDeps{
		connectTimeout: 75 * time.Millisecond,
		sleep:          blockingSleep,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shell.Start(ctx)
	waitForState(t, shell, StateConnected, 500*time.Millisecond)

	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
	defer stopCancel()
	if err := shell.Stop(stopCtx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	if shell.Snapshot().State != StateStopped {
		t.Fatalf("expected stopped state, got %s", shell.Snapshot().State)
	}
}

func newTestShell(cfg config.OneBotConfig, deps shellDeps) *Shell {
	if deps.sleep == nil {
		deps.sleep = blockingSleep
	}
	if deps.connectTimeout <= 0 {
		deps.connectTimeout = 50 * time.Millisecond
	}
	if deps.backoff == nil {
		deps.backoff = &Backoff{
			initial:    10 * time.Millisecond,
			max:        10 * time.Millisecond,
			multiplier: 1,
			jitter:     0,
			randFloat:  func() float64 { return 0.5 },
		}
	}

	return newShell(cfg, slog.New(slog.NewJSONHandler(io.Discard, nil)), deps)
}

func waitForState(t *testing.T, shell *Shell, want State, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if shell.Snapshot().State == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for state %s, got %s", want, shell.Snapshot().State)
}

func blockingSleep(ctx context.Context, _ time.Duration) error {
	<-ctx.Done()
	return ctx.Err()
}

func wsURL(raw string) string {
	return "ws" + strings.TrimPrefix(raw, "http")
}
