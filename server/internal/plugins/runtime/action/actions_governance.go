package action

import (
	"encoding/json"
	"strings"

	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
)

func parseGovernanceBlacklistReadAction(raw json.RawMessage) (*Action, error) {
	if err := parseEmptyObjectAction(raw, "governance.blacklist.read"); err != nil {
		return nil, err
	}
	return &Action{Kind: "governance.blacklist.read"}, nil
}

func parseGovernanceBlacklistWriteAction(raw json.RawMessage) (*Action, error) {
	var frame runtimeprotocol.ProtocolActionGovernanceBlacklistWriteFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		return nil, errorf(codePluginProtocolViolation, "plugin returned malformed governance.blacklist.write data", err)
	}

	operation := strings.TrimSpace(frame.Operation)
	switch operation {
	case "upsert":
		if frame.EntryType == nil || frame.TargetID == nil || frame.Reason == nil {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required governance.blacklist.write fields", nil)
		}
		entryType := strings.TrimSpace(*frame.EntryType)
		targetID := strings.TrimSpace(*frame.TargetID)
		reason := strings.TrimSpace(*frame.Reason)
		if entryType == "" || targetID == "" || reason == "" {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required governance.blacklist.write fields", nil)
		}
		return &Action{
			Kind:                "governance.blacklist.write",
			GovernanceOperation: operation,
			GovernanceEntryType: entryType,
			GovernanceTargetID:  targetID,
			GovernanceReason:    reason,
		}, nil
	case "delete":
		if frame.EntryType == nil || frame.TargetID == nil {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required governance.blacklist.write fields", nil)
		}
		entryType := strings.TrimSpace(*frame.EntryType)
		targetID := strings.TrimSpace(*frame.TargetID)
		if entryType == "" || targetID == "" {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required governance.blacklist.write fields", nil)
		}
		return &Action{
			Kind:                "governance.blacklist.write",
			GovernanceOperation: operation,
			GovernanceEntryType: entryType,
			GovernanceTargetID:  targetID,
		}, nil
	default:
		return nil, errorf(codePluginProtocolViolation, "plugin action frame uses unsupported governance.blacklist.write operation", nil)
	}
}

func parseGovernanceWhitelistReadAction(raw json.RawMessage) (*Action, error) {
	if err := parseEmptyObjectAction(raw, "governance.whitelist.read"); err != nil {
		return nil, err
	}
	return &Action{Kind: "governance.whitelist.read"}, nil
}

func parseGovernanceWhitelistWriteAction(raw json.RawMessage) (*Action, error) {
	var frame runtimeprotocol.ProtocolActionGovernanceWhitelistWriteFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		return nil, errorf(codePluginProtocolViolation, "plugin returned malformed governance.whitelist.write data", err)
	}

	operation := strings.TrimSpace(frame.Operation)
	switch operation {
	case "set_enabled":
		if frame.Enabled == nil {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required governance.whitelist.write fields", nil)
		}
		return &Action{
			Kind:                "governance.whitelist.write",
			GovernanceOperation: operation,
			GovernanceEnabled:   frame.Enabled,
		}, nil
	case "upsert":
		if frame.EntryType == nil || frame.TargetID == nil || frame.Reason == nil {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required governance.whitelist.write fields", nil)
		}
		entryType := strings.TrimSpace(*frame.EntryType)
		targetID := strings.TrimSpace(*frame.TargetID)
		reason := strings.TrimSpace(*frame.Reason)
		if entryType == "" || targetID == "" || reason == "" {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required governance.whitelist.write fields", nil)
		}
		return &Action{
			Kind:                "governance.whitelist.write",
			GovernanceOperation: operation,
			GovernanceEntryType: entryType,
			GovernanceTargetID:  targetID,
			GovernanceReason:    reason,
		}, nil
	case "delete":
		if frame.EntryType == nil || frame.TargetID == nil {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required governance.whitelist.write fields", nil)
		}
		entryType := strings.TrimSpace(*frame.EntryType)
		targetID := strings.TrimSpace(*frame.TargetID)
		if entryType == "" || targetID == "" {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required governance.whitelist.write fields", nil)
		}
		return &Action{
			Kind:                "governance.whitelist.write",
			GovernanceOperation: operation,
			GovernanceEntryType: entryType,
			GovernanceTargetID:  targetID,
		}, nil
	default:
		return nil, errorf(codePluginProtocolViolation, "plugin action frame uses unsupported governance.whitelist.write operation", nil)
	}
}

func parseGovernanceCommandPolicyReadAction(raw json.RawMessage) (*Action, error) {
	if err := parseEmptyObjectAction(raw, "governance.command_policy.read"); err != nil {
		return nil, err
	}
	return &Action{Kind: "governance.command_policy.read"}, nil
}
