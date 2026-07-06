package bridge

import (
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/onebot11"
)

func (b *Bridge) PublishDispatcherRuntime(data DispatcherRuntimeData) {
	if b == nil {
		return
	}
	if strings.TrimSpace(data.ObservabilityScope) == "" {
		data.ObservabilityScope = observabilityScopeDispatcher
	}
	frame := ObservabilityFrame{
		Channel:   eventsChannel,
		Type:      eventsTypeReceived,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data:      data,
	}
	b.hub.PublishReplace(frame)
}

func (b *Bridge) SubscribeObservability(buffer int) (<-chan ObservabilityFrame, func()) {
	return b.hub.Subscribe(buffer)
}

func (b *Bridge) ObservabilitySubscriberCount() int {
	return b.hub.SubscriberCount()
}

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

func (b *Bridge) emitObservabilityLocked(observedAt time.Time, outcome Outcome) {
	lastKind := b.snapshot.LastEventKind
	if lastKind == "" {
		lastKind = onebot11.EventKindMessageText
	}
	data := ObservabilityData{
		ObservabilityScope:  observabilityScopeBridge,
		Summary:             summaryBridgeRuntime,
		LastSupportedKind:   lastKind,
		LastDeliveryOutcome: outcome,
		DeliveredCount:      b.snapshot.DeliveredCount,
		ResultCount:         b.snapshot.ResultCount,
		ErrorCount:          b.snapshot.ErrorCount,
		BridgeIgnoredTotal:  b.snapshot.IgnoredCount,
	}
	if b.adapterStats != nil {
		data.AdapterDedupDropsTotal = b.adapterStats.DedupDropsSnapshot()
	}
	if b.dispatcherStats != nil {
		stats := b.dispatcherStats.Stats()
		data.DispatcherDelivered = stats.Delivered
		data.DispatcherDropped = stats.Dropped
		data.DispatcherIgnored = stats.Ignored
	}
	frame := ObservabilityFrame{
		Channel:   eventsChannel,
		Type:      eventsTypeReceived,
		Timestamp: observedAt.UTC().Format(time.RFC3339),
		Data:      data,
	}

	b.hub.PublishReplace(frame)
}
