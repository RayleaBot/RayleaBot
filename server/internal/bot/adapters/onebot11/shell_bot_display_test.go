package onebot11

import (
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

func TestSnapshotLoginNicknameFollowsTheTransportSignedInAsTheBot(t *testing.T) {
	t.Parallel()

	snapshot := Snapshot{
		ForwardWS: TransportSnapshot{RuntimeInfo: TransportRuntimeInfo{UserID: "20002", Nickname: "另一个账号"}},
		ReverseWS: TransportSnapshot{RuntimeInfo: TransportRuntimeInfo{UserID: "10001", Nickname: "测试机器人"}},
	}

	if got := snapshot.loginNickname("10001"); got != "测试机器人" {
		t.Fatalf("loginNickname = %q, want the transport signed in as 10001", got)
	}
	// A transport signed in as someone else never names this account.
	if got := snapshot.loginNickname("30003"); got != "" {
		t.Fatalf("loginNickname = %q, want no name for an account no transport is signed in as", got)
	}

	unnamed := Snapshot{
		HTTPAPI: TransportSnapshot{RuntimeInfo: TransportRuntimeInfo{Nickname: "仅昵称"}},
	}
	if got := unnamed.loginNickname("10001"); got != "仅昵称" {
		t.Fatalf("loginNickname = %q, want the nickname of a transport that reported no account id", got)
	}
}

func TestShellResolveBotDisplayUsesItsOwnLogin(t *testing.T) {
	t.Parallel()

	shell := newShell("bot-one", config.OneBotConfig{}, config.AdapterConfig{}, nil, shellDeps{skipRuntimeInfo: true})
	shell.updateTransportRuntimeInfo(TransportForwardWS, TransportRuntimeInfo{UserID: "10001", Nickname: "测试机器人"})

	// Before any frame names the bot, the login info is the only identity.
	if id, nickname := shell.ResolveBotDisplay("bot-one"); id != "10001" || nickname != "测试机器人" {
		t.Fatalf("ResolveBotDisplay = %q/%q, want the login info identity", id, nickname)
	}

	shell.mu.Lock()
	shell.snapshot.BotID = "10001"
	shell.mu.Unlock()
	if id, nickname := shell.ResolveBotDisplay(""); id != "10001" || nickname != "测试机器人" {
		t.Fatalf("ResolveBotDisplay = %q/%q, want the connected bot with its nickname", id, nickname)
	}
	if id, nickname := shell.ResolveBotDisplay("bot-two"); id != "" || nickname != "" {
		t.Fatalf("ResolveBotDisplay = %q/%q, want no answer about another instance", id, nickname)
	}
}
