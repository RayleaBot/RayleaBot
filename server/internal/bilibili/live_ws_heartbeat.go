package bilibili

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"github.com/coder/websocket"
)

const liveHeartbeatURL = "https://live-trace.bilibili.com/xlive/rdata-interface/v1/heartbeat/webHeartBeat"

func startLiveSocketHeartbeat(ctx context.Context, conn *websocket.Conn, done <-chan struct{}) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-ticker.C:
			_ = conn.Write(ctx, websocket.MessageBinary, liveWSPack([]byte("[object Object]"), 1, liveWSOpHeartbeat))
		}
	}
}

func (s *Source) startLiveHTTPHeartbeat(ctx context.Context, roomID, cookie string, done <-chan struct{}) {
	ticker := time.NewTicker(s.identity.JitteredDelay(60 * time.Second))
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-ticker.C:
			hbData := fmt.Sprintf(`{"room_id":%d,"hb_type":1}`, parseInt(roomID))
			hbEncoded := make([]byte, base64.StdEncoding.EncodedLen(len(hbData)))
			base64.StdEncoding.Encode(hbEncoded, []byte(hbData))
			hbURL := fmt.Sprintf("%s?pf=web&hb=%s", liveHeartbeatURL, string(hbEncoded))
			hbReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, hbURL, nil)
			s.identity.ApplyLiveHeaders(hbReq, http.MethodPost)
			if cookie != "" {
				hbReq.Header.Set("Cookie", cookie)
			}
			resp, err := s.client.Do(hbReq)
			if err == nil {
				resp.Body.Close()
			}
		}
	}
}
