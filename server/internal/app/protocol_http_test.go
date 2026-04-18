package app

import (
	"reflect"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

func TestProtocolIssuesFromSnapshotUsesStableOperatorSummaries(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		apply       func(*adapter.Snapshot)
		wantCode    string
		wantSummary string
	}{
		{
			name: "forward websocket auth failure hides low level error",
			apply: func(snapshot *adapter.Snapshot) {
				snapshot.ForwardWS = adapter.TransportSnapshot{
					State:            adapter.TransportStateAuthFailed,
					LastErrorCode:    "adapter.transport_forward_ws_connection_failed",
					LastErrorMessage: "websocket: bad handshake",
				}
			},
			wantCode:    "adapter.transport_forward_ws_connection_failed",
			wantSummary: "OneBot 主动连接鉴权失败，请检查访问令牌。",
		},
		{
			name: "reverse websocket auth failure stays readable",
			apply: func(snapshot *adapter.Snapshot) {
				snapshot.ReverseWS = adapter.TransportSnapshot{
					State:            adapter.TransportStateAuthFailed,
					LastErrorCode:    "adapter.transport_reverse_ws_auth_failed",
					LastErrorMessage: "reverse websocket authentication failed",
				}
			},
			wantCode:    "adapter.transport_reverse_ws_auth_failed",
			wantSummary: "OneBot 回连鉴权失败，请检查访问令牌。",
		},
		{
			name: "http api invalid response hides parse detail",
			apply: func(snapshot *adapter.Snapshot) {
				snapshot.HTTPAPI = adapter.TransportSnapshot{
					State:            adapter.TransportStateReconnecting,
					LastErrorCode:    "adapter.transport_http_api_invalid_response",
					LastErrorMessage: "invalid character 'b' looking for beginning of value",
				}
			},
			wantCode:    "adapter.transport_http_api_invalid_response",
			wantSummary: "OneBot HTTP API 返回无效响应。",
		},
		{
			name: "webhook invalid payload hides raw frame detail",
			apply: func(snapshot *adapter.Snapshot) {
				snapshot.Webhook = adapter.TransportSnapshot{
					State:            adapter.TransportStateListening,
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

			snapshot := adapter.Snapshot{}
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

	snapshot := adapter.Snapshot{
		ForwardWS: adapter.TransportSnapshot{
			State:            adapter.TransportStateConnected,
			LastErrorCode:    "",
			LastErrorMessage: "dial tcp 127.0.0.1:5700: connectex: connection refused",
		},
		Webhook: adapter.TransportSnapshot{
			State:            adapter.TransportStateListening,
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

	service := newProtocolService(&appRuntimeState{
		Config: config.Config{
			OneBot: config.OneBotConfig{
				Provider: "luckylillia",
			},
		},
	}, nil)

	snapshot := service.currentOneBot11ProtocolSnapshot()
	if snapshot.Provider != "luckylillia" {
		t.Fatalf("unexpected provider: got %q want %q", snapshot.Provider, "luckylillia")
	}
	if len(snapshot.TransportStatus) != 4 {
		t.Fatalf("unexpected transport count: got %d want 4", len(snapshot.TransportStatus))
	}
	for _, item := range snapshot.TransportStatus {
		if item.Transport == "sse" {
			t.Fatalf("unexpected transport in protocol snapshot: %#v", item)
		}
	}

	frame := service.protocolSnapshotEvent()
	data, ok := frame.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected event data map, got %T", frame.Data)
	}
	projected, ok := data["protocol_snapshot"].(oneBot11ProtocolSnapshotResponse)
	if !ok {
		t.Fatalf("expected protocol snapshot payload, got %T", data["protocol_snapshot"])
	}
	if !reflect.DeepEqual(projected, snapshot) {
		t.Fatalf("unexpected event projection: got %#v want %#v", projected, snapshot)
	}
}
