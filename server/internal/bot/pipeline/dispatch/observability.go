package dispatch

import "time"

// DispatcherDropRow captures the per-window, per-reason drop count for one
// plugin. Plugin id and event type are populated when known; the reason is
// always set.
type DispatcherDropRow struct {
	Reason    string
	PluginID  string
	EventType string
	Count     uint64
}

// DispatcherWindowSnapshot is the delta carried by a single dispatcher_runtime
// observability frame. Counts are window-local and reset every flush.
type DispatcherWindowSnapshot struct {
	WindowSeconds int
	Delivered     uint64
	Dropped       uint64
	Ignored       uint64
	DropsByReason []DispatcherDropRow
}

// DispatcherRuntimePublisher receives window snapshots so the bridge (or a
// test double) can fan them out to management WebSocket subscribers as the
// formal dispatcher_runtime observability event.
type DispatcherRuntimePublisher interface {
	PublishDispatcherRuntime(snapshot DispatcherWindowSnapshot)
}

// SetRuntimePublisher wires the runtime publisher the dispatcher hands window
// snapshots to. Calling with nil disables publication.
func (d *Dispatcher) SetRuntimePublisher(publisher DispatcherRuntimePublisher) {
	d.flushMu.Lock()
	defer d.flushMu.Unlock()
	d.runtimePublisher = publisher
}

// SetMetricsObserver wires the Prometheus observer the dispatcher uses to
// record drop and pipeline counters. Passing nil disables instrumentation.
func (d *Dispatcher) SetMetricsObserver(observer MetricsObserver) {
	d.flushMu.Lock()
	defer d.flushMu.Unlock()
	d.metrics = observer
}
func (d *Dispatcher) currentMetrics() MetricsObserver {
	d.flushMu.Lock()
	defer d.flushMu.Unlock()
	return d.metrics
}

func (d *Dispatcher) snapshotStatsLocked() DispatcherStats {
	d.statsMu.Lock()
	defer d.statsMu.Unlock()
	cloned := make(map[string]map[string]uint64, len(d.dropsByReason))
	for reason, plugins := range d.dropsByReason {
		row := make(map[string]uint64, len(plugins))
		for pluginID, count := range plugins {
			row[pluginID] = count
		}
		cloned[reason] = row
	}
	return DispatcherStats{
		Delivered:     d.delivered,
		Dropped:       d.dropped,
		Errored:       d.errored,
		Ignored:       d.ignored,
		DropsByReason: cloned,
	}
}

func deltaUint64(current, baseline uint64) uint64 {
	if current < baseline {
		return 0
	}
	return current - baseline
}

func diffDropsByReason(current, baseline map[string]map[string]uint64) []DispatcherDropRow {
	var rows []DispatcherDropRow
	for reason, plugins := range current {
		base := baseline[reason]
		for pluginID, count := range plugins {
			delta := count
			if prev, ok := base[pluginID]; ok && prev <= count {
				delta = count - prev
			}
			if delta == 0 {
				continue
			}
			rows = append(rows, DispatcherDropRow{
				Reason:   reason,
				PluginID: pluginID,
				Count:    delta,
			})
		}
	}
	return rows
}

// FlushDispatcherWindow computes the delta against the last flushed baseline
// and forwards it to the runtime publisher. Exposed primarily for tests; the
// flush goroutine started by StartObservabilityFlush calls it on a ticker.
func (d *Dispatcher) FlushDispatcherWindow(windowSeconds int) {
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
	if interval <= 0 {
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
