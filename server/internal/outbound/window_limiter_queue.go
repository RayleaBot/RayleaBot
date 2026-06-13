package outbound

import "time"

func (l *windowLimiter) enqueue(key string) *windowWaiter {
	waiter := &windowWaiter{ready: make(chan struct{})}

	l.mu.Lock()
	defer l.mu.Unlock()

	state := l.windows[key]
	if state == nil {
		state = &windowState{}
		l.windows[key] = state
	}
	wasEmpty := len(state.queue) == 0
	state.queue = append(state.queue, waiter)
	if wasEmpty {
		close(waiter.ready)
	}
	return waiter
}

func (l *windowLimiter) tryReserve(key string, waiter *windowWaiter) (time.Duration, <-chan struct{}, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	state := l.windows[key]
	if state == nil || len(state.queue) == 0 || state.queue[0] != waiter {
		return time.Millisecond, l.updated, false
	}

	now := l.now().UTC()
	state.records = pruneWindowRecords(state.records, now, l.limit.Window)
	if len(state.records) < l.limit.Count {
		state.records = append(state.records, now)
		l.popHead(key, state)
		return 0, l.updated, true
	}

	waitUntil := state.records[0].Add(l.limit.Window)
	waitFor := time.Until(waitUntil)
	if waitFor <= 0 {
		waitFor = time.Millisecond
	}
	return waitFor, l.updated, false
}

func (l *windowLimiter) cancelWaiter(key string, waiter *windowWaiter) {
	l.mu.Lock()
	defer l.mu.Unlock()

	state := l.windows[key]
	if state == nil {
		return
	}
	for index, candidate := range state.queue {
		if candidate != waiter {
			continue
		}
		state.queue = append(state.queue[:index], state.queue[index+1:]...)
		if index == 0 && len(state.queue) > 0 {
			close(state.queue[0].ready)
		}
		break
	}
	now := l.now().UTC()
	state.records = pruneWindowRecords(state.records, now, l.limit.Window)
	if len(state.records) == 0 && len(state.queue) == 0 {
		delete(l.windows, key)
	}
}

func (l *windowLimiter) popHead(key string, state *windowState) {
	if len(state.queue) > 0 {
		state.queue = state.queue[1:]
	}
	if len(state.queue) > 0 {
		close(state.queue[0].ready)
		return
	}
	if len(state.records) == 0 {
		delete(l.windows, key)
	}
}
