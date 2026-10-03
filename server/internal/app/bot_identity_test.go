package app

import (
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
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
