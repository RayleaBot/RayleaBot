package intake

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/coder/websocket"
)

func ClassifyFrame(messageType websocket.MessageType, payload []byte, observedAt time.Time) ClassifiedFrame {
	payloadPreview := PreviewFramePayload(payload)

	if messageType != websocket.MessageText && messageType != websocket.MessageBinary {
		return ClassifiedFrame{
			Summary: FrameSummary{
				Category:   FrameCategoryInvalid,
				Type:       string(FrameCategoryInvalid),
				ObservedAt: observedAt,
			},
			InvalidSummary: "unexpected websocket message type",
			PayloadPreview: payloadPreview,
		}
	}

	var frame OneBotFrame
	if err := json.Unmarshal(payload, &frame); err != nil {
		return ClassifiedFrame{
			Summary: FrameSummary{
				Category:   FrameCategoryInvalid,
				Type:       string(FrameCategoryInvalid),
				ObservedAt: observedAt,
			},
			InvalidSummary: summarizeError(err),
			PayloadPreview: payloadPreview,
		}
	}

	summary := FrameSummary{
		ObservedAt: observedAt,
	}

	switch {
	case frame.PostType == "meta_event" && frame.MetaEventType == "lifecycle" && frame.SubType == "enable":
		summary.Category = FrameCategoryLifecycleReady
		summary.Type = "meta.lifecycle.enable"
	case frame.PostType == "meta_event" && frame.MetaEventType == "lifecycle" && frame.SubType == "connect":
		summary.Category = FrameCategoryLifecycleReady
		summary.Type = "meta.lifecycle.connect"
	case frame.PostType == "meta_event" && frame.MetaEventType == "heartbeat":
		summary.Category = FrameCategoryHeartbeat
		summary.Type = "meta.heartbeat"
		if frame.Interval > 0 {
			summary.HeartbeatInterval = time.Duration(frame.Interval) * time.Millisecond
		}
	case frame.Echo != nil:
		if _, ok := frameEcho(frame.Echo); !ok {
			return ClassifiedFrame{
				Summary: FrameSummary{
					Category:   FrameCategoryUnknown,
					Type:       "api.response.ignored",
					ObservedAt: observedAt,
				},
				InvalidSummary: "api response echo must be a non-empty string",
				Frame:          frame,
				PayloadPreview: payloadPreview,
			}
		}
		summary.Category = FrameCategoryAPIResponse
		summary.Type = "api.response"
	case frame.PostType != "":
		summary.Category = FrameCategoryEvent
		summary.Type = frame.PostType
	default:
		summary.Category = FrameCategoryUnknown
		summary.Type = string(FrameCategoryUnknown)
	}

	return ClassifiedFrame{
		Summary:        summary,
		Frame:          frame,
		PayloadPreview: payloadPreview,
	}
}

func frameEcho(value any) (string, bool) {
	echo, ok := value.(string)
	if !ok {
		return "", false
	}
	echo = strings.TrimSpace(echo)
	if echo == "" {
		return "", false
	}
	return echo, true
}

func FrameEcho(value any) (string, bool) {
	return frameEcho(value)
}

func frameStatusText(value any) string {
	status, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(status)
}

func FrameStatusText(value any) string {
	return frameStatusText(value)
}

func summarizeError(err error) string {
	if err == nil {
		return ""
	}

	return strings.Join(strings.Fields(err.Error()), " ")
}
