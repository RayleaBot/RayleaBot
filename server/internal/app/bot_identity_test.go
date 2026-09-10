package app

import (
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

func TestBotIdentitiesKeepAdapterNamespaces(t *testing.T) {
	t.Parallel()
	source := botIdentitySource{providers: []botIdentityProvider{
		func() chatevent.BotIdentity {
			return chatevent.BotIdentity{SourceAdapter: "qq", SourceProtocol: "qqofficial", ID: "same-id"}
		},
		func() chatevent.BotIdentity {
			return chatevent.BotIdentity{SourceAdapter: "onebot", SourceProtocol: "onebot11", ID: "same-id"}
		},
		func() chatevent.BotIdentity { return chatevent.BotIdentity{} },
	}}
	got := source.BotIdentities()
	if len(got) != 2 || got[0].SourceAdapter != "onebot" || got[1].SourceAdapter != "qq" {
		t.Fatalf("identities=%#v", got)
	}
	got[0].ID = "mutated"
	if source.BotIdentities()[0].ID != "same-id" {
		t.Fatal("identity snapshot was shared")
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

	t.Cleanup(state.Close)
	// Disabled providers contribute no identity.
	if len(state.BotIdentity.providers) != 3 {
		t.Fatalf("built %d identity providers, want one per configured adapter", len(state.BotIdentity.providers))
	}
	if got := state.BotIdentity.BotIdentities(); len(got) != 0 {
		t.Fatalf("BotIdentities() = %#v, want empty while nothing has connected", got)
	}
}
