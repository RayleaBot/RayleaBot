package dispatch

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
