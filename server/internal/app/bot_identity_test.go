package app

import (
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

func TestBotIdentityFallsBackThroughConfiguredAdapters(t *testing.T) {
	t.Parallel()

	oneBot, qq := "", ""
	source := botIdentitySource{providers: []botIdentityProvider{
		func() string { return oneBot },
		func() string { return qq },
	}}

	// Nothing connected: the runtime has no identity rather than a made-up one.
	if got := source.CurrentBotID(); got != "" {
		t.Fatalf("CurrentBotID() = %q, want empty before any adapter connects", got)
	}

	// A deployment running only a QQ adapter still has an identity, which is
	// what the plugin runtime needs to reconcile anything bot-scoped.
	qq = "qq-bot"
	if got := source.CurrentBotID(); got != "qq-bot" {
		t.Fatalf("CurrentBotID() = %q, want the connected QQ identity", got)
	}

	// With both connected the first configured adapter wins, so the identity
	// does not flip between adapters as connections come and go.
	oneBot = "onebot-bot"
	if got := source.CurrentBotID(); got != "onebot-bot" {
		t.Fatalf("CurrentBotID() = %q, want the first configured adapter's identity", got)
	}
}

func TestEventWiringBuildsIdentityProvidersInConfigurationOrder(t *testing.T) {
	t.Parallel()

	state := buildEvents(eventDeps{
		Logger: discardLogger(),
		Config: config.Config{Adapters: []config.AdapterInstance{
			{ID: config.DefaultQQOfficialAdapterID, Type: config.AdapterTypeQQOfficial, Enabled: true,
				QQOfficial: &config.QQOfficialConfig{AppID: "100000001", AppSecret: "fixture-secret"}},
			{ID: config.DefaultOneBot11AdapterID, Type: config.AdapterTypeOneBot11, Enabled: true, OneBot11: &config.OneBotConfig{}},
			{ID: "off-bot", Type: config.AdapterTypeOneBot11, Enabled: false, OneBot11: &config.OneBotConfig{}},
		}},
	})

	// Only running adapters can report an identity, so a disabled instance
	// contributes nothing to fall back through.
	if len(state.BotIdentity.providers) != 2 {
		t.Fatalf("built %d identity providers, want one per running adapter", len(state.BotIdentity.providers))
	}
	if got := state.BotIdentity.CurrentBotID(); got != "" {
		t.Fatalf("CurrentBotID() = %q, want empty while nothing has connected", got)
	}
}
