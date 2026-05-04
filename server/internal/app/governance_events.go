package app

import (
	"strings"
	"sync"
)

type governanceEventService struct {
	mu          sync.RWMutex
	nextSubID   uint64
	subscribers map[uint64]chan managementEventFrame
}

func newGovernanceEventService() *governanceEventService {
	return &governanceEventService{
		subscribers: make(map[uint64]chan managementEventFrame),
	}
}

func (s *governanceEventService) PublishChanged(summary string) {
	if s == nil {
		return
	}
	s.publishEvent(governanceChangedEventFrame(summary))
}

func governanceChangedEventFrame(summary string) managementEventFrame {
	summary = strings.TrimSpace(summary)
	if summary == "" {
		summary = "治理设置已更新"
	}
	return newEventsReceivedFrame(genericManagementEventPayload{
		EventType: "governance.changed",
		Summary:   summary,
	})
}

func (s *governanceEventService) publishEvent(frame managementEventFrame) {
	if s == nil {
		return
	}

	s.mu.RLock()
	subscribers := make([]chan managementEventFrame, 0, len(s.subscribers))
	for _, subscriber := range s.subscribers {
		subscribers = append(subscribers, subscriber)
	}
	s.mu.RUnlock()

	for _, subscriber := range subscribers {
		select {
		case subscriber <- frame:
		default:
		}
	}
}

func (s *governanceEventService) subscribeGovernanceEvents(buffer int) (<-chan managementEventFrame, func()) {
	if buffer <= 0 {
		buffer = 1
	}

	ch := make(chan managementEventFrame, buffer)
	s.mu.Lock()
	id := s.nextSubID
	s.nextSubID++
	s.subscribers[id] = ch
	s.mu.Unlock()

	return ch, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		subscriber, ok := s.subscribers[id]
		if !ok {
			return
		}
		delete(s.subscribers, id)
		close(subscriber)
	}
}
