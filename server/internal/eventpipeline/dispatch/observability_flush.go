package dispatch

import "time"

// FlushDispatcherWindow computes the delta against the last flushed baseline
// and forwards it to the runtime publisher. Exposed primarily for tests; the
// flush goroutine started by StartObservabilityFlush calls it on a ticker.
func (d *Dispatcher) FlushDispatcherWindow(windowSeconds int) {
	if d == nil {
		return
	}
	d.flushMu.Lock()
	publisher := d.runtimePublisher
	baseline := d.flushBaseline
	d.flushMu.Unlock()
	if publisher == nil {
		return
	}

	current := d.Stats()
	snapshot := DispatcherWindowSnapshot{
		WindowSeconds: windowSeconds,
		Delivered:     deltaUint64(current.Delivered, baseline.Delivered),
		Dropped:       deltaUint64(current.Dropped, baseline.Dropped),
		Ignored:       deltaUint64(current.Ignored, baseline.Ignored),
		DropsByReason: diffDropsByReason(current.DropsByReason, baseline.DropsByReason),
	}

	d.flushMu.Lock()
	d.flushBaseline = current
	d.flushMu.Unlock()

	publisher.PublishDispatcherRuntime(snapshot)
}

// StartObservabilityFlush spawns a goroutine that periodically flushes window
// snapshots. The goroutine exits when Close is called. Calling more than once
// without an intervening Close is a no-op after the first call.
func (d *Dispatcher) StartObservabilityFlush(interval time.Duration) {
	if d == nil || interval <= 0 {
		return
	}
	windowSeconds := int(interval / time.Second)
	if windowSeconds <= 0 {
		windowSeconds = 1
	}
	d.flushMu.Lock()
	if d.flushStop != nil {
		d.flushMu.Unlock()
		return
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	d.flushStop = stop
	d.flushDone = done
	d.flushBaseline = d.snapshotStatsLocked()
	d.flushMu.Unlock()

	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				d.FlushDispatcherWindow(windowSeconds)
			}
		}
	}()
}
