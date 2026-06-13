package dispatch

func (d *Dispatcher) recordOutcome(outcome Outcome, pluginID, reason string) {
	if d == nil {
		return
	}
	d.statsMu.Lock()
	switch outcome {
	case OutcomeDelivered:
		d.delivered++
	case OutcomeDropped:
		d.dropped++
		if reason == "" {
			reason = "unknown"
		}
		if d.dropsByReason == nil {
			d.dropsByReason = make(map[string]map[string]uint64)
		}
		bucket, ok := d.dropsByReason[reason]
		if !ok {
			bucket = make(map[string]uint64)
			d.dropsByReason[reason] = bucket
		}
		bucket[pluginID]++
	case OutcomeError:
		d.errored++
	case OutcomeIgnored:
		d.ignored++
	}
	d.statsMu.Unlock()

	if observer := d.currentMetrics(); observer != nil {
		observer.IncEventPipelineStage("dispatch", string(outcome))
		if outcome == OutcomeDropped {
			normalisedReason := reason
			if normalisedReason == "" {
				normalisedReason = "unknown"
			}
			observer.IncDispatcherDrop(pluginID, normalisedReason)
		}
	}
}

// Stats returns a deep-copied snapshot of cumulative dispatcher outcome counts.
func (d *Dispatcher) Stats() DispatcherStats {
	if d == nil {
		return DispatcherStats{}
	}
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
