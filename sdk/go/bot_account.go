package rayleabot

import "github.com/RayleaBot/RayleaBot/sdk/go/internal/pluginwire"

// BotAccount identifies an account independently of its current connection.
// QQ official IDs are opaque strings, not QQ numbers or AppIDs.
type BotAccount pluginwire.BotAccountRef

// Address creates a persistent delivery address for a target owned by this bot.
func (bot BotAccount) Address(targetType, targetID string) Address {
	return Address{SourceProtocol: bot.SourceProtocol, BotID: bot.BotID, TargetType: targetType, TargetID: targetID}
}

// ManagementPageID returns the host-supplied page ID for a management action.
// A legacy action can omit the page; nested business payload is never consulted.
func (event Event) ManagementPageID() string {
	if event.EventType != "management.action" || event.SourceProtocol != "management" {
		return ""
	}
	page, _ := event.Payload["page_id"].(string)
	return page
}

// ManagementBot returns the account selected for a management action. It does
// not infer an account from the business payload or the runtime's Bots list.
// The account may be offline or have no connection.
func (event Event) ManagementBot() (BotAccount, bool) {
	if event.ManagementPageID() == "" {
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
