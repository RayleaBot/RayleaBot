package metrics

type AdapterObserver struct {
	registry *Registry
}

func NewAdapterObserver(registry *Registry) AdapterObserver {
	return AdapterObserver{registry: registry}
}

func (a AdapterObserver) IncAdapterDedupDrop() {
	if a.registry == nil || a.registry.AdapterDedupDrops == nil {
		return
	}
	a.registry.AdapterDedupDrops.Inc()
}

func (a AdapterObserver) IncEventPipelineStage(stage, outcome string) {
	if a.registry == nil || a.registry.EventPipelineStage == nil {
		return
	}
	a.registry.EventPipelineStage.WithLabelValues(stage, outcome).Inc()
}
