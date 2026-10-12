package rayleabot

import "github.com/RayleaBot/RayleaBot/sdk/go/internal/pluginwire"

// BotAccount identifies an account independently of its current connection.
// QQ official IDs are opaque strings, not QQ numbers or AppIDs.
type BotAccount pluginwire.BotAccountRef

// ManagementBot returns the account selected for a management action. It does
// not infer an account from the business payload or the runtime's Bots list.
// The account may be offline or have no connection.
func (event Event) ManagementBot() (BotAccount, bool) {
	if event.EventType != "management.action" || event.SourceProtocol != "management" {
		return BotAccount{}, false
	}
	// The host only sends an account together with the page it validated.
	if page, _ := event.Payload["page_id"].(string); page == "" {
		return BotAccount{}, false
	}
	fields, ok := event.Payload["bot"].(map[string]any)
	if !ok {
		return BotAccount{}, false
	}
	protocol, _ := fields["source_protocol"].(string)
	id, _ := fields["bot_id"].(string)
	if (protocol != "onebot11" && protocol != "qqofficial") || id == "" {
		return BotAccount{}, false
	}
	return BotAccount{SourceProtocol: protocol, BotID: id}, true
}
