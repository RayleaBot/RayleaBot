package app

import (
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/metrics"
)

// renderMetricsAdapter routes render service outcomes into the Prometheus registry.
type renderMetricsAdapter struct {
	registry *metrics.Registry
}

func (a renderMetricsAdapter) SetRenderQueueDepth(depth int) {
	if a.registry == nil || a.registry.RenderQueueDepth == nil {
		return
	}
	a.registry.RenderQueueDepth.Set(float64(depth))
}

func (a renderMetricsAdapter) ObserveRenderDuration(outcome string, duration time.Duration) {
	if a.registry == nil || a.registry.RenderDuration == nil {
		return
	}
	a.registry.RenderDuration.WithLabelValues(outcome).Observe(duration.Seconds())
}
