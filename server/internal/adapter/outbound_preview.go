package adapter

import "strings"

// OutboundSegmentsToPlainText generates a human-readable preview from
// outbound message segments using the same semantic labels as inbound logs.
func OutboundSegmentsToPlainText(segments []OutboundMessageSegment) string {
	normalized := make([]MessageSegment, 0, len(segments))
	for _, seg := range segments {
		normalized = append(normalized, MessageSegment{
			Type: strings.TrimSpace(seg.Type),
			Data: seg.Data,
		})
	}
	return segmentsToPlainText(normalized)
}
