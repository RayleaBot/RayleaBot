package qqofficial

import (
	"context"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

func newReloadTestClient(t *testing.T, qq config.QQOfficialConfig) *Client {
	t.Helper()
	return New("qq-official", qq, config.AdapterConfig{
		ConnectTimeoutSeconds:   1,
		ReconnectInitialSeconds: 1,
		ReconnectMultiplier:     2,
		ReconnectMaxSeconds:     2,
	}, discardLogger())
}

func TestReloadReportsWhetherTheConnectionHasToChange(t *testing.T) {
	t.Parallel()

	settings := config.QQOfficialConfig{
		AppID:     "100000001",
		AppSecret: "first-secret",
		Intents:   []string{"group_and_c2c"},
	}
	client := newReloadTestClient(t, settings)

	// Every field of the settings block is fixed at connect time, so an
	// unchanged block means there is nothing to reconnect for.
	if client.Reload(settings) {
		t.Fatal("Reload reported a change for identical settings")
	}

	for _, testCase := range []struct {
		name string
		next config.QQOfficialConfig
	}{
		{name: "app id", next: config.QQOfficialConfig{AppID: "100000002", AppSecret: "first-secret", Intents: []string{"group_and_c2c"}}},
		{name: "app secret", next: config.QQOfficialConfig{AppID: "100000001", AppSecret: "second-secret", Intents: []string{"group_and_c2c"}}},
		{name: "intents", next: config.QQOfficialConfig{AppID: "100000001", AppSecret: "first-secret", Intents: []string{"group_and_c2c", "guilds"}}},
		{name: "sandbox", next: config.QQOfficialConfig{AppID: "100000001", AppSecret: "first-secret", Intents: []string{"group_and_c2c"}, Sandbox: true}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			client := newReloadTestClient(t, settings)
			if !client.Reload(testCase.next) {
				t.Fatalf("Reload reported no change after %s changed", testCase.name)
			}
		})
	}
}

func TestReloadReplacesWhatTheNextConnectionUses(t *testing.T) {
	t.Parallel()

	client := newReloadTestClient(t, config.QQOfficialConfig{
		AppID:     "100000001",
		AppSecret: "first-secret",
		Intents:   []string{"group_and_c2c"},
	})
	beforeAppID, beforeBase, beforeIntents, beforeTokens := client.currentSettings()

	client.Reload(config.QQOfficialConfig{
		AppID:     "100000002",
		AppSecret: "second-secret",
		Intents:   []string{"group_and_c2c", "guilds"},
		Sandbox:   true,
	})
	afterAppID, afterBase, afterIntents, afterTokens := client.currentSettings()

	if afterAppID == beforeAppID || afterIntents == beforeIntents {
		t.Fatalf("settings after reload = %q/%d, want the new app id and intent mask", afterAppID, afterIntents)
	}
	// The sandbox switch selects a different host, and the credential pair is
	// exchanged for the token, so both have to be rebuilt rather than reused.
	if afterBase == beforeBase {
		t.Fatalf("api base = %q, want the sandbox host", afterBase)
	}
	if afterTokens == beforeTokens {
		t.Fatal("the token source survived a credential change")
	}
}

func TestReloadInvalidatesTheResumableSession(t *testing.T) {
	t.Parallel()

	client := newReloadTestClient(t, config.QQOfficialConfig{
		AppID: "100000001", AppSecret: "first-secret", Intents: []string{"group_and_c2c"},
	})
	client.session.startSession("session-1", "bot-1", "洛箐箐", "")
	if _, _, resumable := client.session.snapshot(); !resumable {
		t.Fatal("fixture session is not resumable")
	}

	client.Reload(config.QQOfficialConfig{
		AppID: "100000001", AppSecret: "first-secret", Intents: []string{"group_and_c2c", "guilds"},
	})

	// A resumed session carries the intents of the connection that opened it,
	// so resuming would silently keep subscribing to the old set.
	if _, _, resumable := client.session.snapshot(); resumable {
		t.Fatal("the session stayed resumable across an intent change")
	}
}

func TestReloadEndsTheConnectionItReplaces(t *testing.T) {
	t.Parallel()

	client := newReloadTestClient(t, config.QQOfficialConfig{
		AppID: "100000001", AppSecret: "first-secret", Intents: []string{"group_and_c2c"},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	connCtx, connCancel := context.WithCancel(ctx)
	client.setConnectionCancel(connCancel)

	client.Reload(config.QQOfficialConfig{
		AppID: "100000001", AppSecret: "first-secret", Intents: []string{"guilds"},
	})

	select {
	case <-connCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("the live connection was left running after a reload")
	}
	// The loop distinguishes a reload from a drop so it redials at once instead
	// of serving a backoff the operator did not cause.
	if !client.takeReloading() {
		t.Fatal("the ended connection was not marked as reloaded")
	}
	if client.takeReloading() {
		t.Fatal("the reload flag was not cleared by reading it")
	}
}
