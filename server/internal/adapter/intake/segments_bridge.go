package intake

import (
	"encoding/json"

	adaptersegments "github.com/RayleaBot/RayleaBot/server/internal/adapter/segments"
)

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
