package adapter

import (
	"encoding/json"

	adaptersegments "github.com/RayleaBot/RayleaBot/server/internal/adapter/segments"
)

// MessageSegment represents a structured message segment from the OneBot11
// protocol, normalized into a protocol-agnostic form.
type MessageSegment = adaptersegments.MessageSegment

func parseCQString(raw string) []MessageSegment {
	return adaptersegments.ParseCQString(raw)
}

func parseMessageArray(raw json.RawMessage) ([]MessageSegment, error) {
	return adaptersegments.ParseMessageArray(raw)
}

func segmentsToPlainText(segments []MessageSegment) string {
	return adaptersegments.ToPlainText(segments)
}
