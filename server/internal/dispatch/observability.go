package dispatch

// SetRuntimePublisher wires the runtime publisher the dispatcher hands window
// snapshots to. Calling with nil disables publication.
func (d *Dispatcher) SetRuntimePublisher(publisher DispatcherRuntimePublisher) {
	if d == nil {
		return
	}
	d.flushMu.Lock()
	defer d.flushMu.Unlock()
	d.runtimePublisher = publisher
}

// SetMetricsObserver wires the Prometheus observer the dispatcher uses to
// record drop and pipeline counters. Passing nil disables instrumentation.
func (d *Dispatcher) SetMetricsObserver(observer MetricsObserver) {
	if d == nil {
		return
	}
	d.flushMu.Lock()
	defer d.flushMu.Unlock()
	d.metrics = observer
}
func (d *Dispatcher) currentMetrics() MetricsObserver {
	if d == nil {
		return nil
	}
	d.flushMu.Lock()
	defer d.flushMu.Unlock()
	return d.metrics
}
