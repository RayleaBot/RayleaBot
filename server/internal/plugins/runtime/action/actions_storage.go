package action

import (
	"encoding/json"
	"strings"

	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
)

func parseStorageKVAction(raw json.RawMessage) (*Action, error) {
	var frame runtimeprotocol.ProtocolActionStorageKVFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		return nil, errorf(codePluginProtocolViolation, "plugin returned malformed storage.kv data", err)
	}

	switch strings.TrimSpace(frame.Operation) {
	case "get":
		if frame.Key == nil {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required storage.kv fields", nil)
		}
		key := strings.TrimSpace(*frame.Key)
		if key == "" {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required storage.kv fields", nil)
		}
		return &Action{Kind: "storage.kv", StorageOperation: "get", StorageKey: key}, nil
	case "set":
		if frame.Key == nil || frame.Value == nil {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required storage.kv fields", nil)
		}
		key := strings.TrimSpace(*frame.Key)
		if key == "" {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required storage.kv fields", nil)
		}
		var value any
		if err := json.Unmarshal(*frame.Value, &value); err != nil {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame has invalid storage.kv value", err)
		}
		return &Action{Kind: "storage.kv", StorageOperation: "set", StorageKey: key, StorageValue: value}, nil
	case "delete":
		if frame.Key == nil {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required storage.kv fields", nil)
		}
		key := strings.TrimSpace(*frame.Key)
		if key == "" {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required storage.kv fields", nil)
		}
		return &Action{Kind: "storage.kv", StorageOperation: "delete", StorageKey: key}, nil
	case "list":
		if frame.Prefix == nil {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required storage.kv fields", nil)
		}
		prefix := *frame.Prefix
		return &Action{Kind: "storage.kv", StorageOperation: "list", StoragePrefix: prefix}, nil
	default:
		return nil, errorf(codePluginProtocolViolation, "plugin action frame uses unsupported storage.kv operation", nil)
	}
}

func parseStorageFileAction(raw json.RawMessage) (*Action, error) {
	var frame runtimeprotocol.ProtocolActionStorageFileFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		return nil, errorf(codePluginProtocolViolation, "plugin returned malformed storage.file data", err)
	}

	if strings.TrimSpace(frame.Root) != "plugin_data" {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame uses unsupported storage.file root", nil)
	}

	switch strings.TrimSpace(frame.Operation) {
	case "read":
		if frame.Path == nil || *frame.Path == "" {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required storage.file fields", nil)
		}
		return &Action{Kind: "storage.file", StorageOperation: "read", StorageRoot: "plugin_data", StoragePath: *frame.Path}, nil
	case "write":
		if frame.Path == nil {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required storage.file fields", nil)
		}
		content, err := decodeExclusiveTextOrBase64(frame.ContentText, frame.ContentBase64, true)
		if err != nil {
			return nil, err
		}
		return &Action{
			Kind:             "storage.file",
			StorageOperation: "write",
			StorageRoot:      "plugin_data",
			StoragePath:      *frame.Path,
			StorageContent:   content,
		}, nil
	case "delete":
		if frame.Path == nil || *frame.Path == "" {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required storage.file fields", nil)
		}
		return &Action{Kind: "storage.file", StorageOperation: "delete", StorageRoot: "plugin_data", StoragePath: *frame.Path}, nil
	case "list":
		if frame.Prefix == nil {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required storage.file fields", nil)
		}
		return &Action{Kind: "storage.file", StorageOperation: "list", StorageRoot: "plugin_data", StoragePrefix: *frame.Prefix}, nil
	default:
		return nil, errorf(codePluginProtocolViolation, "plugin action frame uses unsupported storage.file operation", nil)
	}
}
