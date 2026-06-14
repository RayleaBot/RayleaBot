package live

import (
	"context"
	"time"

	"github.com/coder/websocket"
)

const HeartbeatURL = "https://live-trace.bilibili.com/xlive/rdata-interface/v1/heartbeat/webHeartBeat"

func StartSocketHeartbeat(ctx context.Context, conn *websocket.Conn, done <-chan struct{}) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-ticker.C:
			_ = conn.Write(ctx, websocket.MessageBinary, Pack([]byte("[object Object]"), 1, WSOpHeartbeat))
		}
	}
}
