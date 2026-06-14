package shell

import (
	"time"

	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/adapter/intake"
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
