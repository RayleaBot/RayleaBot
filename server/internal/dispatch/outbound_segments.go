package dispatch

import (
	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

func toOutboundSegments(segments []runtime.ActionSegment) []adapter.OutboundMessageSegment {
	if len(segments) == 0 {
		return nil
	}

	items := make([]adapter.OutboundMessageSegment, 0, len(segments))
	for _, segment := range segments {
		data := make(map[string]any, len(segment.Data))
		for key, value := range segment.Data {
			data[key] = value
		}
		items = append(items, adapter.OutboundMessageSegment{
			Type: segment.Type,
			Data: data,
		})
	}
	return items
}
