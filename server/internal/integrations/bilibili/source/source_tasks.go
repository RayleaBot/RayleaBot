package source

import (
	"context"
	"strings"

	bilibilivalues "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/values"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
)

func (s *Source) ensureRoomTasks(ctx context.Context, subjects map[string]Subject, account thirdparty.Account, cookie string) {
	needed := make(map[string]Subject)
	if strings.TrimSpace(cookie) != "" {
		for uid, subject := range subjects {
			if subject.Services["live"] {
				needed[uid] = subject
			}
		}
	}
	fingerprint := bilibilivalues.CookieFingerprint(cookie)
	s.mu.Lock()
	for uid, task := range s.roomTasks {
		if _, ok := needed[uid]; !ok {
			task.cancel()
			delete(s.roomTasks, uid)
		}
	}
	for uid, subject := range needed {
		if task, ok := s.roomTasks[uid]; ok {
			if task.cookieFingerprint == fingerprint && task.accountID == account.AccountID {
				continue
			}
			task.cancel()
			delete(s.roomTasks, uid)
		}
		roomCtx, cancel := context.WithCancel(ctx)
		s.roomTasks[uid] = liveRoomTask{
			ctx:               roomCtx,
			cancel:            cancel,
			cookieFingerprint: fingerprint,
			accountID:         account.AccountID,
		}
		go s.runLiveRoom(roomCtx, subject, account, cookie)
	}
	s.mu.Unlock()
}
func (s *Source) stopRoomTasks() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for uid, task := range s.roomTasks {
		task.cancel()
		delete(s.roomTasks, uid)
	}
}
func (s *Source) updateWatchCounts(ctx context.Context, subjects map[string]Subject) {
	liveWatched := 0
	dynamicWatched := 0
	liveUIDs := make(map[string]bool)
	for _, subject := range subjects {
		if subject.Services["live"] {
			liveWatched++
			liveUIDs[subject.UID] = true
		}
		if bilibilivalues.HasDynamicService(subject.Services) {
			dynamicWatched++
		}
	}
	connected, failed := s.roomConnectionCounts(ctx, liveUIDs)
	s.mu.Lock()
	s.status.Live.WatchedRooms = liveWatched
	s.status.Live.ConnectedRooms = connected
	s.status.Live.FailedRooms = failed
	s.status.Live.FallbackPolling = liveWatched > 0
	s.status.Dynamic.Enabled = dynamicWatched > 0
	s.status.Dynamic.WatchedUIDs = dynamicWatched
	s.status.Dynamic.IntervalSeconds = defaultDynamicIntervalSeconds
	s.status.Dynamic.AutoFollow = true
	s.status.Status = s.deriveStateLocked()
	s.status.Summary = sourceSummary(s.status.Status)
	s.status.Diagnosis = s.diagnosisForStatusLocked(s.status, nil)
	status := s.status
	s.mu.Unlock()
	s.publishStatus(ctx, status)
}
