package metrics

import (
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/dispatch"
)

// DispatcherStatsAdapter projects dispatcher statistics into bridge observability.
type DispatcherStatsAdapter struct {
	dispatcher *dispatch.Dispatcher
}

func NewDispatcherStatsAdapter(dispatcher *dispatch.Dispatcher) DispatcherStatsAdapter {
	return DispatcherStatsAdapter{dispatcher: dispatcher}
}

func (a DispatcherStatsAdapter) Stats() bridge.DispatcherStatsView {
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

// DispatcherRuntimePublisher publishes dispatch window snapshots through bridge fan-out.
type DispatcherRuntimePublisher struct {
	bridge *bridge.Bridge
}

func NewDispatcherRuntimePublisher(eventBridge *bridge.Bridge) DispatcherRuntimePublisher {
	return DispatcherRuntimePublisher{bridge: eventBridge}
}

func (p DispatcherRuntimePublisher) PublishDispatcherRuntime(snap dispatch.DispatcherWindowSnapshot) {
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

// BridgeObserver routes bridge outcomes into the Prometheus registry.
type BridgeObserver struct {
	registry *Registry
}

func NewBridgeObserver(registry *Registry) BridgeObserver {
	return BridgeObserver{registry: registry}
}

func (a BridgeObserver) IncEventPipelineStage(stage, outcome string) {
	if a.registry == nil || a.registry.EventPipelineStage == nil {
		return
	}
	a.registry.EventPipelineStage.WithLabelValues(stage, outcome).Inc()
}

func (a BridgeObserver) IncBridgeIgnored() {
	if a.registry == nil || a.registry.BridgeIgnoredTotal == nil {
		return
	}
	a.registry.BridgeIgnoredTotal.Inc()
}
