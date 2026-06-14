package events

import (
	"strings"
	"sync"
)

type GovernanceService struct {
	mu          sync.RWMutex
	nextSubID   uint64
	subscribers map[uint64]chan Frame
}

func NewGovernanceService() *GovernanceService {
	return &GovernanceService{
		subscribers: make(map[uint64]chan Frame),
	}
}

func (s *GovernanceService) PublishChanged(summary string) {
	if s == nil {
		return
	}
	s.publishEvent(governanceChangedEventFrame(summary))
}

func governanceChangedEventFrame(summary string) Frame {
	summary = strings.TrimSpace(summary)
	if summary == "" {
		summary = "治理设置已更新"
	}
	return NewReceivedFrame(GenericPayload{
		EventType: "governance.changed",
		Summary:   summary,
	})
}

func (s *GovernanceService) publishEvent(frame Frame) {
	if s == nil {
		return
	}

	s.mu.RLock()
	subscribers := make([]chan Frame, 0, len(s.subscribers))
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

func (s *GovernanceService) Subscribe(buffer int) (<-chan Frame, func()) {
	if buffer <= 0 {
		buffer = 1
	}

	ch := make(chan Frame, buffer)
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
