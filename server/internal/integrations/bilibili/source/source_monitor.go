package source

import (
	"context"
	"sort"
	"strings"
	"time"

	bilibiliSession "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/session"
	sourcestate "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/source/state"
	bilibilivalues "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/values"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
)

func (s *Source) MonitorSnapshot(ctx context.Context) (MonitorSnapshot, error) {
	snapshot := MonitorSnapshot{
		Platform: thirdparty.PlatformBilibili,
		Items:    []MonitorItem{},
	}
	if s == nil {
		snapshot.UpdatedAt = time.Now().UTC()
		return snapshot, nil
	}
	subjects, err := s.loadSubjects(ctx)
	if err != nil {
		return snapshot, err
	}
	if err := s.refreshMonitorDynamics(ctx, subjects); err != nil {
		s.setDynamicError(err)
	}
	dynamics := s.stateStore.LoadDynamics(ctx)
	for _, subject := range sortedSubjects(subjects) {
		room := s.stateStore.LoadRoom(ctx, subject.UID, StateIdle)
		dynamic := dynamics[subject.UID]
		if !bilibilivalues.HasDynamicService(subject.Services) {
			dynamic = sourcestate.Dynamic{}
		}
		item := MonitorItem{
			UID:        subject.UID,
			Username:   bilibilivalues.FirstNonEmpty(room.Name, dynamic.Username, subject.Name, subject.UID),
			AvatarURL:  bilibilivalues.FirstNonEmpty(room.Face, dynamic.AvatarURL, subject.AvatarURL),
			ProfileURL: bilibiliProfileURL(subject.UID),
			Services:   sortedServiceNames(subject.Services),
			Dynamic:    dynamic.MonitorDynamic(),
			Live:       monitorLiveFromRoom(room),
			UpdatedAt:  latestTime(room.UpdatedAt, dynamic.UpdatedAt),
		}
		if item.UpdatedAt.IsZero() {
			item.UpdatedAt = s.now()
		}
		if snapshot.UpdatedAt.IsZero() || item.UpdatedAt.After(snapshot.UpdatedAt) {
			snapshot.UpdatedAt = item.UpdatedAt
		}
		snapshot.Items = append(snapshot.Items, item)
	}
	if snapshot.UpdatedAt.IsZero() {
		snapshot.UpdatedAt = s.now()
	}
	return snapshot, nil
}
func sortedSubjects(subjects map[string]Subject) []Subject {
	items := make([]Subject, 0, len(subjects))
	for _, subject := range subjects {
		items = append(items, subject)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].UID < items[j].UID
	})
	return items
}
func sortedServiceNames(services map[string]bool) []string {
	items := make([]string, 0, len(services))
	for service, enabled := range services {
		if enabled {
			items = append(items, service)
		}
	}
	sort.Strings(items)
	return items
}
func monitorLiveFromRoom(room sourcestate.Room) MonitorLive {
	live := MonitorLive{
		RoomID:          room.RoomID,
		RoomName:        room.Name,
		CoverURL:        room.CoverURL,
		IsLive:          room.LiveStatus == 1,
		ConnectionState: normalizeRoomConnectionState(room.ConnectionState, room.LastError),
		LastError:       roomMonitorLastError(room.LastError),
	}
	if room.RoomID != "" {
		live.RoomURL = "https://live.bilibili.com/" + room.RoomID
	}
	if room.LiveStartedAt > 0 {
		startedAt := time.Unix(room.LiveStartedAt, 0).UTC()
		live.LiveStartedAt = &startedAt
	}
	if room.LastEventAt != nil && room.LiveStatus == 0 && strings.TrimSpace(room.LiveEventID) != "" {
		live.LiveEndedAt = room.LastEventAt
	}
	if !room.UpdatedAt.IsZero() {
		live.UpdatedAt = &room.UpdatedAt
	}
	return live
}
func bilibiliProfileURL(uid string) string {
	uid = strings.TrimSpace(uid)
	if uid == "" {
		return ""
	}
	return "https://space.bilibili.com/" + uid + "/"
}
func normalizeRoomConnectionState(state string, lastError string) string {
	if bilibiliSession.IsRiskControlErrorText(lastError) {
		return StateIdle
	}
	return bilibilivalues.FirstNonEmpty(state, StateIdle)
}
func roomMonitorLastError(value string) string {
	if bilibiliSession.IsRiskControlErrorText(value) {
		return ""
	}
	return value
}
func (s *Source) roomConnectionCounts(ctx context.Context, watchedUIDs map[string]bool) (int, int) {
	return s.stateStore.RoomConnectionCounts(ctx, watchedUIDs, StateConnected, map[string]bool{
		StateFailed:   true,
		StateDegraded: true,
	})
}
func latestTime(values ...time.Time) time.Time {
	var latest time.Time
	for _, value := range values {
		if value.IsZero() {
			continue
		}
		if latest.IsZero() || value.After(latest) {
			latest = value
		}
	}
	return latest
}
