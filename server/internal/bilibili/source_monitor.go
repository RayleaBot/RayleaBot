package bilibili

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
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
	dynamics := s.loadDynamicSnapshots(ctx)
	for _, subject := range sortedSubjects(subjects) {
		room := s.loadRoomState(ctx, subject.UID)
		dynamic := dynamics[subject.UID]
		if !hasDynamicService(subject.Services) {
			dynamic = dynamicSnapshot{}
		}
		item := MonitorItem{
			UID:        subject.UID,
			Username:   firstNonEmpty(room.Name, dynamic.Username, subject.Name, subject.UID),
			AvatarURL:  firstNonEmpty(room.Face, dynamic.AvatarURL, subject.AvatarURL),
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
func monitorLiveFromRoom(room roomState) MonitorLive {
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
	if isBilibiliRiskControlErrorText(lastError) {
		return StateIdle
	}
	return firstNonEmpty(state, StateIdle)
}
func roomMonitorLastError(value string) string {
	if isBilibiliRiskControlErrorText(value) {
		return ""
	}
	return value
}
func (item dynamicSnapshot) MonitorDynamic() *MonitorDynamic {
	if item.UID == "" || item.DynamicID == "" {
		return nil
	}
	return &MonitorDynamic{
		LastID:      item.DynamicID,
		Service:     item.Service,
		Title:       item.Title,
		Summary:     item.Summary,
		URL:         item.URL,
		Images:      item.Images,
		PublishedAt: item.PublishedAt,
		ObservedAt:  item.ObservedAt,
	}
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
