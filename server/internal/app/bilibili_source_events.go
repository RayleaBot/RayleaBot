package app

import (
	"sync"
	"time"

	source "github.com/RayleaBot/RayleaBot/server/internal/bilibili"
)

type bilibiliSourceEventService struct {
	mu          sync.Mutex
	subscribers map[uint64]chan managementEventFrame
	nextID      uint64
	current     source.Status
}

func newBilibiliSourceEventService() *bilibiliSourceEventService {
	return &bilibiliSourceEventService{
		subscribers: make(map[uint64]chan managementEventFrame),
		current: source.Status{
			Status:  source.StateIdle,
			Summary: "Bilibili 事件源等待订阅",
		},
	}
}

func (s *bilibiliSourceEventService) Publish(status source.Status) {
	if s == nil {
		return
	}
	frame := bilibiliSourceStatusEventFrame(status)
	s.mu.Lock()
	s.current = status
	subscribers := make([]chan managementEventFrame, 0, len(s.subscribers))
	for _, ch := range s.subscribers {
		subscribers = append(subscribers, ch)
	}
	s.mu.Unlock()
	for _, ch := range subscribers {
		select {
		case ch <- frame:
		default:
		}
	}
}

func (s *bilibiliSourceEventService) currentEvent() managementEventFrame {
	if s == nil {
		return bilibiliSourceStatusEventFrame(source.Status{Status: source.StateIdle, Summary: "Bilibili 事件源等待订阅"})
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return bilibiliSourceStatusEventFrame(s.current)
}

func (s *bilibiliSourceEventService) subscribe(buffer int) (<-chan managementEventFrame, func()) {
	if s == nil {
		return nil, func() {}
	}
	if buffer <= 0 {
		buffer = 1
	}
	s.mu.Lock()
	s.nextID++
	id := s.nextID
	ch := make(chan managementEventFrame, buffer)
	s.subscribers[id] = ch
	s.mu.Unlock()
	return ch, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if _, ok := s.subscribers[id]; ok {
			delete(s.subscribers, id)
			close(ch)
		}
	}
}

type bilibiliSourceEventPayload struct {
	Source             string  `json:"source"`
	Status             string  `json:"status"`
	Summary            string  `json:"summary"`
	LiveWatchedRooms   int     `json:"live_watched_rooms"`
	LiveConnectedRooms int     `json:"live_connected_rooms"`
	LiveFailedRooms    int     `json:"live_failed_rooms"`
	FallbackPolling    bool    `json:"fallback_polling"`
	DynamicEnabled     bool    `json:"dynamic_enabled"`
	DynamicWatchedUIDs int     `json:"dynamic_watched_uids"`
	LastEventAt        *string `json:"last_event_at"`
	LastError          string  `json:"last_error"`
}

func bilibiliSourceStatusEventFrame(status source.Status) managementEventFrame {
	var lastEventAt *string
	if value := newestTimeString(status.Live.LastEventAt, status.Dynamic.LastEventAt); value != "" {
		lastEventAt = &value
	}
	lastError := status.Live.LastError
	if lastError == "" {
		lastError = status.Dynamic.LastError
	}
	return newEventsReceivedFrame(bilibiliSourceEventPayload{
		Source:             "bilibili",
		Status:             status.Status,
		Summary:            status.Summary,
		LiveWatchedRooms:   status.Live.WatchedRooms,
		LiveConnectedRooms: status.Live.ConnectedRooms,
		LiveFailedRooms:    status.Live.FailedRooms,
		FallbackPolling:    status.Live.FallbackPolling,
		DynamicEnabled:     status.Dynamic.Enabled,
		DynamicWatchedUIDs: status.Dynamic.WatchedUIDs,
		LastEventAt:        lastEventAt,
		LastError:          lastError,
	})
}

func newestTimeString(values ...*time.Time) string {
	var newest *time.Time
	for _, value := range values {
		if value == nil || value.IsZero() {
			continue
		}
		if newest == nil || value.After(*newest) {
			newest = value
		}
	}
	if newest == nil {
		return ""
	}
	return newest.UTC().Format(time.RFC3339)
}
