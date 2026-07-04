package runtime

import (
	"encoding/base64"
	"encoding/json"
	"strings"
)

func parseEmptyObjectAction(raw json.RawMessage, actionKind string) error {
	payload := make(map[string]any)
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &payload); err != nil {
			return errorf(codePluginProtocolViolation, "plugin returned malformed "+actionKind+" data", err)
		}
	}
	if len(payload) > 0 {
		return errorf(codePluginProtocolViolation, "plugin action frame has invalid "+actionKind+" data", nil)
	}
	return nil
}

func decodeExclusiveTextOrBase64(text *string, encoded *string, required bool) ([]byte, error) {
	if text != nil && encoded != nil {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame mixes text and base64 content fields", nil)
	}
	if text != nil {
		return []byte(*text), nil
	}
	if encoded != nil {
		content, err := base64.StdEncoding.DecodeString(*encoded)
		if err != nil {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame has invalid base64 content", err)
		}
		return content, nil
	}
	if !required {
		return nil, nil
	}
	return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required text or base64 content fields", nil)
}

func cloneHTTPActionHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return map[string]string{}
	}
	cloned := make(map[string]string, len(headers))
	for key, value := range headers {
		cloned[key] = value
	}
	return cloned
}

func validateActionTarget(rawType, rawID, actionKind string) (string, string, error) {
	targetType := strings.TrimSpace(rawType)
	targetID := strings.TrimSpace(rawID)
	if targetID == "" {
		return "", "", errorf(codePluginProtocolViolation, "plugin action frame is missing required "+actionKind+" fields", nil)
	}
	switch targetType {
	case "group", "private":
		return targetType, targetID, nil
	default:
		return "", "", errorf(codePluginProtocolViolation, "plugin action frame uses unsupported target_type", nil)
	}
}
