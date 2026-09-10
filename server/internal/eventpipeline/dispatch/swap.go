package dispatch

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

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
	case <-ctx.Done():
		return ctx.Err()
	}
}

// SwapPlugin publishes an already initialized target atomically. It does not
// start or stop processes; the caller owns initialization and retirement.
func (d *Dispatcher) SwapPlugin(pluginID string, target runtimeDeliverer, subscriptions []string, commands []plugins.Command, concurrency int) *Drain {
	d.mu.Lock()
	previous := d.slots[pluginID]
	next := d.newPluginSlot(target, subscriptions, commands, concurrency)
	d.slots[pluginID] = next
	go d.worker(pluginID, next)
	d.mu.Unlock()
	if previous == nil {
		return nil
	}
	previous.closeQueues()
	return &Drain{done: previous.done, cancel: previous.cancel}
}
