package bridge

import (
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
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
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, subscriber := range b.subscribers {
		emitObservabilityFrame(subscriber, frame)
	}
}

func (b *Bridge) emitObservabilityLocked(observedAt time.Time, outcome Outcome) {
	lastKind := b.snapshot.LastEventKind
	if lastKind == "" {
		lastKind = adapter.EventKindMessageText
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

	for _, subscriber := range b.subscribers {
		emitObservabilityFrame(subscriber, frame)
	}
}
