package dispatch

import (
	"github.com/RayleaBot/RayleaBot/server/internal/onebot11"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
)

func toOutboundSegments(segments []pluginruntime.ActionSegment) []onebot11.OutboundMessageSegment {
	if len(segments) == 0 {
		return nil
	}

	items := make([]onebot11.OutboundMessageSegment, 0, len(segments))
	for _, segment := range segments {
		data := make(map[string]any, len(segment.Data))
		for key, value := range segment.Data {
			data[key] = value
		}
		items = append(items, onebot11.OutboundMessageSegment{
			Type: segment.Type,
			Data: data,
		})
	}
	return items
}
