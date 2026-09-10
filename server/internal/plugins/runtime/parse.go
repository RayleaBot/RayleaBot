package runtime

import (
	"encoding/json"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/adapters/onebot11"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func ParseTerminalAction(kind string, raw json.RawMessage) (*chatevent.MessageCommand, error) {
	switch kind {
	case "message.send":
		action, err := parseMessageSendAction(raw)
		if err != nil {
			return nil, err
		}
		command := action.MessageCommand()
		return &command, nil
	default:
		if isLocalActionKind(kind) || isOneBotFamilyAction(kind) || isProviderExtensionAction(kind) {
			return nil, errorf(codePluginProtocolViolation, "plugin local action request_id must differ from the current event request_id", nil)
		}
		return nil, errorf(codePluginProtocolViolation, "plugin returned unsupported action kind", nil)
	}
}

func ParseLocalAction(kind string, raw json.RawMessage) (*plugins.Action, error) {
	switch kind {
	case "logger.write":
		return parseLoggerWriteAction(raw)
	case "storage.kv":
		return parseStorageKVAction(raw)
	case "plugin.list":
		return parsePluginListAction(raw)
	case "secret.read":
		return parseSecretReadAction(raw)
	case "thirdparty.account.read":
		return parseThirdPartyAccountReadAction(raw)
	case "thirdparty.account.validate":
		return parseThirdPartyAccountValidateAction(raw)
	case "thirdparty.resolve":
		return parseThirdPartyResolveAction(raw)
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
	case "render.image":
		return parseRenderImageAction(raw)
	case "message.send":
		return parseMessageSendAction(raw)
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
		"plugin.list",
		"secret.read",
		"thirdparty.account.read",
		"thirdparty.account.validate",
		"thirdparty.resolve",
		"config.write",
		"governance.blacklist.read",
		"governance.blacklist.write",
		"governance.whitelist.read",
		"governance.whitelist.write",
		"governance.command_policy.read",
		"storage.file",
		"http.request",
		"scheduler.create",
		"render.image",
		"message.send":
		return true
	default:
		return false
	}
}

func isOneBotFamilyAction(kind string) bool {
	return onebot11.IsGenericAction(kind)
}

func isProviderExtensionAction(kind string) bool {
	return onebot11.IsProviderExtensionAction(kind)
}

func parseOneBotFamilyAction(actionKind string, raw json.RawMessage) (*plugins.Action, error) {
	payload := map[string]any{}
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &payload); err != nil {
			return nil, errorf(codePluginProtocolViolation, "plugin returned malformed onebot action data", err)
		}
		if payload == nil {
			payload = map[string]any{}
		}
	}
	sourceAdapter := ""
	if value, present := payload["source_adapter"]; present {
		var ok bool
		sourceAdapter, ok = value.(string)
		if !ok || !adapterInstanceIDPattern.MatchString(sourceAdapter) {
			return nil, errorf(codePluginProtocolViolation, "OneBot action source_adapter must be a configured instance ID", nil)
		}
		delete(payload, "source_adapter")
	}
	sourceProtocol := ""
	if value, present := payload["source_protocol"]; present {
		var ok bool
		sourceProtocol, ok = value.(string)
		if !ok || sourceProtocol != "onebot11" {
			return nil, errorf(codePluginProtocolViolation, "OneBot action source_protocol must be onebot11", nil)
		}
		delete(payload, "source_protocol")
	}
	return &plugins.Action{
		Kind:           actionKind,
		RawData:        payload,
		SourceAdapter:  sourceAdapter,
		SourceProtocol: sourceProtocol,
	}, nil
}
