package logging

func (s *Stream) appendInMemory(summary Summary) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.history) == s.limit {
		copy(s.history, s.history[1:])
		s.history[len(s.history)-1] = summary
	} else {
		s.history = append(s.history, summary)
	}

	for _, subscriber := range s.subscribers {
		select {
		case subscriber <- summary:
		default:
			select {
			case <-subscriber:
			default:
			}
			select {
			case subscriber <- summary:
			default:
			}
		}
	}
}

func (s *Stream) Subscribe(buffer int) (<-chan Summary, func()) {
	if buffer <= 0 {
		buffer = 1
	}

	ch := make(chan Summary, buffer)

	s.mu.Lock()
	id := s.nextSubscriberID
	s.nextSubscriberID++
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

func (s *Stream) SubscriberCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.subscribers)
}
