package desktop

import (
	"context"
	"sync"
)

// startupGate owns startup admission and cancellation. Blocking precedes the
// operation lock so stop and shutdown can interrupt a start already in flight.
type startupGate struct {
	mu              sync.Mutex
	nextID          uint64
	cancels         map[uint64]context.CancelFunc
	blockers        int
	shutdownStarted bool
}

func (g *startupGate) begin() (context.Context, func(), bool) {
	g.mu.Lock()
	if g.shutdownStarted || g.blockers > 0 {
		g.mu.Unlock()
		return nil, nil, false
	}
	startupContext, cancel := context.WithCancel(context.Background())
	g.nextID++
	startupID := g.nextID
	if g.cancels == nil {
		g.cancels = make(map[uint64]context.CancelFunc)
	}
	g.cancels[startupID] = cancel
	g.mu.Unlock()
	var finishOnce sync.Once
	return startupContext, func() {
		finishOnce.Do(func() {
			g.mu.Lock()
			delete(g.cancels, startupID)
			g.mu.Unlock()
			cancel()
		})
	}, true
}

func (g *startupGate) block(permanent bool) func() {
	unblock, _ := g.blockWithState(permanent)
	return unblock
}

func (g *startupGate) blockWithState(permanent bool) (func(), bool) {
	g.mu.Lock()
	if permanent {
		g.shutdownStarted = true
	} else {
		g.blockers++
	}
	cancels := make([]context.CancelFunc, 0, len(g.cancels))
	for _, cancel := range g.cancels {
		cancels = append(cancels, cancel)
	}
	hadActiveStartup := len(cancels) > 0
	g.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
	if permanent {
		return func() {}, hadActiveStartup
	}
	var unblockOnce sync.Once
	return func() {
		unblockOnce.Do(func() {
			g.mu.Lock()
			if g.blockers > 0 {
				g.blockers--
			}
			g.mu.Unlock()
		})
	}, hadActiveStartup
}
