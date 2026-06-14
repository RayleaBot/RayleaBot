package metrics

import (
	"time"
)

type RenderObserver struct {
	registry *Registry
}

func NewRenderObserver(registry *Registry) RenderObserver {
	return RenderObserver{registry: registry}
}

func (a RenderObserver) SetRenderQueueDepth(depth int) {
	if a.registry == nil || a.registry.RenderQueueDepth == nil {
		return
	}
	a.registry.RenderQueueDepth.Set(float64(depth))
}

func (a RenderObserver) ObserveRenderDuration(outcome string, duration time.Duration) {
	if a.registry == nil || a.registry.RenderDuration == nil {
		return
	}
	a.registry.RenderDuration.WithLabelValues(outcome).Observe(duration.Seconds())
}
