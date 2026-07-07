package app

import (
	"strconv"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
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

type AdapterObserver struct {
	registry *MetricsRegistry
}

func NewAdapterObserver(registry *MetricsRegistry) AdapterObserver {
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

// BridgeObserver routes bridge outcomes into the Prometheus registry.
type BridgeObserver struct {
	registry *MetricsRegistry
}

func NewBridgeObserver(registry *MetricsRegistry) BridgeObserver {
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

type DispatchObserver struct {
	registry *MetricsRegistry
}

func NewDispatchObserver(registry *MetricsRegistry) DispatchObserver {
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

type HTTPObserver struct {
	registry *MetricsRegistry
}

func NewHTTPObserver(registry *MetricsRegistry) HTTPObserver {
	return HTTPObserver{registry: registry}
}

func (o HTTPObserver) ObserveHTTPRequest(method, route string, status int, duration time.Duration) {
	if o.registry == nil {
		return
	}
	statusLabel := strconv.Itoa(status)
	if o.registry.HTTPRequestTotal != nil {
		o.registry.HTTPRequestTotal.WithLabelValues(method, route, statusLabel).Inc()
	}
	if o.registry.HTTPRequestDuration != nil {
		o.registry.HTTPRequestDuration.WithLabelValues(method, route, statusLabel).Observe(duration.Seconds())
	}
}

func (o HTTPObserver) ObserveHTTPPanic(method, route string) {
	if o.registry == nil || o.registry.HTTPPanicTotal == nil {
		return
	}
	o.registry.HTTPPanicTotal.WithLabelValues(method, route).Inc()
}

type RenderObserver struct {
	registry *MetricsRegistry
}

func NewRenderObserver(registry *MetricsRegistry) RenderObserver {
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

type TaskObserver struct {
	registry *MetricsRegistry
}

func NewTaskObserver(registry *MetricsRegistry) TaskObserver {
	return TaskObserver{registry: registry}
}

func (a TaskObserver) ObserveTaskExecution(taskType, outcome string, duration time.Duration) {
	if a.registry == nil || a.registry.TaskExecutionLatency == nil {
		return
	}
	a.registry.TaskExecutionLatency.WithLabelValues(taskType, outcome).Observe(duration.Seconds())
}

type WebhookReplayObserver struct {
	registry *MetricsRegistry
}

func NewWebhookReplayObserver(registry *MetricsRegistry) WebhookReplayObserver {
	return WebhookReplayObserver{registry: registry}
}

func (a WebhookReplayObserver) IncReplayObserved(outcome string) {
	if a.registry == nil || a.registry.WebhookReplayObserved == nil {
		return
	}
	a.registry.WebhookReplayObserved.WithLabelValues(outcome).Inc()
}

var pluginStates = []string{
	plugins.PluginStateDisabled,
	plugins.PluginStateEnabled,
	plugins.PluginStateStarting,
	plugins.PluginStateRunning,
	plugins.PluginStateStopping,
	plugins.PluginStateFailed,
	plugins.PluginStateInvalid,
}

func RefreshPluginStateGauge(registry *MetricsRegistry, catalog *plugincatalog.Catalog) {
	if registry == nil || registry.PluginState == nil || catalog == nil {
		return
	}
	counts := make(map[string]int, len(pluginStates))
	for _, state := range pluginStates {
		counts[state] = 0
	}
	for _, snapshot := range catalog.List() {
		state, _ := plugins.ProjectState(snapshot)
		if _, ok := counts[state]; !ok {
			counts[state] = 0
		}
		counts[state]++
	}
	for state, count := range counts {
		registry.PluginState.WithLabelValues(state).Set(float64(count))
	}
}

func StartPluginStateGaugeRefresh(registry *MetricsRegistry, catalog *plugincatalog.Catalog) (stop func()) {
	if registry == nil || catalog == nil {
		return func() {}
	}
	RefreshPluginStateGauge(registry, catalog)
	events, unsubscribe := catalog.Subscribe(16)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case _, ok := <-events:
				if !ok {
					return
				}
				RefreshPluginStateGauge(registry, catalog)
			case <-ticker.C:
				RefreshPluginStateGauge(registry, catalog)
			}
		}
	}()
	return func() {
		unsubscribe()
		<-done
	}
}
