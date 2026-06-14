package shell

import (
	"time"

	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/adapter/intake"
	"github.com/coder/websocket"
)

type FrameCategory = adapterintake.FrameCategory

const (
	FrameCategoryLifecycleReady = adapterintake.FrameCategoryLifecycleReady
	FrameCategoryHeartbeat      = adapterintake.FrameCategoryHeartbeat
	FrameCategoryEvent          = adapterintake.FrameCategoryEvent
	FrameCategoryAPIResponse    = adapterintake.FrameCategoryAPIResponse
	FrameCategoryUnknown        = adapterintake.FrameCategoryUnknown
	FrameCategoryInvalid        = adapterintake.FrameCategoryInvalid
)

type FrameSummary = adapterintake.FrameSummary

const (
	EventKindMessageText = adapterintake.EventKindMessageText
	EventKindMessage     = adapterintake.EventKindMessage
	EventKindMessageSent = adapterintake.EventKindMessageSent
	EventKindNotice      = adapterintake.EventKindNotice
	EventKindRequest     = adapterintake.EventKindRequest
	EventKindMeta        = adapterintake.EventKindMeta
)

type NormalizedEvent = adapterintake.NormalizedEvent
type MessageSegment = adapterintake.MessageSegment
type oneBotFrame = adapterintake.OneBotFrame
type classifiedFrame = adapterintake.ClassifiedFrame

func classifyFrame(messageType websocket.MessageType, payload []byte, observedAt time.Time) classifiedFrame {
	return adapterintake.ClassifyFrame(messageType, payload, observedAt)
}

func normalizeSupportedEvent(frame oneBotFrame, observedAt time.Time) (NormalizedEvent, bool) {
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
