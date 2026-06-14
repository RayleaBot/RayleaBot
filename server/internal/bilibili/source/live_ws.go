package source

import (
	"context"

	bilibiliLive "github.com/RayleaBot/RayleaBot/server/internal/bilibili/live"
	"github.com/coder/websocket"
)

func (s *Source) consumeLiveWebSocket(ctx context.Context, subject Subject, roomID, wsURL, token, cookie string) error {
	headers := bilibiliLive.Headers(s.identity, cookie)
	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPHeader: headers, HTTPClient: s.client})
	if err != nil {
		return err
	}
	defer conn.CloseNow()

	verifyBytes := bilibiliLive.VerifyPayload(roomID, token, cookie)
	if err := conn.Write(ctx, websocket.MessageBinary, bilibiliLive.Pack(verifyBytes, 1, bilibiliLive.WSOpVerify)); err != nil {
		return err
	}
	state := s.loadRoomState(ctx, subject.UID)
	state.ConnectionState = StateConnected
	state.LastError = ""
	s.setRoomState(ctx, state)

	heartbeatDone := make(chan struct{})
	defer close(heartbeatDone)
	go bilibiliLive.StartSocketHeartbeat(ctx, conn, heartbeatDone)
	go s.startLiveHTTPHeartbeat(ctx, roomID, cookie, heartbeatDone)

	for {
		messageType, data, err := conn.Read(ctx)
		if err != nil {
			return err
		}
		if messageType != websocket.MessageBinary {
			continue
		}
		events, err := bilibiliLive.Unpack(data)
		if err != nil {
			return err
		}
		for _, event := range events {
			s.handleLiveWebSocketEvent(ctx, subject, roomID, event)
		}
	}
}
