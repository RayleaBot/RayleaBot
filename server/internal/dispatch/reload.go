package dispatch

import (
	"context"
	"fmt"
	"time"

	"rayleabot/server/internal/runtime"
)

// ReloadPlugin performs a zero-gap reload by starting a new runtime before
// stopping the old one. The sequence is:
//  1. Create and start the new runtime manager.
//  2. Wait for successful init_ack from the new process.
//  3. Atomically swap: deregister old runtime, register new runtime.
//  4. Stop the old runtime.
//
// If the new runtime fails to start, the old runtime remains active.
func (d *Dispatcher) ReloadPlugin(
	ctx context.Context,
	pluginID string,
	oldManager *runtime.Manager,
	newManager *runtime.Manager,
	spec runtime.Spec,
	payload runtime.InitPayload,
	cmds []CommandDecl,
) error {
	// Start the new process. This blocks until init_ack or failure.
	if err := newManager.Start(ctx, spec, payload); err != nil {
		return fmt.Errorf("new runtime init failed: %w", err)
	}
	subscriptions := newManager.Snapshot().Subscriptions

	// Atomically swap runtimes in the dispatcher.
	d.mu.Lock()
	oldSlot, hadOld := d.slots[pluginID]
	// Register new slot.
	newSlot := &pluginSlot{
		runtime:       newManager,
		subscriptions: append([]string(nil), subscriptions...),
		commands:      append([]CommandDecl(nil), cmds...),
		queue:         make(chan dispatchItem, d.queueSize),
		done:          make(chan struct{}),
	}
	d.slots[pluginID] = newSlot
	go d.worker(pluginID, newSlot)
	d.mu.Unlock()

	// Stop old runtime in background (non-blocking for the caller).
	if hadOld && oldSlot != nil {
		go func(slot *pluginSlot, manager *runtime.Manager) {
			close(slot.queue)
			<-slot.done
			if manager == nil {
				return
			}
			stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := manager.Stop(stopCtx); err != nil {
				d.logger.Warn(
					"reload left old runtime running after swap",
					"component", "dispatch",
					"plugin_id", pluginID,
					"err", err.Error(),
				)
			}
		}(oldSlot, oldManager)
	}

	return nil
}
