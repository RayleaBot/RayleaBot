package shell

import (
	"fmt"

	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/adapter/intake"
)

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
