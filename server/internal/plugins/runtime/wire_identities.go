package runtime

import (
	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/pluginwire"
)

func wireBotIdentities(bots []chatevent.BotIdentity) []pluginwire.BotIdentity {
	result := make([]pluginwire.BotIdentity, 0, len(bots))
	for _, bot := range bots {
		result = append(result, pluginwire.BotIdentity(bot))
	}
	return result
}
