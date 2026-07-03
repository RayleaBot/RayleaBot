package onebot11

import (
	"fmt"
	"time"

	"github.com/coder/websocket"
)

func classifyFrame(messageType websocket.MessageType, payload []byte, observedAt time.Time) ClassifiedFrame {
	return ClassifyFrame(messageType, payload, observedAt)
}

func normalizeSupportedEvent(frame OneBotFrame, observedAt time.Time) (NormalizedEvent, bool) {
	return NormalizeSupportedEvent(frame, observedAt)
}

func applyFrameSummary(snapshot *Snapshot, frame ClassifiedFrame) {
	if snapshot == nil {
		return
	}
	summary := frame.Summary

	snapshot.TotalReceivedFrames++
	snapshot.LastFrameCategory = summary.Category
	snapshot.LastFrameType = summary.Type
	if frame.Frame.SelfID > 0 {
		snapshot.BotID = fmt.Sprintf("%d", frame.Frame.SelfID)
	}

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

func isLifecycleDisable(frame OneBotFrame) bool {
	return frame.PostType == "meta_event" && frame.MetaEventType == "lifecycle" && frame.SubType == "disable"
}
