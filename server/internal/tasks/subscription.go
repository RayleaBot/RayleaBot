package tasks

func (r *Registry) Subscribe(buffer int) (<-chan Snapshot, func()) {
	if buffer <= 0 {
		buffer = 1
	}

	ch := make(chan Snapshot, buffer)

	r.mu.Lock()
	id := r.nextSubscriberID
	r.nextSubscriberID++
	r.subscribers[id] = ch
	r.mu.Unlock()

	return ch, func() {
		r.mu.Lock()
		defer r.mu.Unlock()

		subscriber, ok := r.subscribers[id]
		if !ok {
			return
		}

		delete(r.subscribers, id)
		close(subscriber)
	}
}

func (r *Registry) SubscriberCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.subscribers)
}

func (r *Registry) broadcastLocked(snapshot Snapshot) {
	cloned := cloneSnapshot(snapshot)
	for _, subscriber := range r.subscribers {
		select {
		case subscriber <- cloned:
		default:
			select {
			case <-subscriber:
			default:
			}
			select {
			case subscriber <- cloned:
			default:
			}
		}
	}
}
