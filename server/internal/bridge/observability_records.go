package bridge

import (
	"time"

	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/adapter/intake"
)

func (b *Bridge) recordIgnored(event adapterintake.NormalizedEvent, observedAt time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.snapshot.IgnoredCount++
	b.snapshot.LastEventType = event.EventType
	b.snapshot.LastEventKind = event.Kind
	b.snapshot.LastOutcome = OutcomeIgnored
	b.snapshot.LastErrorCode = ""
	b.snapshot.LastErrorText = ""
	b.snapshot.LastEventAt = &observedAt
	if b.metrics != nil {
		b.metrics.IncEventPipelineStage("bridge", string(OutcomeIgnored))
		b.metrics.IncBridgeIgnored()
	}
}

func (b *Bridge) recordRejected(event adapterintake.NormalizedEvent, observedAt time.Time, code, message string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.snapshot.AcceptedCount++
	b.snapshot.RejectedCount++
	b.snapshot.LastEventType = event.EventType
	b.snapshot.LastEventKind = event.Kind
	b.snapshot.LastOutcome = OutcomeRejected
	b.snapshot.LastErrorCode = code
	b.snapshot.LastErrorText = message
	b.snapshot.LastEventAt = &observedAt
	if b.metrics != nil {
		b.metrics.IncEventPipelineStage("bridge", string(OutcomeRejected))
	}
}

func (b *Bridge) recordError(event adapterintake.NormalizedEvent, observedAt time.Time, code, message string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.snapshot.AcceptedCount++
	b.snapshot.ErrorCount++
	b.snapshot.LastEventType = event.EventType
	b.snapshot.LastEventKind = event.Kind
	b.snapshot.LastOutcome = OutcomeError
	b.snapshot.LastErrorCode = code
	b.snapshot.LastErrorText = message
	b.snapshot.LastEventAt = &observedAt
	if b.metrics != nil {
		b.metrics.IncEventPipelineStage("bridge", string(OutcomeError))
	}
	b.emitObservabilityLocked(observedAt, OutcomeError)
}

func (b *Bridge) recordDelivered(event adapterintake.NormalizedEvent, observedAt time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.snapshot.AcceptedCount++
	b.snapshot.DeliveredCount++
	b.snapshot.ResultCount++
	b.snapshot.LastEventType = event.EventType
	b.snapshot.LastEventKind = event.Kind
	b.snapshot.LastOutcome = OutcomeDelivered
	b.snapshot.LastErrorCode = ""
	b.snapshot.LastErrorText = ""
	b.snapshot.LastEventAt = &observedAt
	if b.metrics != nil {
		b.metrics.IncEventPipelineStage("bridge", string(OutcomeDelivered))
	}
	b.emitObservabilityLocked(observedAt, OutcomeDelivered)
}
