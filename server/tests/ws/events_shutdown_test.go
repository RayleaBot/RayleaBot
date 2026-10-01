package ws

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/RayleaBot/RayleaBot/server/internal/management"
	"github.com/RayleaBot/RayleaBot/server/tests/testutil"
)

func TestRunningAppWritesStoppingBeforeClosingEveryEventSubscriber(t *testing.T) {
	for _, entry := range []string{"/api/launcher/shutdown", "/api/system/shutdown", "signal context"} {
		t.Run(entry, func(t *testing.T) {
			ctx, stopTest := context.WithTimeout(t.Context(), 10*time.Second)
			defer stopTest()
			// The outbound adapter request synchronizes with App.Run after it
			// installs its cancellation callback, without a readiness sleep.
			started := make(chan struct{}, 1)
			peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				select {
				case started <- struct{}{}:
				default:
				}
				w.WriteHeader(http.StatusServiceUnavailable)
			}))
			defer peer.Close()
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			port := listener.Addr().(*net.TCPAddr).Port
			if err := listener.Close(); err != nil {
				t.Fatal(err)
			}
			application, _, _ := newTestAppWithConfigMutation(t, func(input map[string]any) {
				input["server"].(map[string]any)["port"] = port
				instance := testutil.ConfigDocumentAdapterInstance(t, input, "onebot11")
				instance["enabled"] = true
				forward := instance["onebot11"].(map[string]any)["forward_ws"].(map[string]any)
				forward["enabled"] = true
				forward["url"] = websocketURL(peer.URL)
			}, deterministicAuthOptions()...)
			token := issueLoginToken(t, application)
			server := newManagementTestServer(t, application.Handler())
			defer server.Close()
			connections := make([]*websocket.Conn, 3)
			for i := range connections {
				conn := dialEventsWebSocket(t, server.URL, token)
				defer func() { _ = conn.CloseNow() }()
				readProtocolReplayFrame(t, conn)
				connections[i] = conn
			}
			runCtx, cancelRun := context.WithCancel(ctx)
			done := make(chan struct{})
			var runErr error
			go func() { runErr = application.Run(runCtx); close(done) }()
			defer func() {
				cancelRun()
				select {
				case <-done:
					if runErr != nil {
						t.Errorf("App.Run: %v", runErr)
					}
				case <-ctx.Done():
					t.Error("App.Run did not finish shutdown")
				}
			}()
			select {
			case <-started:
			case <-done:
				t.Fatalf("App.Run exited before shutdown: %v", runErr)
			case <-ctx.Done():
				t.Fatal("App.Run did not start its adapter")
			}
			if entry == "signal context" {
				// main's signal.NotifyContext cancels this same parent context.
				cancelRun()
			} else {
				request, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+entry, nil)
				if err != nil {
					t.Fatal(err)
				}
				request.Header.Set("Authorization", "Bearer "+token)
				request.Header.Set(management.LauncherControlTokenHeader, testutil.TestLauncherControlToken)
				response, err := server.Client().Do(request)
				if err != nil {
					t.Fatal(err)
				}
				_ = response.Body.Close()
				if response.StatusCode != http.StatusAccepted {
					t.Fatalf("shutdown status = %d, want 202", response.StatusCode)
				}
			}
			for i, conn := range connections {
				stopping := false
				for {
					_, payload, err := conn.Read(ctx)
					if err != nil {
						if ctx.Err() != nil {
							t.Fatalf("subscriber %d did not close: %v", i, err)
						}
						if !stopping {
							t.Fatalf("subscriber %d closed before receiving stopping: %v", i, err)
						}
						break
					}
					var frame struct {
						Channel string `json:"channel"`
						Type    string `json:"type"`
						Data    struct {
							ServiceStatus string `json:"service_status"`
						} `json:"data"`
					}
					if err := json.Unmarshal(payload, &frame); err != nil {
						t.Fatal(err)
					}
					if frame.Channel == "events" && frame.Type == "events.received" && frame.Data.ServiceStatus == "stopping" {
						stopping = true
					}
				}
			}
		})
	}
}
