package app

import (
	"context"
	"net/http"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
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
		protocolFrames, unsubscribeProtocol := h.protocol.subscribeProtocolEvents(2)
		defer unsubscribeProtocol()

		for _, frame := range []managementEventFrame{
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
			case frame, ok := <-protocolFrames:
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
