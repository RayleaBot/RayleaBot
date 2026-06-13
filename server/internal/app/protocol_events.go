package app

func (s *protocolService) protocolSnapshotEvent() managementEventFrame {
	return newEventsReceivedFrame(protocolSnapshotEventPayload{
		Protocol:         "onebot11",
		ProtocolSnapshot: s.currentOneBot11ProtocolSnapshot(),
	})
}

func (s *protocolService) PublishSnapshot() {
	s.publishProtocolEvent(s.protocolSnapshotEvent())
}

func (s *protocolService) publishProtocolEvent(frame managementEventFrame) {
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

func (s *protocolService) subscribeProtocolEvents(buffer int) (<-chan managementEventFrame, func()) {
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
