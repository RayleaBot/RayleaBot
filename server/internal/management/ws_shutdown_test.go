package management

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/auth"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/console"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	pluginCatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/go-chi/chi/v5"
)

type replayLogStream struct{ *logging.Stream }

func (s replayLogStream) Replay(context.Context) []logging.Summary { return nil }

func TestLogAndConsoleShutdownClosesNormallyAndReleasesSubscriptions(t *testing.T) {
	logs := logging.NewStream(4)
	defer logs.Close()
	logs.Append(logging.Summary{Message: "fixture"})
	consoleStream := console.NewStream(4, 1024)
	consoleStream.Append(console.Entry{PluginID: "fixture", Stream: "stdout", Text: "fixture"})
	logHandler := NewLogsHandler(replayLogStream{logs})
	consoleHandler := NewConsoleHandler(consoleStream, pluginCatalog.New([]plugins.Snapshot{{PluginID: "fixture"}}))
	for _, tc := range []struct {
		name     string
		handler  http.HandlerFunc
		shutdown func(context.Context)
		count    func() int
	}{
		{"logs", logHandler.HandleLogsWebSocket(), logHandler.Shutdown, logs.SubscriberCount},
		{"console", consoleHandler.HandlePluginConsoleWebSocket(), consoleHandler.Shutdown, func() int { return consoleStream.SubscriberCount("fixture") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx := ContextWithClaims(r.Context(), auth.Claims{Subject: "fixture"})
				ctx = context.WithValue(ctx, webSocketOriginAuthorityKey{}, r.Host)
				route := chi.NewRouteContext()
				route.URLParams.Add("id", "fixture")
				ctx = context.WithValue(ctx, chi.RouteCtxKey, route)
				tc.handler(w, r.WithContext(ctx))
			}))
			defer server.Close()
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = conn.CloseNow() }()
			var initial any
			if err := wsjson.Read(ctx, conn, &initial); err != nil {
				t.Fatal(err)
			}
			closed := make(chan error, 1)
			go func() { _, _, err := conn.Read(ctx); closed <- err }()
			shutdownCtx, stop := context.WithTimeout(t.Context(), 200*time.Millisecond)
			defer stop()
			tc.shutdown(shutdownCtx)
			if err := <-closed; websocket.CloseStatus(err) != websocket.StatusNormalClosure {
				t.Fatalf("close = %v", err)
			}
			if tc.count() != 0 {
				t.Fatal("shutdown retained subscription")
			}
			tc.shutdown(shutdownCtx)
		})
	}
}

func TestStreamShutdownBoundsBlockedWritesBySharedDeadline(t *testing.T) {
	var owner websocketStreams
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.CloseNow() }()
		owner.run(conn, func(ctx context.Context) {
			close(started)
			payload := []byte(strings.Repeat("x", 16<<20))
			for {
				if err := conn.Write(ctx, websocket.MessageBinary, payload); err != nil {
					return
				}
			}
		})
	}))
	defer server.Close()
	conn, _, err := websocket.Dial(t.Context(), "ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.CloseNow() }()
	<-started
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	done := make(chan struct{})
	go func() { owner.shutdown(ctx); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("blocked peer extended shutdown")
	}
}
