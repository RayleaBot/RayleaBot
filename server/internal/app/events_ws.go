package app

import (
	"context"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func (h *eventsWSHandler) handleEventsWebSocket() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := ClaimsFromContext(r.Context()); !ok {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() {
			_ = conn.Close(websocket.StatusNormalClosure, "")
		}()

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

		for _, frame := range []managementEventFrame{
			h.serviceStatus.currentServiceStatusEvent(),
			h.protocol.protocolSnapshotEvent(),
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
				if err := wsjson.Write(eventsCtx, conn, pluginStateEventFrame(snapshot)); err != nil {
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
			}
		}
	}
}

func pluginStateEventFrame(snapshot plugins.Snapshot) managementEventFrame {
	return managementEventFrame{
		Channel:   "events",
		Type:      "events.received",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data: map[string]any{
			"plugin_id":          snapshot.PluginID,
			"registration_state": snapshot.RegistrationState,
			"desired_state":      snapshot.DesiredState,
			"runtime_state":      snapshot.RuntimeState,
			"display_state":      snapshot.DisplayState,
		},
	}
}
