package app

import (
	"slices"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

func TestBotIdentitiesKeepAdapterNamespaces(t *testing.T) {
	t.Parallel()
	cfg := config.Config{Adapters: []config.AdapterInstance{
		{ID: "qq", Type: "qqofficial", Enabled: true},
		{ID: "onebot", Type: "onebot11", Enabled: true},
		{ID: "idle", Type: "onebot11", Enabled: true},
	}}
	providers := map[string]botIdentityProvider{
		"qq": {protocol: "qqofficial", identity: func() chatevent.BotIdentity {
			return chatevent.BotIdentity{SourceAdapter: "qq", SourceProtocol: "qqofficial", ID: "same-id"}
		}},
		"onebot": {protocol: "onebot11", identity: func() chatevent.BotIdentity {
			return chatevent.BotIdentity{SourceAdapter: "onebot", SourceProtocol: "onebot11", ID: "same-id"}
		}},
		"idle": {protocol: "onebot11", identity: func() chatevent.BotIdentity { return chatevent.BotIdentity{} }},
	}
	source := botIdentitySource{snapshot: func() (config.Config, map[string]botIdentityProvider) { return cfg, providers }}
	got := source.BotIdentities()
	if len(got) != 2 || got[0].SourceAdapter != "onebot" || got[1].SourceAdapter != "qq" {
		t.Fatalf("identities=%#v", got)
	}
	got[0].ID = "mutated"
	if source.BotIdentities()[0].ID != "same-id" {
		t.Fatal("identity snapshot was shared")
	}
	for _, tc := range []struct {
		name     string
		adapters []config.AdapterInstance
		want     []string
	}{
		{name: "reordered", adapters: []config.AdapterInstance{cfg.Adapters[1], cfg.Adapters[0]}, want: []string{"onebot", "qq"}},
		{name: "disabled", adapters: []config.AdapterInstance{{ID: "onebot", Type: "onebot11", Enabled: false}, cfg.Adapters[0]}, want: []string{"qq"}},
		{name: "changed protocol", adapters: []config.AdapterInstance{{ID: "onebot", Type: "qqofficial", Enabled: true}, cfg.Adapters[0]}, want: []string{"qq"}},
		{name: "removed", adapters: []config.AdapterInstance{cfg.Adapters[0]}, want: []string{"qq"}},
		{name: "unregistered", adapters: []config.AdapterInstance{{ID: "new", Type: "onebot11", Enabled: true}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg = config.Config{Adapters: tc.adapters}
			var ids []string
			for _, identity := range source.BotIdentities() {
				ids = append(ids, identity.SourceAdapter)
			}
			if !slices.Equal(ids, tc.want) {
				t.Fatalf("identities = %v, want %v", ids, tc.want)
			}
		})
	}
}
