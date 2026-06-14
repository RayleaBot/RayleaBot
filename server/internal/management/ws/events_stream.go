package ws

import (
	"context"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	managementevents "github.com/RayleaBot/RayleaBot/server/internal/management/events"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func (h *EventsHandler) streamEventsWebSocket(conn *websocket.Conn) {
	eventsCtx := conn.CloseRead(context.Background())
	bridgeFrames, unsubscribeBridge := h.bridge.SubscribeObservability(1)
	defer unsubscribeBridge()
	var pluginFrames <-chan plugins.Snapshot
	unsubscribePlugins := func() {}
	if h.plugins != nil {
		pluginFrames, unsubscribePlugins = h.plugins.Subscribe(8)
	}
	defer unsubscribePlugins()
	protocolFrames, unsubscribeProtocol := h.protocol.SubscribeProtocolEvents(2)
	defer unsubscribeProtocol()
	statusFrames, unsubscribeStatus := h.serviceStatus.Subscribe(4)
	defer unsubscribeStatus()
	var bilibiliFrames <-chan managementevents.Frame
	unsubscribeBilibili := func() {}
	if h.bilibili != nil {
		bilibiliFrames, unsubscribeBilibili = h.bilibili.Subscribe(4)
	}
	defer unsubscribeBilibili()
	var governanceFrames <-chan managementevents.Frame
	unsubscribeGovernance := func() {}
	if h.governance != nil {
		governanceFrames, unsubscribeGovernance = h.governance.Subscribe(4)
	}
	defer unsubscribeGovernance()

	for _, frame := range []managementevents.Frame{
		h.serviceStatus.CurrentEvent(),
		h.protocol.ProtocolSnapshotEvent(),
		h.bilibili.CurrentEvent(),
	} {
		if err := wsjson.Write(eventsCtx, conn, frame); err != nil {
			return
		}
	}

	for {
		select {
		case <-eventsCtx.Done():
			return
		case frame, ok := <-bridgeFrames:
			if !ok {
				return
			}
			if err := wsjson.Write(eventsCtx, conn, frame); err != nil {
				return
			}
		case snapshot, ok := <-pluginFrames:
			if !ok {
				return
			}
			if err := wsjson.Write(eventsCtx, conn, pluginStateEventFrame(snapshot, pluginSnapshotsForConflicts(h.plugins))); err != nil {
				return
			}
		case frame, ok := <-protocolFrames:
			if !ok {
				return
			}
			if err := wsjson.Write(eventsCtx, conn, frame); err != nil {
				return
			}
		case frame, ok := <-statusFrames:
			if !ok {
				return
			}
			if err := wsjson.Write(eventsCtx, conn, frame); err != nil {
				return
			}
		case frame, ok := <-bilibiliFrames:
			if !ok {
				return
			}
			if err := wsjson.Write(eventsCtx, conn, frame); err != nil {
				return
			}
		case frame, ok := <-governanceFrames:
			if !ok {
				return
			}
			if err := wsjson.Write(eventsCtx, conn, frame); err != nil {
				return
			}
		}
	}
}
