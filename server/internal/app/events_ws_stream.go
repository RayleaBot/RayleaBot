package app

import (
	"context"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func (h *eventsWSHandler) streamEventsWebSocket(conn *websocket.Conn) {
	eventsCtx := conn.CloseRead(context.Background())
	bridgeFrames, unsubscribeBridge := h.bridge.SubscribeObservability(1)
	defer unsubscribeBridge()
	var pluginFrames <-chan plugins.Snapshot
	unsubscribePlugins := func() {}
	if h.plugins != nil {
		pluginFrames, unsubscribePlugins = h.plugins.Subscribe(8)
	}
	defer unsubscribePlugins()
	protocolFrames, unsubscribeProtocol := h.protocol.subscribeProtocolEvents(2)
	defer unsubscribeProtocol()
	statusFrames, unsubscribeStatus := h.serviceStatus.subscribeStatusEvents(4)
	defer unsubscribeStatus()
	var bilibiliFrames <-chan managementEventFrame
	unsubscribeBilibili := func() {}
	if h.bilibili != nil {
		bilibiliFrames, unsubscribeBilibili = h.bilibili.subscribe(4)
	}
	defer unsubscribeBilibili()
	var governanceFrames <-chan managementEventFrame
	unsubscribeGovernance := func() {}
	if h.governance != nil {
		governanceFrames, unsubscribeGovernance = h.governance.subscribeGovernanceEvents(4)
	}
	defer unsubscribeGovernance()

	for _, frame := range []managementEventFrame{
		h.serviceStatus.currentServiceStatusEvent(),
		h.protocol.protocolSnapshotEvent(),
		h.bilibili.currentEvent(),
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
