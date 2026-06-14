package outbound

import (
	"strings"

	adaptersegments "github.com/RayleaBot/RayleaBot/server/internal/adapter/segments"
)

// OutboundSegmentsToPlainText generates a human-readable preview from
// outbound message segments using the same semantic labels as inbound logs.
func OutboundSegmentsToPlainText(segments []OutboundMessageSegment) string {
	normalized := make([]adaptersegments.MessageSegment, 0, len(segments))
	for _, seg := range segments {
		normalized = append(normalized, adaptersegments.MessageSegment{
			Type: strings.TrimSpace(seg.Type),
			Data: seg.Data,
		})
	}
	return adaptersegments.ToPlainText(normalized)
}
