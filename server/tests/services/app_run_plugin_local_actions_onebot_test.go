package services

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"

	adaptershell "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/shell"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/action"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
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

	shell := adaptershell.NewForTest(config.OneBotConfig{
		ForwardWS: config.OneBotTransportConfig{
			Enabled: true,
			URL:     "ws" + server.URL[len("http"):],
		},
	}, defaultAdapterTestConfig(), slog.New(slog.NewJSONHandler(io.Discard, nil)), true)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shell.Start(ctx)
	waitForAdapterState(t, shell, adaptershell.StateConnected, time.Second)

	application := newTestAppState(config.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	application.setTestLocalActions(&stubCapabilityView{capabilities: map[string][]stubCapability{
		"weather": {{PluginID: "weather", Capability: "message.history.get"}},
	}}, nil, nil, nil, nil, nil, nil, shell, nil, nil)

	result, err := application.executeOneBotLocalAction(context.Background(), "weather", "req_hist", runtimeaction.Action{
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

	application := newTestAppState(config.Config{}, nil)
	application.setTestLocalActions(&stubCapabilityView{capabilities: map[string][]stubCapability{
		"weather": {{PluginID: "weather", Capability: "provider.napcat.message_emoji.like.set"}},
	}}, nil, nil, nil, nil, nil, nil, &adaptershell.Shell{}, nil, nil)

	_, err := application.executeOneBotLocalAction(context.Background(), "weather", "req_provider", runtimeaction.Action{
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

	shell := adaptershell.New(config.OneBotConfig{
		ForwardWS: config.OneBotTransportConfig{
			Enabled: true,
			URL:     "ws" + server.URL[len("http"):],
		},
	}, defaultAdapterTestConfig(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	shell.Start(ctx)
	waitForAdapterState(t, shell, adaptershell.StateConnected, time.Second)
	waitForRuntimeInfo(t, shell, adaptershell.TransportForwardWS, "napcat", time.Second)

	application := newTestAppState(config.Config{}, nil)
	application.setTestLocalActions(&stubCapabilityView{capabilities: map[string][]stubCapability{
		"weather": {{PluginID: "weather", Capability: "provider.napcat.message_emoji.like.set"}},
	}}, nil, nil, nil, nil, nil, nil, shell, nil, nil)

	_, err := application.executeOneBotLocalAction(context.Background(), "weather", "req_provider", runtimeaction.Action{
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
	application.setTestLocalActions(nil, nil, nil, nil, nil, nil, nil, &adaptershell.Shell{}, nil, nil)

	_, err := application.executeOneBotLocalAction(context.Background(), "weather", "req_provider", runtimeaction.Action{
		Kind: "message.history.get",
		RawData: map[string]any{
			"conversation_type": "group",
			"conversation_id":   "456",
		},
	})
	assertRuntimeErrorCode(t, err, "plugin.capability_violation")
}

func TestExecuteOneBotLocalActionConnectionLossKeepsPluginRunning(t *testing.T) {
	t.Parallel()

	application := newTestAppState(config.Config{}, nil)
	application.pluginStack.Plugins = plugincatalog.New([]plugins.Snapshot{{
		PluginID:          "weather",
		Name:              "Weather",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
	}})
	application.setTestLocalActions(&stubCapabilityView{capabilities: map[string][]stubCapability{
		"weather": {{PluginID: "weather", Capability: "message.history.get"}},
	}}, nil, nil, nil, nil, nil, nil, &adaptershell.Shell{}, nil, nil)

	_, err := application.executeOneBotLocalAction(context.Background(), "weather", "req_hist_disconnected", runtimeaction.Action{
		Kind: "message.history.get",
		RawData: map[string]any{
			"conversation_type": "group",
			"conversation_id":   "456",
		},
	})
	assertRuntimeErrorCode(t, err, "adapter.connection_lost")

	snapshot, ok := application.pluginStack.Plugins.Get("weather")
	if !ok {
		t.Fatal("plugin missing from catalog")
	}
	if snapshot.RuntimeState != string(runtimemanager.StateRunning) {
		t.Fatalf("runtime_state = %q, want running", snapshot.RuntimeState)
	}
}

func waitForAdapterState(t *testing.T, shell *adaptershell.Shell, want adaptershell.State, timeout time.Duration) {
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

func waitForRuntimeInfo(t *testing.T, shell *adaptershell.Shell, transport adaptershell.TransportKey, wantProvider string, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		snapshot := shell.Snapshot()
		var info adaptershell.TransportRuntimeInfo
		switch transport {
		case adaptershell.TransportForwardWS:
			info = snapshot.ForwardWS.RuntimeInfo
		case adaptershell.TransportReverseWS:
			info = snapshot.ReverseWS.RuntimeInfo
		case adaptershell.TransportHTTPAPI:
			info = snapshot.HTTPAPI.RuntimeInfo
		case adaptershell.TransportWebhook:
			info = snapshot.Webhook.RuntimeInfo
		}
		if info.Provider == wantProvider {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for %s runtime provider %s, got %#v", transport, wantProvider, shell.Snapshot())
}
