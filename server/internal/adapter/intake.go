package adapter

import (
	"encoding/json"
	"time"

	"github.com/coder/websocket"
)

type FrameCategory string

const (
	FrameCategoryLifecycleReady FrameCategory = "lifecycle_ready"
	FrameCategoryHeartbeat      FrameCategory = "heartbeat"
	FrameCategoryEvent          FrameCategory = "event"
	FrameCategoryUnknown        FrameCategory = "unknown"
	FrameCategoryInvalid        FrameCategory = "invalid"
)

type FrameSummary struct {
	Category          FrameCategory
	Type              string
	ObservedAt        time.Time
	HeartbeatInterval time.Duration
}

type oneBotFrame struct {
	PostType      string `json:"post_type"`
	MetaEventType string `json:"meta_event_type"`
	SubType       string `json:"sub_type"`
	Interval      int    `json:"interval"`
}

type classifiedFrame struct {
	Summary        FrameSummary
	Frame          oneBotFrame
	InvalidSummary string
}

func classifyFrame(messageType websocket.MessageType, payload []byte, observedAt time.Time) classifiedFrame {
	if messageType != websocket.MessageText && messageType != websocket.MessageBinary {
		return classifiedFrame{
			Summary: FrameSummary{
				Category:   FrameCategoryInvalid,
				Type:       string(FrameCategoryInvalid),
				ObservedAt: observedAt,
			},
			InvalidSummary: "unexpected websocket message type",
		}
	}

	var frame oneBotFrame
	if err := json.Unmarshal(payload, &frame); err != nil {
		return classifiedFrame{
			Summary: FrameSummary{
				Category:   FrameCategoryInvalid,
				Type:       string(FrameCategoryInvalid),
				ObservedAt: observedAt,
			},
			InvalidSummary: summarizeError(err),
		}
	}

	summary := FrameSummary{
		ObservedAt: observedAt,
	}

	switch {
	case frame.PostType == "meta_event" && frame.MetaEventType == "lifecycle" && frame.SubType == "enable":
		summary.Category = FrameCategoryLifecycleReady
		summary.Type = "meta.lifecycle.enable"
	case frame.PostType == "meta_event" && frame.MetaEventType == "heartbeat":
		summary.Category = FrameCategoryHeartbeat
		summary.Type = "meta.heartbeat"
		if frame.Interval > 0 {
			summary.HeartbeatInterval = time.Duration(frame.Interval) * time.Millisecond
		}
	case frame.PostType != "":
		summary.Category = FrameCategoryEvent
		summary.Type = frame.PostType
	default:
		summary.Category = FrameCategoryUnknown
		summary.Type = string(FrameCategoryUnknown)
	}

	return classifiedFrame{
		Summary: summary,
		Frame:   frame,
	}
}

func applyFrameSummary(snapshot *Snapshot, summary FrameSummary) {
	if snapshot == nil {
		return
	}

	snapshot.TotalReceivedFrames++
	snapshot.LastFrameCategory = summary.Category
	snapshot.LastFrameType = summary.Type

	if summary.Category == FrameCategoryInvalid {
		snapshot.InvalidReceivedFrames++
	} else {
		snapshot.LastFrameAt = cloneTime(&summary.ObservedAt)
	}

	if summary.Category == FrameCategoryHeartbeat {
		snapshot.HeartbeatSeen = true
		snapshot.LastHeartbeatAt = cloneTime(&summary.ObservedAt)
		if summary.HeartbeatInterval > 0 {
			snapshot.HeartbeatInterval = summary.HeartbeatInterval
		}
	}
}

func isReadySummary(summary FrameSummary) bool {
	return summary.Category == FrameCategoryLifecycleReady || summary.Category == FrameCategoryHeartbeat
}

func isLifecycleDisable(frame oneBotFrame) bool {
	return frame.PostType == "meta_event" && frame.MetaEventType == "lifecycle" && frame.SubType == "disable"
}
