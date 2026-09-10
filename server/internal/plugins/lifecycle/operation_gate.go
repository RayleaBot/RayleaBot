package lifecycle

import (
	"context"
	"sync"
)

// OperationGate serializes runtime and package mutations for one plugin while
// allowing unrelated plugins to proceed. A transaction's synchronous lifecycle
// callbacks reuse its ownership through an internal, expiring context token.
type OperationGate struct {
	mu      sync.Mutex
	entries map[string]*operationEntry
}

type operationEntry struct {
	gate       chan struct{}
	references int
}
type operationContextKey struct{}
type heldOperation struct {
	owner    *OperationGate
	pluginID string
	released <-chan struct{}
}

func NewOperationGate() *OperationGate {
	return &OperationGate{entries: make(map[string]*operationEntry)}
}

func (g *OperationGate) Acquire(ctx context.Context, pluginID string) (context.Context, func(), error) {
	if err := ctx.Err(); err != nil {
		return ctx, nil, err
	}
	if held, ok := ctx.Value(operationContextKey{}).(heldOperation); ok && held.owner == g && held.pluginID == pluginID {
		select {
		case <-held.released:
		default:
			return ctx, func() {}, nil
		}
	}
	g.mu.Lock()
	entry := g.entries[pluginID]
	if entry == nil {
		entry = &operationEntry{gate: make(chan struct{}, 1)}
		g.entries[pluginID] = entry
	}
	entry.references++
	g.mu.Unlock()
	forget := func() {
		g.mu.Lock()
		entry.references--
		if entry.references == 0 {
			delete(g.entries, pluginID)
		}
		g.mu.Unlock()
	}
	select {
	case entry.gate <- struct{}{}:
		if err := ctx.Err(); err != nil {
			<-entry.gate
			forget()
			return ctx, nil, err
		}
	case <-ctx.Done():
		forget()
		return ctx, nil, ctx.Err()
	}
	done := make(chan struct{})
	var once sync.Once
	release := func() { once.Do(func() { close(done); <-entry.gate; forget() }) }
	return context.WithValue(ctx, operationContextKey{}, heldOperation{owner: g, pluginID: pluginID, released: done}), release, nil
}
