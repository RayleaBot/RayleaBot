package app

import "github.com/RayleaBot/RayleaBot/server/internal/metrics"

// adapterMetricsAdapter routes adapter observations into the Prometheus registry.
type adapterMetricsAdapter struct {
	registry *metrics.Registry
}

func (a adapterMetricsAdapter) IncAdapterDedupDrop() {
	if a.registry == nil || a.registry.AdapterDedupDrops == nil {
		return
	}
	a.registry.AdapterDedupDrops.Inc()
}

func (a adapterMetricsAdapter) IncEventPipelineStage(stage, outcome string) {
	if a.registry == nil || a.registry.EventPipelineStage == nil {
		return
	}
	a.registry.EventPipelineStage.WithLabelValues(stage, outcome).Inc()
}
