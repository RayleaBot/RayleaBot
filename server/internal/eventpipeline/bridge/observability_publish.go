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
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, subscriber := range b.subscribers {
		emitObservabilityFrame(subscriber, frame)
	}
}

func (b *Bridge) SubscribeObservability(buffer int) (<-chan ObservabilityFrame, func()) {
	if buffer <= 0 {
		buffer = 1
	}

	ch := make(chan ObservabilityFrame, buffer)

	b.mu.Lock()
	id := b.nextSubscriberID
	b.nextSubscriberID++
	b.subscribers[id] = ch
	b.mu.Unlock()

	return ch, func() {
		b.mu.Lock()
		defer b.mu.Unlock()

		subscriber, ok := b.subscribers[id]
		if !ok {
			return
		}

		delete(b.subscribers, id)
		close(subscriber)
	}
}

func (b *Bridge) ObservabilitySubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return len(b.subscribers)
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

func emitObservabilityFrame(subscriber chan ObservabilityFrame, frame ObservabilityFrame) {
	select {
	case subscriber <- frame:
	default:
		select {
		case <-subscriber:
		default:
		}
		select {
		case subscriber <- frame:
		default:
		}
	}
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

	for _, subscriber := range b.subscribers {
		emitObservabilityFrame(subscriber, frame)
	}
}
