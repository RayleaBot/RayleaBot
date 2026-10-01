package desktop

import (
	"context"
	"sync"
)

// updateLifecycle owns the entire operation, so exit waits for helper reaping
// and final snapshot publication, not just for the cancellation request.
type updateLifecycle struct {
	mu     sync.Mutex
	closed bool
	cancel context.CancelFunc
	done   chan struct{}
}

func (u *updateLifecycle) begin() (context.Context, func(), bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.closed || u.done != nil {
		return nil, nil, false
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	u.cancel, u.done = cancel, done
	return ctx, func() {
		cancel()
		u.mu.Lock()
		defer u.mu.Unlock()
		u.cancel, u.done = nil, nil
		close(done)
	}, true
}

func (u *updateLifecycle) shutdown() {
	u.mu.Lock()
	u.closed = true
	cancel, done := u.cancel, u.done
	u.mu.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
}
