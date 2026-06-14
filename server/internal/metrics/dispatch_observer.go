package metrics

import (
	"time"
)

type DispatchObserver struct {
	registry *Registry
}

func NewDispatchObserver(registry *Registry) DispatchObserver {
	return DispatchObserver{registry: registry}
}

func (a DispatchObserver) IncEventPipelineStage(stage, outcome string) {
	if a.registry == nil || a.registry.EventPipelineStage == nil {
		return
	}
	a.registry.EventPipelineStage.WithLabelValues(stage, outcome).Inc()
}

func (a DispatchObserver) IncDispatcherDrop(pluginID, reason string) {
	if a.registry == nil || a.registry.DispatcherDropTotal == nil {
		return
	}
	a.registry.DispatcherDropTotal.WithLabelValues(pluginID, reason).Inc()
}

func (a DispatchObserver) IncOutboundSend(adapterLabel, outcome string) {
	if a.registry == nil || a.registry.OutboundSendTotal == nil {
		return
	}
	a.registry.OutboundSendTotal.WithLabelValues(adapterLabel, outcome).Inc()
}

func (a DispatchObserver) ObserveOutboundDuration(adapterLabel string, duration time.Duration) {
	if a.registry == nil || a.registry.OutboundSendDuration == nil {
		return
	}
	a.registry.OutboundSendDuration.WithLabelValues(adapterLabel).Observe(duration.Seconds())
}
