package protocolapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	adaptershell "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/shell"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	managementevents "github.com/RayleaBot/RayleaBot/server/internal/management/events"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

type protocolTestConfigSource struct {
	config config.Config
}

func (s protocolTestConfigSource) CurrentConfig() config.Config {
	return s.config
}

func TestProtocolIssuesFromSnapshotUsesStableOperatorSummaries(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		apply       func(*adaptershell.Snapshot)
		wantCode    string
		wantSummary string
	}{
		{
			name: "forward websocket auth failure hides low level error",
			apply: func(snapshot *adaptershell.Snapshot) {
				snapshot.ForwardWS = adaptershell.TransportSnapshot{
					State:            adaptershell.TransportStateAuthFailed,
					LastErrorCode:    "adapter.transport_forward_ws_connection_failed",
					LastErrorMessage: "websocket: bad handshake",
				}
			},
			wantCode:    "adapter.transport_forward_ws_connection_failed",
			wantSummary: "OneBot 主动连接鉴权失败，请检查访问令牌。",
		},
		{
			name: "reverse websocket auth failure stays readable",
			apply: func(snapshot *adaptershell.Snapshot) {
				snapshot.ReverseWS = adaptershell.TransportSnapshot{
					State:            adaptershell.TransportStateAuthFailed,
					LastErrorCode:    "adapter.transport_reverse_ws_auth_failed",
					LastErrorMessage: "reverse websocket authentication failed",
				}
			},
			wantCode:    "adapter.transport_reverse_ws_auth_failed",
			wantSummary: "OneBot 回连鉴权失败，请检查访问令牌。",
		},
		{
			name: "http api invalid response hides parse detail",
			apply: func(snapshot *adaptershell.Snapshot) {
				snapshot.HTTPAPI = adaptershell.TransportSnapshot{
					State:            adaptershell.TransportStateReconnecting,
					LastErrorCode:    "adapter.transport_http_api_invalid_response",
					LastErrorMessage: "invalid character 'b' looking for beginning of value",
				}
			},
			wantCode:    "adapter.transport_http_api_invalid_response",
			wantSummary: "OneBot HTTP API 返回无效响应。",
		},
		{
			name: "webhook invalid payload hides raw frame detail",
			apply: func(snapshot *adaptershell.Snapshot) {
				snapshot.Webhook = adaptershell.TransportSnapshot{
					State:            adaptershell.TransportStateListening,
					LastErrorCode:    "adapter.transport_webhook_invalid_payload",
					LastErrorMessage: "invalid frame: unsupported payload",
				}
			},
			wantCode:    "adapter.transport_webhook_invalid_payload",
			wantSummary: "OneBot Webhook 上报格式无效。",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			snapshot := adaptershell.Snapshot{}
			tc.apply(&snapshot)

			issues := protocolIssuesFromSnapshot(snapshot)
			if len(issues) != 1 {
				t.Fatalf("expected one issue, got %#v", issues)
			}
			if issues[0].Code != tc.wantCode {
				t.Fatalf("unexpected issue code: got %q want %q", issues[0].Code, tc.wantCode)
			}
			if issues[0].Severity != "warning" {
				t.Fatalf("unexpected issue severity: got %q want %q", issues[0].Severity, "warning")
			}
			if issues[0].Summary != tc.wantSummary {
				t.Fatalf("unexpected issue summary: got %q want %q", issues[0].Summary, tc.wantSummary)
			}
		})
	}
}

func TestProtocolIssuesFromSnapshotSkipsClearedErrors(t *testing.T) {
	t.Parallel()

	snapshot := adaptershell.Snapshot{
		ForwardWS: adaptershell.TransportSnapshot{
			State:            adaptershell.TransportStateConnected,
			LastErrorCode:    "",
			LastErrorMessage: "dial tcp 127.0.0.1:5700: connectex: connection refused",
		},
		Webhook: adaptershell.TransportSnapshot{
			State:            adaptershell.TransportStateListening,
			LastErrorCode:    "",
			LastErrorMessage: "invalid frame: unsupported payload",
		},
	}

	issues := protocolIssuesFromSnapshot(snapshot)
	if len(issues) != 0 {
		t.Fatalf("expected cleared issues to be omitted, got %#v", issues)
	}
}

func TestProtocolSnapshotEventMatchesCurrentProjection(t *testing.T) {
	t.Parallel()

	requests := make(chan map[string]any, 2)
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
				t.Errorf("read runtime info request: %v", err)
				return
			}
			requests <- request
			data := map[string]any{}
			switch request["action"] {
			case "get_version_info":
				data = map[string]any{
					"app_name":         "LLOneBot",
					"protocol_version": 11,
					"app_version":      "6.5.0",
				}
			case "get_login_info":
				data = map[string]any{
					"user_id":  10001,
					"nickname": "LuckyBot",
				}
			default:
				t.Errorf("unexpected action: %v", request["action"])
			}
			if err := wsjson.Write(context.Background(), conn, map[string]any{
				"status":  "ok",
				"retcode": 0,
				"data":    data,
				"echo":    request["echo"],
			}); err != nil {
				t.Errorf("write runtime info response: %v", err)
				return
			}
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
	waitForRuntimeInfo(t, shell, adaptershell.TransportForwardWS, "luckylillia", time.Second)
	if len(requests) != 2 {
		t.Fatalf("runtime info requests = %d, want 2", len(requests))
	}
	service := NewProtocolService(protocolTestConfigSource{}, shell)

	snapshot := service.currentOneBot11ProtocolSnapshot()
	if snapshot.Provider != "luckylillia" {
		t.Fatalf("unexpected provider: got %q want %q", snapshot.Provider, "luckylillia")
	}
	var forward protocolTransportStatusResponse
	for _, item := range snapshot.TransportStatus {
		if item.Transport == "forward_ws" {
			forward = item
			break
		}
	}
	if forward.Provider != "luckylillia" || forward.AppName != "LLOneBot" || forward.ProtocolVersion != "11" || forward.UserID != "10001" || forward.Nickname != "LuckyBot" {
		t.Fatalf("unexpected forward runtime info: %#v", forward)
	}
	if len(snapshot.TransportStatus) != 4 {
		t.Fatalf("unexpected transport count: got %d want 4", len(snapshot.TransportStatus))
	}
	for _, item := range snapshot.TransportStatus {
		if item.Transport == "sse" {
			t.Fatalf("unexpected transport in protocol snapshot: %#v", item)
		}
	}

	frame := service.ProtocolSnapshotEvent()
	data, ok := frame.Data.(managementevents.ProtocolSnapshotPayload)
	if !ok {
		t.Fatalf("expected protocol snapshot event payload, got %T", frame.Data)
	}
	if data.Protocol != "onebot11" {
		t.Fatalf("unexpected protocol: got %q want %q", data.Protocol, "onebot11")
	}
	projected := data.ProtocolSnapshot
	if !reflect.DeepEqual(projected, snapshot) {
		t.Fatalf("unexpected event projection: got %#v want %#v", projected, snapshot)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
	defer stopCancel()
	if err := shell.Stop(stopCtx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestProtocolTargetsReturnPartialResultsWhenFriendListTimesOut(t *testing.T) {
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
			t.Errorf("write ready frame: %v", err)
			return
		}

		for {
			var request map[string]any
			if err := wsjson.Read(r.Context(), conn, &request); err != nil {
				return
			}
			switch request["action"] {
			case "get_group_list":
				if err := wsjson.Write(context.Background(), conn, map[string]any{
					"status":  "ok",
					"retcode": 0,
					"data": []any{
						map[string]any{"group_id": 5050, "group_name": "测试群"},
					},
					"echo": request["echo"],
				}); err != nil {
					t.Errorf("write group list response: %v", err)
					return
				}
			case "get_friend_list":
				// Leave this request unanswered. The management endpoint must
				// return the group list instead of waiting forever for friends.
			default:
				t.Errorf("unexpected action: %v", request["action"])
			}
		}
	}))
	defer server.Close()

	shell := adaptershell.NewForTest(config.OneBotConfig{
		ForwardWS: config.OneBotTransportConfig{
			Enabled: true,
			URL:     "ws" + server.URL[len("http"):],
		},
	}, defaultAdapterTestConfig(), slog.New(slog.NewTextHandler(io.Discard, nil)), true)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	shell.Start(ctx)
	waitForAdapterState(t, shell, adaptershell.StateConnected, time.Second)

	service := NewProtocolService(protocolTestConfigSource{}, shell)
	service.oneBot11TargetReadTimeout = 60 * time.Millisecond

	started := time.Now()
	response := service.currentOneBot11ProtocolTargets(context.Background())
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("target lookup took too long: %s", elapsed)
	}
	if response.Available {
		t.Fatalf("response should be partially available after friend timeout: %#v", response)
	}
	if len(response.Groups) != 1 || response.Groups[0].TargetID != "5050" || response.Groups[0].TargetName != "测试群" {
		t.Fatalf("unexpected groups: %#v", response.Groups)
	}
	if len(response.PrivateUsers) != 0 {
		t.Fatalf("unexpected private users: %#v", response.PrivateUsers)
	}
	if len(response.Issues) != 1 || response.Issues[0].Scope != "private_users" {
		t.Fatalf("unexpected issues: %#v", response.Issues)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
	defer stopCancel()
	if err := shell.Stop(stopCtx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestProtocolCompatibilityProjectionKeepsUnsupportedGapsVisible(t *testing.T) {
	t.Parallel()

	service := NewProtocolService(protocolTestConfigSource{}, nil)

	response, err := service.currentOneBot11ProtocolCompatibility()
	if err != nil {
		t.Fatalf("unexpected compatibility projection error: %v", err)
	}
	if response.Protocol != "onebot11" {
		t.Fatalf("unexpected protocol: got %q want %q", response.Protocol, "onebot11")
	}
	if len(response.Categories) != 4 {
		t.Fatalf("unexpected category count: got %d want 4", len(response.Categories))
	}

	var foundProviderExtension bool
	for _, category := range response.Categories {
		if category.Key != "provider_extensions" {
			continue
		}
		for _, item := range category.Items {
			if item.Key != "provider.napcat.group.sign.set" {
				continue
			}
			foundProviderExtension = true
			if item.Support.NapCat != "supported" {
				t.Fatalf("unexpected NapCat support: %#v", item.Support)
			}
			if item.Support.Standard != "unsupported" || item.Support.LuckyLillia != "unsupported" {
				t.Fatalf("unsupported provider gaps should remain explicit: %#v", item.Support)
			}
		}
	}
	if !foundProviderExtension {
		t.Fatal("expected napcat provider extension item in compatibility matrix")
	}
}
