package app

import (
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/metrics"
)

// dispatchMetricsAdapter routes dispatcher outcomes into the Prometheus registry.
type dispatchMetricsAdapter struct {
	registry *metrics.Registry
}

func (a dispatchMetricsAdapter) IncEventPipelineStage(stage, outcome string) {
	if a.registry == nil || a.registry.EventPipelineStage == nil {
		return
	}
	a.registry.EventPipelineStage.WithLabelValues(stage, outcome).Inc()
}

func (a dispatchMetricsAdapter) IncDispatcherDrop(pluginID, reason string) {
	if a.registry == nil || a.registry.DispatcherDropTotal == nil {
		return
	}
	a.registry.DispatcherDropTotal.WithLabelValues(pluginID, reason).Inc()
}

func (a dispatchMetricsAdapter) IncOutboundSend(adapterLabel, outcome string) {
	if a.registry == nil || a.registry.OutboundSendTotal == nil {
		return
	}
	a.registry.OutboundSendTotal.WithLabelValues(adapterLabel, outcome).Inc()
}

func (a dispatchMetricsAdapter) ObserveOutboundDuration(adapterLabel string, duration time.Duration) {
	if a.registry == nil || a.registry.OutboundSendDuration == nil {
		return
	}
	a.registry.OutboundSendDuration.WithLabelValues(adapterLabel).Observe(duration.Seconds())
}
