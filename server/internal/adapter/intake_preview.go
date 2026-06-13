package adapter

import (
	"bytes"
	"encoding/json"

	"github.com/RayleaBot/RayleaBot/server/internal/textsafe"
)

func previewFramePayload(payload []byte) any {
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 {
		return ""
	}

	var decoded any
	if err := json.Unmarshal(trimmed, &decoded); err == nil {
		return textsafe.SanitizeAny(decoded)
	}

	text := textsafe.SanitizeString(string(trimmed))
	if utf8SafeText := textsafe.TruncateRunes(text, 256, "...(truncated)"); utf8SafeText != text {
		return utf8SafeText
	}
	return text
}
