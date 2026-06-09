package bilibili

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
	"github.com/coder/websocket"
)

const (
	liveStatusBatchURL = "https://api.live.bilibili.com/room/v1/Room/get_status_info_by_uids"
	liveDanmuInfoURL   = "https://api.live.bilibili.com/xlive/web-room/v1/index/getDanmuInfo?id=%s&type=0"

	liveWSHeaderSize = 16
	liveWSProtoRaw   = 0
	liveWSProtoZlib  = 2

	liveWSOpHeartbeat      = 2
	liveWSOpHeartbeatReply = 3
	liveWSOpNotice         = 5
	liveWSOpVerify         = 7
	liveWSOpVerifyReply    = 8
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
			state := s.loadRoomState(ctx, subject.UID)
			state.UID = subject.UID
			state.Name = firstNonEmpty(state.Name, subject.Name)
			state.Face = firstNonEmpty(state.Face, subject.AvatarURL)
			state.ConnectionState = StateDegraded
			state.LastError = err.Error()
			s.setRoomState(ctx, state)
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
	items, err := s.fetchLiveStatuses(ctx, []Subject{subject}, cookie)
	if err != nil {
		return err
	}
	item, ok := items[subject.UID]
	if !ok {
		return fmt.Errorf("live room status missing for uid %s", subject.UID)
	}
	roomID := strings.TrimSpace(stringValue(item.RoomID))
	if roomID == "" || roomID == "0" {
		state := roomState{
			UID:             subject.UID,
			Name:            firstNonEmpty(item.UName, subject.Name),
			Face:            firstNonEmpty(normalizeURL(item.Face), subject.AvatarURL),
			LiveStatus:      normalizeLiveStatus(item.LiveStatus),
			ConnectionState: StateIdle,
		}
		s.setRoomState(ctx, state)
		return fmt.Errorf("uid %s has no live room", subject.UID)
	}

	state := s.loadRoomState(ctx, subject.UID)
	state.UID = subject.UID
	state.RoomID = roomID
	state.Name = firstNonEmpty(item.UName, subject.Name)
	state.Face = firstNonEmpty(normalizeURL(item.Face), subject.AvatarURL)
	state.CoverURL = firstLiveImageURL(item)
	state.LiveStatus = normalizeLiveStatus(item.LiveStatus)
	state.LiveStartedAt = liveTimeFromItem(item)
	state.ConnectionState = StateConnecting
	state.LastError = ""
	s.setRoomState(ctx, state)
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

func (s *Source) fetchLiveStatuses(ctx context.Context, subjects []Subject, cookie string) (map[string]liveStatusItem, error) {
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
	if err := s.requestJSON(ctx, http.MethodGet, liveStatusBatchURL+"?"+strings.Join(values, "&"), cookie, nil, &doc); err != nil {
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
	if err := s.requestJSON(ctx, http.MethodGet, fmt.Sprintf(liveDanmuInfoURL, roomID), cookie, nil, &doc); err != nil {
		return doc, err
	}
	if strings.TrimSpace(doc.Data.Token) == "" {
		return doc, fmt.Errorf("live room %s websocket token missing", roomID)
	}
	return doc, nil
}

func (s *Source) consumeLiveWebSocket(ctx context.Context, subject Subject, roomID, wsURL, token, cookie string) error {
	headers := http.Header{}
	headers.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	if cookie != "" {
		headers.Set("Cookie", cookie)
	}
	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPHeader: headers})
	if err != nil {
		return err
	}
	defer conn.CloseNow()

	verify := map[string]any{
		"uid":      0,
		"roomid":   parseInt(roomID),
		"protover": liveWSProtoZlib,
		"platform": "web",
		"type":     2,
		"key":      token,
	}
	verifyBytes, _ := json.Marshal(verify)
	if err := conn.Write(ctx, websocket.MessageBinary, liveWSPack(verifyBytes, 1, liveWSOpVerify)); err != nil {
		return err
	}
	state := s.loadRoomState(ctx, subject.UID)
	state.ConnectionState = StateConnected
	state.LastError = ""
	s.setRoomState(ctx, state)

	heartbeatDone := make(chan struct{})
	defer close(heartbeatDone)
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-heartbeatDone:
				return
			case <-ticker.C:
				_ = conn.Write(ctx, websocket.MessageBinary, liveWSPack([]byte("[object Object]"), 1, liveWSOpHeartbeat))
			}
		}
	}()

	for {
		messageType, data, err := conn.Read(ctx)
		if err != nil {
			return err
		}
		if messageType != websocket.MessageBinary {
			continue
		}
		events, err := liveWSUnpack(data)
		if err != nil {
			return err
		}
		for _, event := range events {
			cmd := strings.TrimSpace(stringValue(event["cmd"]))
			if strings.Contains(cmd, ":") {
				cmd = strings.SplitN(cmd, ":", 2)[0]
			}
			switch cmd {
			case "LIVE":
				items, err := s.fetchLiveStatuses(ctx, []Subject{subject}, cookie)
				if err != nil {
					s.emitSyntheticLiveTransition(ctx, subject, roomID, 1)
					continue
				}
				if item, ok := items[subject.UID]; ok {
					s.emitLiveTransition(ctx, subject, item, 1, "websocket")
				}
			case "PREPARING":
				items, err := s.fetchLiveStatuses(ctx, []Subject{subject}, cookie)
				if err != nil {
					s.emitSyntheticLiveTransition(ctx, subject, roomID, 0)
					continue
				}
				if item, ok := items[subject.UID]; ok {
					s.emitLiveTransition(ctx, subject, item, 0, "websocket")
				}
			}
		}
	}
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
		return
	}
	items, err := s.fetchLiveStatuses(ctx, liveSubjects, cookie)
	if err != nil {
		_ = s.handleAccountRequestError(ctx, account, cookie, bilibiliRequestCooldownLive, err)
		s.setLiveError(err)
		return
	}
	s.clearRequestCooldown(bilibiliRequestCooldownLive, account, cookie)
	for _, subject := range liveSubjects {
		item, ok := items[subject.UID]
		if !ok {
			continue
		}
		s.emitLiveTransition(ctx, subject, item, normalizeLiveStatus(item.LiveStatus), "fallback")
	}
}

func (s *Source) emitLiveTransition(ctx context.Context, subject Subject, item liveStatusItem, liveStatus int, source string) {
	liveStatus = normalizeLiveStatus(liveStatus)
	state := s.loadRoomState(ctx, subject.UID)
	if state.UID == "" {
		state.UID = subject.UID
	}
	roomID := strings.TrimSpace(stringValue(item.RoomID))
	if roomID != "" {
		state.RoomID = roomID
	}
	state.Name = firstNonEmpty(item.UName, subject.Name, state.Name)
	state.Face = firstNonEmpty(normalizeURL(item.Face), subject.AvatarURL, state.Face)
	state.CoverURL = firstNonEmpty(firstLiveImageURL(item), state.CoverURL)
	state.LiveStartedAt = liveTimeFromItem(item)
	state.ConnectionState = firstNonEmpty(state.ConnectionState, StateIdle)
	if state.LiveStatus == liveStatus && source != "status" {
		s.setRoomState(ctx, state)
		return
	}
	state.LiveStatus = liveStatus
	now := s.now()
	state.LastEventAt = &now
	state.LastError = ""

	eventType := EventLiveStarted
	liveEvent := "started"
	statusLabel := "直播中"
	title := firstNonEmpty(item.Title, "直播间已开播")
	summary := "直播中"
	pubTS := state.LiveStartedAt
	if liveStatus == 0 {
		eventType = EventLiveEnded
		liveEvent = "ended"
		statusLabel = "直播结束"
		title = firstNonEmpty(item.Title, "直播结束")
		summary = "直播结束"
		pubTS = now.Unix()
	}
	if pubTS <= 0 {
		pubTS = now.Unix()
	}
	eventID := fmt.Sprintf("live-%s-%s-%s-%d", subject.UID, state.RoomID, liveEvent, pubTS)
	state.LiveEventID = eventID
	s.setRoomState(ctx, state)
	seenKey := eventType + ":" + eventID
	if !s.markSeen(ctx, seenKey, subject.UID, eventType, eventID) {
		return
	}
	liveStatusCopy := liveStatus
	event := BilibiliEvent{
		EventType: eventType,
		Kind:      "live",
		UID:       subject.UID,
		ID:        eventID,
		RoomID:    state.RoomID,
		Service:   "live",
		Title:     title,
		Summary:   summary,
		URL:       firstNonEmpty(item.URL, "https://live.bilibili.com/"+state.RoomID),
		PubTS:     pubTS,
		CreatedAt: formatTime(pubTS),
		Author: Author{
			UID:    subject.UID,
			Name:   firstNonEmpty(state.Name, subject.Name, subject.UID),
			Avatar: state.Face,
		},
		Images:      liveImages(item),
		LiveStatus:  &liveStatusCopy,
		LiveEvent:   liveEvent,
		StatusLabel: statusLabel,
	}
	if liveStatus == 1 {
		event.LiveStartedAt = formatTime(pubTS)
	} else {
		event.LiveDetectedAt = formatTime(now.Unix())
	}
	s.dispatchEvent(ctx, event)
}

func (s *Source) emitSyntheticLiveTransition(ctx context.Context, subject Subject, roomID string, liveStatus int) {
	item := liveStatusItem{
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

func liveImages(item liveStatusItem) []Image {
	if url := firstLiveImageURL(item); url != "" {
		return []Image{{URL: url}}
	}
	return nil
}

func firstLiveImageURL(item liveStatusItem) string {
	for _, value := range []string{item.CoverFromUser, item.UserCover} {
		if url := normalizeURL(value); url != "" {
			return url
		}
	}
	return ""
}

func liveTimeFromItem(item liveStatusItem) int64 {
	for _, value := range []any{item.LiveTime, item.LiveTimeCompat} {
		switch typed := value.(type) {
		case float64:
			if typed > 0 {
				return int64(typed)
			}
		case int:
			if typed > 0 {
				return int64(typed)
			}
		case int64:
			if typed > 0 {
				return typed
			}
		case string:
			text := strings.TrimSpace(typed)
			if text == "" || text == "0000-00-00 00:00:00" {
				continue
			}
			if parsed, err := strconv.ParseInt(text, 10, 64); err == nil && parsed > 0 {
				return parsed
			}
			if parsed, err := time.ParseInLocation("2006-01-02 15:04:05", text, time.Local); err == nil {
				return parsed.Unix()
			}
		}
	}
	return 0
}

func normalizeLiveStatus(value int) int {
	if value == 1 {
		return 1
	}
	return 0
}

func liveWSPack(body []byte, protocolVersion int, operation int) []byte {
	packetLength := liveWSHeaderSize + len(body)
	buffer := bytes.NewBuffer(make([]byte, 0, packetLength))
	_ = binary.Write(buffer, binary.BigEndian, uint32(packetLength))
	_ = binary.Write(buffer, binary.BigEndian, uint16(liveWSHeaderSize))
	_ = binary.Write(buffer, binary.BigEndian, uint16(protocolVersion))
	_ = binary.Write(buffer, binary.BigEndian, uint32(operation))
	_ = binary.Write(buffer, binary.BigEndian, uint32(1))
	buffer.Write(body)
	return buffer.Bytes()
}

func liveWSUnpack(data []byte) ([]map[string]any, error) {
	if len(data) < liveWSHeaderSize {
		return nil, nil
	}
	protocol := int(binary.BigEndian.Uint16(data[6:8]))
	operation := int(binary.BigEndian.Uint32(data[8:12]))
	if operation == liveWSOpHeartbeatReply || operation == liveWSOpVerifyReply {
		return nil, nil
	}
	if protocol == liveWSProtoZlib {
		reader, err := zlib.NewReader(bytes.NewReader(data[liveWSHeaderSize:]))
		if err != nil {
			return nil, err
		}
		defer reader.Close()
		inflated, err := io.ReadAll(reader)
		if err != nil {
			return nil, err
		}
		return liveWSUnpack(inflated)
	}
	result := []map[string]any{}
	offset := 0
	for offset+liveWSHeaderSize <= len(data) {
		packetLength := int(binary.BigEndian.Uint32(data[offset : offset+4]))
		if packetLength <= liveWSHeaderSize || offset+packetLength > len(data) {
			break
		}
		op := int(binary.BigEndian.Uint32(data[offset+8 : offset+12]))
		if op == liveWSOpNotice {
			body := data[offset+liveWSHeaderSize : offset+packetLength]
			var item map[string]any
			if err := json.Unmarshal(body, &item); err == nil {
				result = append(result, item)
			}
		}
		offset += packetLength
	}
	if len(result) == 0 && protocol == liveWSProtoRaw && operation == liveWSOpNotice {
		body := data[liveWSHeaderSize:]
		var item map[string]any
		if err := json.Unmarshal(body, &item); err == nil {
			result = append(result, item)
		}
	}
	return result, nil
}

func parseInt(value string) int64 {
	parsed, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return parsed
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
