package app

import (
	"sort"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
)

// botIdentityProvider is one adapter's view of who it is logged in as.
type botIdentityProvider func() chatevent.BotIdentity

// botIdentitySource collects confirmed identities without selecting a primary bot.
type botIdentitySource struct {
	providers []botIdentityProvider
}

func (s botIdentitySource) BotIdentities() []chatevent.BotIdentity {
	identities := make([]chatevent.BotIdentity, 0, len(s.providers))
	for _, provider := range s.providers {
		identity := provider()
		if strings.TrimSpace(identity.ID) != "" {
			identities = append(identities, identity)
		}
	}
	sort.Slice(identities, func(i, j int) bool { return identities[i].SourceAdapter < identities[j].SourceAdapter })
	return identities
}
