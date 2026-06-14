package action

import "encoding/json"

func ParseTerminalAction(kind string, raw json.RawMessage) (*Action, error) {
	switch kind {
	case "message.send":
		return parseMessageSendAction(raw)
	case "message.reply":
		return parseMessageReplyAction(raw)
	default:
		if isLocalActionKind(kind) || isOneBotFamilyAction(kind) || isProviderExtensionAction(kind) {
			return nil, errorf(codePluginProtocolViolation, "plugin local action request_id must differ from the current event request_id", nil)
		}
		return nil, errorf(codePluginProtocolViolation, "plugin returned unsupported action kind", nil)
	}
}

func ParseLocalAction(kind string, raw json.RawMessage) (*Action, error) {
	switch kind {
	case "logger.write":
		return parseLoggerWriteAction(raw)
	case "storage.kv":
		return parseStorageKVAction(raw)
	case "config.read":
		return parseConfigReadAction(raw)
	case "plugin.list":
		return parsePluginListAction(raw)
	case "secret.read":
		return parseSecretReadAction(raw)
	case "config.write":
		return parseConfigWriteAction(raw)
	case "governance.blacklist.read":
		return parseGovernanceBlacklistReadAction(raw)
	case "governance.blacklist.write":
		return parseGovernanceBlacklistWriteAction(raw)
	case "governance.whitelist.read":
		return parseGovernanceWhitelistReadAction(raw)
	case "governance.whitelist.write":
		return parseGovernanceWhitelistWriteAction(raw)
	case "governance.command_policy.read":
		return parseGovernanceCommandPolicyReadAction(raw)
	case "storage.file":
		return parseStorageFileAction(raw)
	case "http.request":
		return parseHTTPRequestAction(raw)
	case "scheduler.create":
		return parseSchedulerCreateAction(raw)
	case "event.expose_webhook":
		return parseEventExposeWebhookAction(raw)
	case "render.image":
		return parseRenderImageAction(raw)
	case "message.send", "message.reply":
		return nil, errorf(codePluginProtocolViolation, "terminal message actions must use the current event request_id", nil)
	default:
		switch {
		case isOneBotFamilyAction(kind), isProviderExtensionAction(kind):
			return parseOneBotFamilyAction(kind, raw)
		default:
			return nil, errorf(codePluginProtocolViolation, "plugin returned unsupported action kind", nil)
		}
	}
}

func isLocalActionKind(kind string) bool {
	switch kind {
	case "logger.write",
		"storage.kv",
		"config.read",
		"plugin.list",
		"secret.read",
		"config.write",
		"governance.blacklist.read",
		"governance.blacklist.write",
		"governance.whitelist.read",
		"governance.whitelist.write",
		"governance.command_policy.read",
		"storage.file",
		"http.request",
		"scheduler.create",
		"event.expose_webhook",
		"render.image":
		return true
	default:
		return false
	}
}
