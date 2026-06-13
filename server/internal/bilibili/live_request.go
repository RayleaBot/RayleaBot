package bilibili

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

const (
	liveStatusBatchURL = "https://api.live.bilibili.com/room/v1/Room/get_status_info_by_uids"
	liveDanmuInfoURL   = "https://api.live.bilibili.com/xlive/web-room/v1/index/getDanmuInfo?id=%s&type=0"
)

type liveStatusDocument struct {
	Code int                       `json:"code"`
	Msg  string                    `json:"message"`
	Data map[string]liveStatusItem `json:"data"`
}

type liveStatusItem struct {
	UID            any    `json:"uid"`
	UName          string `json:"uname"`
	Face           string `json:"face"`
	RoomID         any    `json:"room_id"`
	Title          string `json:"title"`
	LiveStatus     int    `json:"live_status"`
	LiveTime       any    `json:"live_time"`
	LiveTimeCompat any    `json:"liveTime"`
	URL            string `json:"url"`
	CoverFromUser  string `json:"cover_from_user"`
	UserCover      string `json:"user_cover"`
}

type danmuInfoDocument struct {
	Code int `json:"code"`
	Data struct {
		Token    string `json:"token"`
		HostList []struct {
			Host    string `json:"host"`
			WSSPort int    `json:"wss_port"`
			WSPort  int    `json:"ws_port"`
		} `json:"host_list"`
	} `json:"data"`
}

func (s *Source) fetchLiveStatuses(ctx context.Context, subjects []Subject) (map[string]liveStatusItem, error) {
	if len(subjects) == 0 {
		return map[string]liveStatusItem{}, nil
	}
	values := make([]string, 0, len(subjects))
	for _, subject := range subjects {
		if subject.UID != "" {
			values = append(values, "uids[]="+subject.UID)
		}
	}
	if len(values) == 0 {
		return map[string]liveStatusItem{}, nil
	}
	var doc liveStatusDocument
	if err := s.requestJSON(ctx, http.MethodGet, liveStatusBatchURL+"?"+strings.Join(values, "&"), "", nil, &doc); err != nil {
		return nil, err
	}
	result := make(map[string]liveStatusItem, len(doc.Data))
	for uid, item := range doc.Data {
		key := strings.TrimSpace(uid)
		if key == "" {
			key = strings.TrimSpace(stringValue(item.UID))
		}
		if key != "" {
			result[key] = item
		}
	}
	return result, nil
}

func (s *Source) fetchDanmuInfo(ctx context.Context, roomID, cookie string) (danmuInfoDocument, error) {
	var doc danmuInfoDocument
	if err := s.requestSignedJSON(ctx, http.MethodGet, fmt.Sprintf(liveDanmuInfoURL, roomID), cookie, nil, &doc); err != nil {
		return doc, err
	}
	if strings.TrimSpace(doc.Data.Token) == "" {
		return doc, fmt.Errorf("live room %s websocket token missing", roomID)
	}
	return doc, nil
}

func (s *Source) pollLiveFallback(ctx context.Context, subjects map[string]Subject, account thirdparty.Account, cookie string) {
	if strings.TrimSpace(cookie) == "" {
		return
	}
	if delay := s.requestCooldownDelay(bilibiliRequestCooldownLive, account, cookie); delay > 0 {
		s.setLiveError(fmt.Errorf("Bilibili 直播检查因平台风控暂停，剩余 %s", formatCooldownDelay(delay)))
		return
	}
	liveSubjects := make([]Subject, 0)
	for _, subject := range sortedSubjects(subjects) {
		if subject.Services["live"] {
			liveSubjects = append(liveSubjects, subject)
		}
	}
	if len(liveSubjects) == 0 {
		s.clearLiveError()
		return
	}
	items, err := s.fetchLiveStatuses(ctx, liveSubjects)
	if err != nil {
		_ = s.handleAccountRequestError(ctx, account, cookie, bilibiliRequestCooldownLive, err)
		s.setLiveError(err)
		return
	}
	s.clearRequestCooldown(bilibiliRequestCooldownLive, account, cookie)
	s.clearLiveError()
	for _, subject := range liveSubjects {
		item, ok := items[subject.UID]
		if !ok {
			continue
		}
		s.emitLiveTransition(ctx, subject, item, normalizeLiveStatus(item.LiveStatus), "fallback")
	}
}
