package shell

import (
	"fmt"
	"time"

	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/intake"
	"github.com/coder/websocket"
)

func classifyFrame(messageType websocket.MessageType, payload []byte, observedAt time.Time) adapterintake.ClassifiedFrame {
	return adapterintake.ClassifyFrame(messageType, payload, observedAt)
}

func normalizeSupportedEvent(frame adapterintake.OneBotFrame, observedAt time.Time) (adapterintake.NormalizedEvent, bool) {
	return adapterintake.NormalizeSupportedEvent(frame, observedAt)
}

func previewFramePayload(payload []byte) any {
	return adapterintake.PreviewFramePayload(payload)
}

func frameEcho(value any) (string, bool) {
	return adapterintake.FrameEcho(value)
}

func frameStatusText(value any) string {
	return adapterintake.FrameStatusText(value)
}

func applyFrameSummary(snapshot *Snapshot, frame adapterintake.ClassifiedFrame) {
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

	if summary.Category == adapterintake.FrameCategoryInvalid {
		snapshot.InvalidReceivedFrames++
	} else {
		snapshot.LastFrameAt = cloneTime(&summary.ObservedAt)
	}

	if summary.Category == adapterintake.FrameCategoryHeartbeat {
		snapshot.HeartbeatSeen = true
		snapshot.LastHeartbeatAt = cloneTime(&summary.ObservedAt)
		if summary.HeartbeatInterval > 0 {
			snapshot.HeartbeatInterval = summary.HeartbeatInterval
		}
	}
}

func isReadySummary(summary adapterintake.FrameSummary) bool {
	return summary.Category == adapterintake.FrameCategoryLifecycleReady || summary.Category == adapterintake.FrameCategoryHeartbeat
}

func isLifecycleDisable(frame adapterintake.OneBotFrame) bool {
	return frame.PostType == "meta_event" && frame.MetaEventType == "lifecycle" && frame.SubType == "disable"
}
