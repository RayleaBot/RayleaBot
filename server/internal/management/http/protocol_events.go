package managementhttp

import managementevents "github.com/RayleaBot/RayleaBot/server/internal/management/events"

func (s *ProtocolService) ProtocolSnapshotEvent() managementevents.Frame {
	return managementevents.NewReceivedFrame(managementevents.ProtocolSnapshotPayload{
		Protocol:         "onebot11",
		ProtocolSnapshot: s.currentOneBot11ProtocolSnapshot(),
	})
}

func (s *ProtocolService) PublishSnapshot() {
	s.publishProtocolEvent(s.ProtocolSnapshotEvent())
}

func (s *ProtocolService) publishProtocolEvent(frame managementevents.Frame) {
	if s == nil {
		return
	}

	s.mu.RLock()
	subscribers := make([]chan managementevents.Frame, 0, len(s.subscribers))
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

func (s *ProtocolService) SubscribeProtocolEvents(buffer int) (<-chan managementevents.Frame, func()) {
	if buffer <= 0 {
		buffer = 1
	}

	ch := make(chan managementevents.Frame, buffer)
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
