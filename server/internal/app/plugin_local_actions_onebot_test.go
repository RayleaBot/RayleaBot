package app

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

func TestExecuteOneBotLocalActionMessageHistoryGet(t *testing.T) {
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
			t.Errorf("write ready frame: %v", err)
			return
		}

		var request map[string]any
		if err := wsjson.Read(context.Background(), conn, &request); err != nil {
			t.Errorf("read api request: %v", err)
			return
		}
		requests <- request

		if err := wsjson.Write(context.Background(), conn, map[string]any{
			"status":  "ok",
			"retcode": 0,
			"data": []any{
				map[string]any{
					"message_id":  12345,
					"user_id":     20001,
					"raw_message": "hello history",
				},
			},
			"echo": request["echo"],
		}); err != nil {
			t.Errorf("write api response: %v", err)
			return
		}

		<-r.Context().Done()
	}))
	defer server.Close()

	shell := adapter.NewForTest(config.OneBotConfig{
		WSURL: "ws" + server.URL[len("http"):],
	}, slog.New(slog.NewJSONHandler(io.Discard, nil)), true)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shell.Start(ctx)
	waitForAdapterState(t, shell, adapter.StateConnected, time.Second)

	application := newTestAppState(config.Config{
		Auth: config.AuthConfig{
			AutoGrantCapabilities: []string{"message.history.get"},
		},
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	application.setTestLocalActions(nil, nil, nil, nil, nil, nil, nil, shell, nil, nil)

	result, err := application.executeOneBotLocalAction(context.Background(), "weather", "req_hist", runtime.Action{
		Kind: "message.history.get",
		RawData: map[string]any{
			"conversation_type": "group",
			"conversation_id":   "456",
			"limit":             float64(20),
		},
	})
	if err != nil {
		t.Fatalf("executeOneBotLocalAction failed: %v", err)
	}

	var request map[string]any
	select {
	case request = <-requests:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for history request")
	}

	if request["action"] != "get_group_msg_history" {
		t.Fatalf("unexpected action: %#v", request["action"])
	}
	params, ok := request["params"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected params: %#v", request["params"])
	}
	if params["group_id"] != "456" {
		t.Fatalf("unexpected group_id: %#v", params["group_id"])
	}
	if params["limit"] != float64(20) {
		t.Fatalf("unexpected limit: %#v", params["limit"])
	}

	messages, ok := result["messages"].([]any)
	if !ok || len(messages) != 1 {
		t.Fatalf("unexpected messages result: %#v", result)
	}
	first, ok := messages[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected first message: %#v", messages[0])
	}
	if first["message_id"] != "12345" {
		t.Fatalf("unexpected message_id normalization: %#v", first["message_id"])
	}
}

func TestExecuteOneBotLocalActionProviderMismatch(t *testing.T) {
	t.Parallel()

	application := newTestAppState(config.Config{
		Auth: config.AuthConfig{
			AutoGrantCapabilities: []string{"provider.napcat.message_emoji.like.set"},
		},
	}, nil)
	application.setTestLocalActions(nil, nil, nil, nil, nil, nil, nil, &adapter.Shell{}, nil, nil)

	_, err := application.executeOneBotLocalAction(context.Background(), "weather", "req_provider", runtime.Action{
		Kind: "provider.napcat.message_emoji.like.set",
		RawData: map[string]any{
			"message_id": "8899",
			"emoji_id":   "128512",
			"enabled":    true,
		},
	})
	assertRuntimeErrorCode(t, err, "adapter.provider_extension_not_supported")
}

func TestExecuteOneBotLocalActionProviderExtensionUsesDetectedProvider(t *testing.T) {
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
			t.Errorf("write ready frame: %v", err)
			return
		}

		for i := 0; i < 2; i++ {
			var request map[string]any
			if err := wsjson.Read(context.Background(), conn, &request); err != nil {
				t.Errorf("read runtime request: %v", err)
				return
			}
			data := map[string]any{}
			switch request["action"] {
			case "get_version_info":
				data = map[string]any{
					"app_name":         "NapCat.Onebot",
					"protocol_version": "v11",
					"app_version":      "1.0.0",
				}
			case "get_login_info":
				data = map[string]any{
					"user_id":  20001,
					"nickname": "NapCatBot",
				}
			default:
				t.Errorf("unexpected runtime info action: %v", request["action"])
			}
			if err := wsjson.Write(context.Background(), conn, map[string]any{
				"status":  "ok",
				"retcode": 0,
				"data":    data,
				"echo":    request["echo"],
			}); err != nil {
				t.Errorf("write runtime response: %v", err)
				return
			}
		}

		var actionRequest map[string]any
		if err := wsjson.Read(context.Background(), conn, &actionRequest); err != nil {
			t.Errorf("read provider action request: %v", err)
			return
		}
		requests <- actionRequest
		if err := wsjson.Write(context.Background(), conn, map[string]any{
			"status":  "ok",
			"retcode": 0,
			"data":    map[string]any{},
			"echo":    actionRequest["echo"],
		}); err != nil {
			t.Errorf("write provider action response: %v", err)
			return
		}

		<-r.Context().Done()
	}))
	defer server.Close()

	shell := adapter.New(config.OneBotConfig{
		WSURL: "ws" + server.URL[len("http"):],
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	shell.Start(ctx)
	waitForAdapterState(t, shell, adapter.StateConnected, time.Second)
	waitForRuntimeInfo(t, shell, adapter.TransportForwardWS, "napcat", time.Second)

	application := newTestAppState(config.Config{
		Auth: config.AuthConfig{
			AutoGrantCapabilities: []string{"provider.napcat.message_emoji.like.set"},
		},
	}, nil)
	application.setTestLocalActions(nil, nil, nil, nil, nil, nil, nil, shell, nil, nil)

	_, err := application.executeOneBotLocalAction(context.Background(), "weather", "req_provider", runtime.Action{
		Kind: "provider.napcat.message_emoji.like.set",
		RawData: map[string]any{
			"message_id": "8899",
			"emoji_id":   "128512",
			"enabled":    true,
		},
	})
	if err != nil {
		t.Fatalf("executeOneBotLocalAction failed: %v", err)
	}

	var request map[string]any
	select {
	case request = <-requests:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for provider action request")
	}
	if request["action"] != "set_msg_emoji_like" {
		t.Fatalf("unexpected provider action: %#v", request["action"])
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
	defer stopCancel()
	if err := shell.Stop(stopCtx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestExecuteOneBotLocalActionRejectsMissingCapability(t *testing.T) {
	t.Parallel()

	application := newTestAppState(config.Config{}, nil)
	application.setTestLocalActions(nil, nil, nil, nil, nil, nil, nil, &adapter.Shell{}, nil, nil)

	_, err := application.executeOneBotLocalAction(context.Background(), "weather", "req_provider", runtime.Action{
		Kind: "message.history.get",
		RawData: map[string]any{
			"conversation_type": "group",
			"conversation_id":   "456",
		},
	})
	assertRuntimeErrorCode(t, err, "permission.scope_violation")
}

func TestExecuteOneBotLocalActionConnectionLossKeepsPluginRunning(t *testing.T) {
	t.Parallel()

	application := newTestAppState(config.Config{
		Auth: config.AuthConfig{
			AutoGrantCapabilities: []string{"message.history.get"},
		},
	}, nil)
	application.plugins = plugins.NewCatalog([]plugins.Snapshot{{
		PluginID:          "weather",
		Name:              "Weather",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
	}})
	application.setTestLocalActions(nil, nil, nil, nil, nil, nil, nil, &adapter.Shell{}, nil, nil)

	_, err := application.executeOneBotLocalAction(context.Background(), "weather", "req_hist_disconnected", runtime.Action{
		Kind: "message.history.get",
		RawData: map[string]any{
			"conversation_type": "group",
			"conversation_id":   "456",
		},
	})
	assertRuntimeErrorCode(t, err, "adapter.connection_lost")

	snapshot, ok := application.plugins.Get("weather")
	if !ok {
		t.Fatal("plugin missing from catalog")
	}
	if snapshot.RuntimeState != string(runtime.StateRunning) {
		t.Fatalf("runtime_state = %q, want running", snapshot.RuntimeState)
	}
}

func waitForAdapterState(t *testing.T, shell *adapter.Shell, want adapter.State, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if shell.Snapshot().State == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for adapter state %s, got %s", want, shell.Snapshot().State)
}

func waitForRuntimeInfo(t *testing.T, shell *adapter.Shell, transport adapter.TransportKey, wantProvider string, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		snapshot := shell.Snapshot()
		var info adapter.TransportRuntimeInfo
		switch transport {
		case adapter.TransportForwardWS:
			info = snapshot.ForwardWS.RuntimeInfo
		case adapter.TransportReverseWS:
			info = snapshot.ReverseWS.RuntimeInfo
		case adapter.TransportHTTPAPI:
			info = snapshot.HTTPAPI.RuntimeInfo
		case adapter.TransportWebhook:
			info = snapshot.Webhook.RuntimeInfo
		}
		if info.Provider == wantProvider {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for %s runtime provider %s, got %#v", transport, wantProvider, shell.Snapshot())
}
