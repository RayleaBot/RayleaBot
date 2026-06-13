package bilibili

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
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
	items, err := s.fetchLiveStatuses(ctx, []Subject{subject})
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
