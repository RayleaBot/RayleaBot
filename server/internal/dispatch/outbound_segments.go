package dispatch

import (
	adapteroutbound "github.com/RayleaBot/RayleaBot/server/internal/adapter/outbound"
	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/runtime/action"
)

func toOutboundSegments(segments []runtimeaction.ActionSegment) []adapteroutbound.OutboundMessageSegment {
	if len(segments) == 0 {
		return nil
	}

	items := make([]adapteroutbound.OutboundMessageSegment, 0, len(segments))
	for _, segment := range segments {
		data := make(map[string]any, len(segment.Data))
		for key, value := range segment.Data {
			data[key] = value
		}
		items = append(items, adapteroutbound.OutboundMessageSegment{
			Type: segment.Type,
			Data: data,
		})
	}
	return items
}
