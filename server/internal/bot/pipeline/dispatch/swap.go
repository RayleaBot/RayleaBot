package dispatch

import (
	"context"
	"errors"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

var ErrClosed = errors.New("plugin dispatcher is closed")

// DrainPlugin ends admission while allowing accepted events to complete.
// The lifecycle owner stops the process and calls Deregister after waiting.
func (d *Dispatcher) DrainPlugin(pluginID string) *Drain {
	d.mu.RLock()
	defer d.mu.RUnlock()
	slot := d.slots[pluginID]
	if slot == nil {
		return nil
	}
	slot.closeQueues()
	return &Drain{done: slot.done, cancel: slot.cancel}
}

// DrainAll closes every admission queue before waiting on the shared budget.
// Process ownership remains with Runtime; callers stop it even after timeout.
func (d *Dispatcher) DrainAll(ctx context.Context) error {
	d.mu.Lock()
	d.closed = true
	drains := make([]*Drain, 0, len(d.slots))
	for _, slot := range d.slots {
		slot.closeQueues()
		drains = append(drains, &Drain{done: slot.done, cancel: slot.cancel})
	}
	for slot := range d.retired {
		slot.closeQueues()
		drains = append(drains, &Drain{done: slot.done, cancel: slot.cancel})
	}
	d.mu.Unlock()
	var failures []error
	for _, drain := range drains {
		if err := drain.Wait(ctx); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

// Drain owns the queue retired by a swap. The lifecycle owner waits for it
// before stopping the old process; cancellation releases queued deliveries.
type Drain struct {
	done   <-chan struct{}
	cancel context.CancelFunc
}

// Wait drains accepted events, or cancels them when the caller's budget expires.
func (d *Drain) Wait(ctx context.Context) error {
	defer d.cancel()
	select {
	case <-d.done:
		return nil
	default:
	}
	select {
	case <-d.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// SwapPlugin publishes an already initialized target atomically. It does not
// start or stop processes; the caller owns initialization and retirement.
func (d *Dispatcher) SwapPlugin(pluginID string, target runtimeDeliverer, subscriptions []string, commands []plugins.Command, concurrency int) (*Drain, error) {
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return nil, ErrClosed
	}
	previous := d.slots[pluginID]
	if previous != nil {
		d.retireSlotLocked(previous)
	}
	next := d.newPluginSlot(target, subscriptions, commands, concurrency)
	d.slots[pluginID] = next
	go d.worker(pluginID, next)
	d.mu.Unlock()
	if previous == nil {
		return nil, nil
	}
	previous.closeQueues()
	return &Drain{done: previous.done, cancel: previous.cancel}, nil
}
