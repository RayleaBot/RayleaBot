package app

import (
	"slices"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

// botIdentityProvider is one adapter's view of who it is logged in as.
type botIdentityProvider struct {
	protocol string
	identity func() chatevent.BotIdentity
}

// botIdentitySource collects confirmed identities without selecting a primary bot.
type botIdentitySource struct {
	providers     map[string]botIdentityProvider
	currentConfig func() config.Config
}

func (s botIdentitySource) BotIdentities() []chatevent.BotIdentity {
	identities := make([]chatevent.BotIdentity, 0, len(s.providers))
	for _, instance := range s.currentConfig().Adapters {
		provider, ok := s.providers[instance.ID]
		if !ok || !instance.Enabled || instance.Type != provider.protocol {
			continue
		}
		identity := provider.identity()
		if strings.TrimSpace(identity.ID) != "" {
			identities = append(identities, identity)
		}
	}
	slices.SortFunc(identities, func(a, b chatevent.BotIdentity) int { return strings.Compare(a.SourceAdapter, b.SourceAdapter) })
	return identities
}
