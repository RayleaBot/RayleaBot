package source

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	bilibiliLive "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/live"
	bilibilimonitoring "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/monitoring"
	sourcestate "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/source/state"
	bilibilivalues "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/values"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"github.com/coder/websocket"
)

func (s *Source) runLiveRoom(ctx context.Context, subject Subject, account thirdparty.Account, cookie string) {
	backoff := time.Second
	for {
		if err := ctx.Err(); err != nil {
			return
		}
		if delay := s.requestCooldownDelay(bilibiliRequestCooldownLive, account, cookie); delay > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
			continue
		}
		if err := s.connectLiveRoom(ctx, subject, cookie); err != nil {
			if s.handleAccountRequestError(ctx, account, cookie, bilibiliRequestCooldownLive, err) == accountRequestErrorAuth {
				s.setLiveError(err)
				return
			}
			state := s.stateStore.LoadRoom(ctx, subject.UID, StateIdle)
			state.UID = subject.UID
			state.Name = bilibilivalues.FirstNonEmpty(state.Name, subject.Name)
			state.Face = bilibilivalues.FirstNonEmpty(state.Face, subject.AvatarURL)
			state.ConnectionState = StateDegraded
			state.LastError = err.Error()
			s.stateStore.SetRoom(ctx, state)
			s.setLiveError(err)
		} else {
			s.clearRequestCooldown(bilibiliRequestCooldownLive, account, cookie)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

func (s *Source) connectLiveRoom(ctx context.Context, subject Subject, cookie string) error {
	items, err := s.fetchLiveStatuses(ctx, []Subject{subject})
	if err != nil {
		return err
	}
	item, ok := items[subject.UID]
	if !ok {
		return fmt.Errorf("live room status missing for uid %s", subject.UID)
	}
	roomID := strings.TrimSpace(bilibilivalues.String(item.RoomID))
	if roomID == "" || roomID == "0" {
		state := sourcestate.Room{
			UID:             subject.UID,
			Name:            bilibilivalues.FirstNonEmpty(item.UName, subject.Name),
			Face:            bilibilivalues.FirstNonEmpty(bilibilivalues.NormalizeURL(item.Face), subject.AvatarURL),
			LiveStatus:      bilibiliLive.NormalizeStatus(item.LiveStatus),
			ConnectionState: StateIdle,
		}
		s.stateStore.SetRoom(ctx, state)
		return fmt.Errorf("uid %s has no live room", subject.UID)
	}

	state := s.stateStore.LoadRoom(ctx, subject.UID, StateIdle)
	state.UID = subject.UID
	state.RoomID = roomID
	state.Name = bilibilivalues.FirstNonEmpty(item.UName, subject.Name)
	state.Face = bilibilivalues.FirstNonEmpty(bilibilivalues.NormalizeURL(item.Face), subject.AvatarURL)
	state.CoverURL = bilibiliLive.FirstImageURL(item)
	state.LiveStatus = bilibiliLive.NormalizeStatus(item.LiveStatus)
	state.LiveStartedAt = bilibiliLive.TimeFromItem(item)
	state.ConnectionState = StateConnecting
	state.LastError = ""
	s.stateStore.SetRoom(ctx, state)
	if state.LiveStatus == 1 {
		s.emitLiveTransition(ctx, subject, item, state.LiveStatus, "status")
	}

	conf, err := s.fetchDanmuInfo(ctx, roomID, cookie)
	if err != nil {
		return err
	}
	if len(conf.Data.HostList) == 0 {
		return fmt.Errorf("live room %s websocket hosts missing", roomID)
	}
	for _, host := range conf.Data.HostList {
		if strings.TrimSpace(host.Host) == "" {
			continue
		}
		port := host.WSSPort
		if port <= 0 {
			port = host.WSPort
		}
		if port <= 0 {
			port = 443
		}
		wsURL := fmt.Sprintf("wss://%s:%d/sub", host.Host, port)
		if err := s.consumeLiveWebSocket(ctx, subject, roomID, wsURL, conf.Data.Token, cookie); err != nil {
			return err
		}
	}
	return fmt.Errorf("live room %s websocket hosts exhausted", roomID)
}

func (s *Source) emitLiveTransition(ctx context.Context, subject Subject, item bilibiliLive.StatusItem, liveStatus int, source string) {
	liveStatus = bilibiliLive.NormalizeStatus(liveStatus)
	state := s.stateStore.LoadRoom(ctx, subject.UID, StateIdle)
	if state.UID == "" {
		state.UID = subject.UID
	}
	roomID := strings.TrimSpace(bilibilivalues.String(item.RoomID))
	if roomID != "" {
		state.RoomID = roomID
	}
	state.Name = bilibilivalues.FirstNonEmpty(item.UName, subject.Name, state.Name)
	state.Face = bilibilivalues.FirstNonEmpty(bilibilivalues.NormalizeURL(item.Face), subject.AvatarURL, state.Face)
	state.CoverURL = bilibilivalues.FirstNonEmpty(bilibiliLive.FirstImageURL(item), state.CoverURL)
	state.LiveStartedAt = bilibiliLive.TimeFromItem(item)
	state.ConnectionState = bilibilivalues.FirstNonEmpty(state.ConnectionState, StateIdle)
	if state.LiveStatus == liveStatus && source != "status" {
		s.stateStore.SetRoom(ctx, state)
		return
	}
	state.LiveStatus = liveStatus
	now := s.now()
	state.LastEventAt = &now
	state.LastError = ""

	event := bilibilimonitoring.LiveTransitionEvent(bilibilimonitoring.LiveTransitionInput{
		Subject:       subject,
		Item:          item,
		RoomID:        state.RoomID,
		Name:          state.Name,
		Face:          state.Face,
		LiveStartedAt: state.LiveStartedAt,
		LiveStatus:    liveStatus,
		Now:           now,
	})
	state.LiveEventID = event.ID
	s.stateStore.SetRoom(ctx, state)
	seenKey := event.EventType + ":" + event.ID
	if !s.markSeen(ctx, seenKey, subject.UID, event.EventType, event.ID) {
		return
	}
	s.dispatchEvent(ctx, event)
}

func (s *Source) emitSyntheticLiveTransition(ctx context.Context, subject Subject, roomID string, liveStatus int) {
	item := bilibiliLive.StatusItem{
		UID:        subject.UID,
		UName:      subject.Name,
		Face:       subject.AvatarURL,
		RoomID:     roomID,
		LiveStatus: liveStatus,
		LiveTime:   s.now().Unix(),
		URL:        "https://live.bilibili.com/" + roomID,
	}
	s.emitLiveTransition(ctx, subject, item, liveStatus, "websocket")
}

func (s *Source) fetchLiveStatuses(ctx context.Context, subjects []Subject) (map[string]bilibiliLive.StatusItem, error) {
	if len(subjects) == 0 {
		return map[string]bilibiliLive.StatusItem{}, nil
	}
	values := make([]string, 0, len(subjects))
	for _, subject := range subjects {
		if subject.UID != "" {
			values = append(values, "uids[]="+subject.UID)
		}
	}
	if len(values) == 0 {
		return map[string]bilibiliLive.StatusItem{}, nil
	}
	var doc bilibiliLive.StatusDocument
	if err := s.requestJSON(ctx, http.MethodGet, bilibiliLive.StatusBatchURL+"?"+strings.Join(values, "&"), "", nil, &doc); err != nil {
		return nil, err
	}
	result := make(map[string]bilibiliLive.StatusItem, len(doc.Data))
	for uid, item := range doc.Data {
		key := strings.TrimSpace(uid)
		if key == "" {
			key = strings.TrimSpace(bilibilivalues.String(item.UID))
		}
		if key != "" {
			result[key] = item
		}
	}
	return result, nil
}

func (s *Source) fetchDanmuInfo(ctx context.Context, roomID, cookie string) (bilibiliLive.DanmuInfoDocument, error) {
	var doc bilibiliLive.DanmuInfoDocument
	if err := s.requestSignedJSON(ctx, http.MethodGet, fmt.Sprintf(bilibiliLive.DanmuInfoURL, roomID), cookie, nil, &doc); err != nil {
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
		s.setLiveError(fmt.Errorf("Bilibili 直播检查因平台风控暂停，剩余 %s", bilibilivalues.FormatCooldownDelay(delay)))
		return
	}
	liveSubjects := make([]Subject, 0)
	for _, subject := range sortedSubjects(subjects) {
		if subject.Services["live"] {
			liveSubjects = append(liveSubjects, subject)
		}
	}
	if len(liveSubjects) == 0 {
		s.clearLiveError(ctx)
		return
	}
	items, err := s.fetchLiveStatuses(ctx, liveSubjects)
	if err != nil {
		_ = s.handleAccountRequestError(ctx, account, cookie, bilibiliRequestCooldownLive, err)
		s.setLiveError(err)
		return
	}
	s.clearRequestCooldown(bilibiliRequestCooldownLive, account, cookie)
	s.clearLiveError(ctx)
	for _, subject := range liveSubjects {
		item, ok := items[subject.UID]
		if !ok {
			continue
		}
		s.emitLiveTransition(ctx, subject, item, bilibiliLive.NormalizeStatus(item.LiveStatus), "fallback")
	}
}

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
	state := s.stateStore.LoadRoom(ctx, subject.UID, StateIdle)
	state.ConnectionState = StateConnected
	state.LastError = ""
	s.stateStore.SetRoom(ctx, state)

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

func (s *Source) handleLiveWebSocketEvent(ctx context.Context, subject Subject, roomID string, event map[string]any) {
	cmd := strings.TrimSpace(bilibilivalues.String(event["cmd"]))
	if strings.Contains(cmd, ":") {
		cmd = strings.SplitN(cmd, ":", 2)[0]
	}
	switch cmd {
	case "LIVE":
		items, err := s.fetchLiveStatuses(ctx, []Subject{subject})
		if err != nil {
			s.emitSyntheticLiveTransition(ctx, subject, roomID, 1)
			return
		}
		if item, ok := items[subject.UID]; ok {
			s.emitLiveTransition(ctx, subject, item, 1, "websocket")
		}
	case "PREPARING":
		items, err := s.fetchLiveStatuses(ctx, []Subject{subject})
		if err != nil {
			s.emitSyntheticLiveTransition(ctx, subject, roomID, 0)
			return
		}
		if item, ok := items[subject.UID]; ok {
			s.emitLiveTransition(ctx, subject, item, 0, "websocket")
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
