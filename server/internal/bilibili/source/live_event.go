package source

import (
	"context"
	"fmt"
	"strings"

	bilibiliLive "github.com/RayleaBot/RayleaBot/server/internal/bilibili/live"
)

func (s *Source) emitLiveTransition(ctx context.Context, subject Subject, item bilibiliLive.StatusItem, liveStatus int, source string) {
	liveStatus = bilibiliLive.NormalizeStatus(liveStatus)
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
	state.CoverURL = firstNonEmpty(bilibiliLive.FirstImageURL(item), state.CoverURL)
	state.LiveStartedAt = bilibiliLive.TimeFromItem(item)
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
