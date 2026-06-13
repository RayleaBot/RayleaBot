package runtime

import (
	"encoding/json"
	"strings"
)

func parseLoggerWriteAction(raw json.RawMessage) (*Action, error) {
	var frame protocolActionLoggerWriteFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		return nil, errorf(codePluginProtocolViolation, "plugin returned malformed logger.write data", err)
	}

	level := strings.TrimSpace(frame.Level)
	switch level {
	case "debug", "info", "warn", "error":
	default:
		return nil, errorf(codePluginProtocolViolation, "plugin action frame has invalid logger.write level", nil)
	}

	message := strings.TrimSpace(frame.Message)
	if message == "" {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required logger.write fields", nil)
	}

	return &Action{
		Kind:       "logger.write",
		LogLevel:   level,
		LogMessage: message,
		LogFields:  cloneActionSegmentData(frame.Fields),
	}, nil
}

func parseConfigReadAction(raw json.RawMessage) (*Action, error) {
	var frame protocolActionConfigReadFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		return nil, errorf(codePluginProtocolViolation, "plugin returned malformed config.read data", err)
	}

	keys := make([]string, 0, len(frame.Keys))
	seen := make(map[string]struct{}, len(frame.Keys))
	for _, key := range frame.Keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required config.read fields", nil)
	}
	return &Action{
		Kind:       "config.read",
		ConfigKeys: keys,
	}, nil
}

func parsePluginListAction(raw json.RawMessage) (*Action, error) {
	var frame protocolActionPluginListFrame
	payload := map[string]json.RawMessage{}
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &frame); err != nil {
			return nil, errorf(codePluginProtocolViolation, "plugin returned malformed plugin.list data", err)
		}
		if err := json.Unmarshal(raw, &payload); err != nil {
			return nil, errorf(codePluginProtocolViolation, "plugin returned malformed plugin.list data", err)
		}
	}
	for key := range payload {
		if key != "visibility" {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame has invalid plugin.list data", nil)
		}
	}

	visibility := strings.TrimSpace(frame.Visibility)
	switch visibility {
	case "", "catalog":
		visibility = "catalog"
	case "caller":
	default:
		return nil, errorf(codePluginProtocolViolation, "plugin action frame has invalid plugin.list visibility", nil)
	}
	return &Action{Kind: "plugin.list", PluginListVisibility: visibility}, nil
}

func parseSecretReadAction(raw json.RawMessage) (*Action, error) {
	var frame protocolActionSecretReadFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		return nil, errorf(codePluginProtocolViolation, "plugin returned malformed secret.read data", err)
	}

	key := strings.TrimSpace(frame.Key)
	if key == "" {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required secret.read fields", nil)
	}
	return &Action{Kind: "secret.read", SecretKey: key}, nil
}

func parseConfigWriteAction(raw json.RawMessage) (*Action, error) {
	var frame protocolActionConfigWriteFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		return nil, errorf(codePluginProtocolViolation, "plugin returned malformed config.write data", err)
	}
	if len(frame.Values) == 0 {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required config.write fields", nil)
	}

	values := make(map[string]any, len(frame.Values))
	for key, rawValue := range frame.Values {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		var value any
		if err := json.Unmarshal(rawValue, &value); err != nil {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame has invalid config.write value", err)
		}
		values[key] = value
	}
	if len(values) == 0 {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required config.write fields", nil)
	}
	return &Action{
		Kind:         "config.write",
		ConfigValues: values,
	}, nil
}
