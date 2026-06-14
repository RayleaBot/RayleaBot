package source

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	bilibiliLive "github.com/RayleaBot/RayleaBot/server/internal/bilibili/live"
)

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
			hbData := fmt.Sprintf(`{"room_id":%d,"hb_type":1}`, bilibiliLive.ParseInt(roomID))
			hbEncoded := make([]byte, base64.StdEncoding.EncodedLen(len(hbData)))
			base64.StdEncoding.Encode(hbEncoded, []byte(hbData))
			hbURL := fmt.Sprintf("%s?pf=web&hb=%s", bilibiliLive.HeartbeatURL, string(hbEncoded))
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
