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

	shell := adapter.New(config.OneBotConfig{
		WSURL: "ws" + server.URL[len("http"):],
	}, slog.New(slog.NewJSONHandler(io.Discard, nil)))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shell.Start(ctx)
	waitForAdapterState(t, shell, adapter.StateConnected, time.Second)

	application := newTestAppState(config.Config{
		OneBot: config.OneBotConfig{Provider: "standard"},
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
		OneBot: config.OneBotConfig{Provider: "standard"},
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

func TestExecuteOneBotLocalActionRejectsMissingCapability(t *testing.T) {
	t.Parallel()

	application := newTestAppState(config.Config{
		OneBot: config.OneBotConfig{Provider: "standard"},
	}, nil)
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
