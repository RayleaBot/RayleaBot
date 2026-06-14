package action

import (
	"encoding/json"

	protocolonebot "github.com/RayleaBot/RayleaBot/server/internal/protocol/onebot"
)

func isOneBotFamilyAction(kind string) bool {
	return protocolonebot.IsGenericAction(kind)
}

func isProviderExtensionAction(kind string) bool {
	return protocolonebot.IsProviderExtensionAction(kind)
}

func parseOneBotFamilyAction(actionKind string, raw json.RawMessage) (*Action, error) {
	payload := map[string]any{}
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &payload); err != nil {
			return nil, errorf(codePluginProtocolViolation, "plugin returned malformed onebot action data", err)
		}
		if payload == nil {
			payload = map[string]any{}
		}
	}
	return &Action{
		Kind:    actionKind,
		RawData: payload,
	}, nil
}
