package dispatch

import (
	"context"
	"fmt"

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
	newManager *runtime.Manager,
	spec runtime.Spec,
	payload runtime.InitPayload,
	subs []string,
	cmds []CommandDecl,
) error {
	// Start the new process. This blocks until init_ack or failure.
	if err := newManager.Start(ctx, spec, payload); err != nil {
		return fmt.Errorf("new runtime init failed: %w", err)
	}

	// Atomically swap runtimes in the dispatcher.
	d.mu.Lock()
	oldSlot, hadOld := d.slots[pluginID]
	// Register new slot.
	newSlot := &pluginSlot{
		runtime:       newManager,
		subscriptions: append([]string(nil), subs...),
		commands:      append([]CommandDecl(nil), cmds...),
		queue:         make(chan dispatchItem, d.queueSize),
		done:          make(chan struct{}),
	}
	d.slots[pluginID] = newSlot
	go d.worker(pluginID, newSlot)
	d.mu.Unlock()

	// Stop old runtime in background (non-blocking for the caller).
	if hadOld && oldSlot != nil {
		close(oldSlot.queue)
		<-oldSlot.done
	}

	return nil
}
