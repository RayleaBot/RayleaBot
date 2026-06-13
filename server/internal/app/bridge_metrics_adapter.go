package app

import (
	"github.com/RayleaBot/RayleaBot/server/internal/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/metrics"
)

// dispatcherStatsAdapter projects dispatcher statistics into bridge observability.
type dispatcherStatsAdapter struct {
	dispatcher *dispatch.Dispatcher
}

func (a dispatcherStatsAdapter) Stats() bridge.DispatcherStatsView {
	if a.dispatcher == nil {
		return bridge.DispatcherStatsView{}
	}
	stats := a.dispatcher.Stats()
	return bridge.DispatcherStatsView{
		Delivered: stats.Delivered,
		Dropped:   stats.Dropped,
		Errored:   stats.Errored,
		Ignored:   stats.Ignored,
	}
}

// dispatcherRuntimePublisher publishes dispatch window snapshots through bridge fan-out.
type dispatcherRuntimePublisher struct {
	bridge *bridge.Bridge
}

func (p dispatcherRuntimePublisher) PublishDispatcherRuntime(snap dispatch.DispatcherWindowSnapshot) {
	if p.bridge == nil {
		return
	}
	rows := make([]bridge.DispatcherRuntimeDropRow, 0, len(snap.DropsByReason))
	for _, row := range snap.DropsByReason {
		rows = append(rows, bridge.DispatcherRuntimeDropRow{
			Reason:    row.Reason,
			PluginID:  row.PluginID,
			EventType: row.EventType,
			Count:     row.Count,
		})
	}
	p.bridge.PublishDispatcherRuntime(bridge.DispatcherRuntimeData{
		WindowSeconds:  snap.WindowSeconds,
		DeliveredCount: snap.Delivered,
		DroppedCount:   snap.Dropped,
		IgnoredCount:   snap.Ignored,
		DropsByReason:  rows,
	})
}

// bridgeMetricsAdapter routes bridge outcomes into the Prometheus registry.
type bridgeMetricsAdapter struct {
	registry *metrics.Registry
}

func (a bridgeMetricsAdapter) IncEventPipelineStage(stage, outcome string) {
	if a.registry == nil || a.registry.EventPipelineStage == nil {
		return
	}
	a.registry.EventPipelineStage.WithLabelValues(stage, outcome).Inc()
}

func (a bridgeMetricsAdapter) IncBridgeIgnored() {
	if a.registry == nil || a.registry.BridgeIgnoredTotal == nil {
		return
	}
	a.registry.BridgeIgnoredTotal.Inc()
}
