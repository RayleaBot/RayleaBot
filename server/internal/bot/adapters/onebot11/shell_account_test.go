package onebot11

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/coder/websocket"
)

func TestRetiredConnectionCannotConfirmOrDisconnectCurrentAccount(t *testing.T) {
	shell := newShell("x", config.OneBotConfig{}, config.AdapterConfig{}, nil, shellDeps{skipRuntimeInfo: true})
	old, current := &websocket.Conn{}, &websocket.Conn{}
	shell.reverseConn = current
	shell.snapshot.ReverseWS = TransportSnapshot{Enabled: true, State: TransportStateConnected, AccountID: "20002"}
	frame := classifyFrame(websocket.MessageText, []byte(`{"time":1700000000,"self_id":10001,"post_type":"meta_event","meta_event_type":"heartbeat","status":{"online":true,"good":true},"interval":1000}`), time.Now())
	if err := shell.recordAndValidateFrame(TransportReverseWS, frame, old); !errors.Is(err, context.Canceled) {
		t.Fatalf("retired frame = %v", err)
	}
	shell.markReverseSessionFailure(old, errors.New("retired disconnect"))
	if id, _ := shell.ConfirmedAccount(); id != "20002" {
		t.Fatalf("retired callback changed identity to %q", id)
	}
	shell.markReverseSessionFailure(current, errors.New("current disconnect"))
	if id, _ := shell.ConfirmedAccount(); id != "" {
		t.Fatalf("disconnected account is still confirmed: %q", id)
	}
}

func TestConfirmedAccountIgnoresMismatchedNickname(t *testing.T) {
	shell := newShell("x", config.OneBotConfig{}, config.AdapterConfig{}, nil, shellDeps{skipRuntimeInfo: true})
	shell.snapshot.ReverseWS = TransportSnapshot{Enabled: true, State: TransportStateConnected, AccountID: "10001", RuntimeInfo: TransportRuntimeInfo{UserID: "20002", Nickname: "other bot"}}
	shell.snapshot.HTTPAPI = TransportSnapshot{Enabled: true, State: TransportStateConnected, RuntimeInfo: TransportRuntimeInfo{UserID: "10001", Nickname: "matching bot"}}
	if id, name := shell.ConfirmedAccount(); id != "10001" || name != "matching bot" {
		t.Fatalf("confirmed = %q/%q", id, name)
	}
}
