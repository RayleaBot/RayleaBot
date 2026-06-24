package bridge

func (b *Bridge) SetAdapterStatsSource(source AdapterDedupStats) {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.adapterStats = source
}

func (b *Bridge) SetDispatcherStatsSource(source DispatcherStatsSnapshot) {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.dispatcherStats = source
}

func (b *Bridge) SetMetricsObserver(observer MetricsObserver) {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.metrics = observer
}
