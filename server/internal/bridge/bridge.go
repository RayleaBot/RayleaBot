package bridge

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"rayleabot/server/internal/adapter"
	"rayleabot/server/internal/runtime"
)

const (
	codePlatformInvalidRequest = "platform.invalid_request"
	codePluginStopping         = "plugin.stopping"
)

type Outcome string

const (
	OutcomeIgnored   Outcome = "ignored"
	OutcomeDelivered Outcome = "delivered"
	OutcomeError     Outcome = "error"
	OutcomeRejected  Outcome = "rejected"
)

type Snapshot struct {
	AcceptedCount  uint64
	DeliveredCount uint64
	ResultCount    uint64
	ErrorCount     uint64
	IgnoredCount   uint64
	RejectedCount  uint64
	LastEventType  string
	LastOutcome    Outcome
	LastErrorCode  string
	LastErrorText  string
	LastEventAt    *time.Time
}

type runtimeClient interface {
	Snapshot() runtime.Snapshot
	DeliverEvent(context.Context, runtime.Event) (runtime.Delivery, error)
}

type Bridge struct {
	logger  *slog.Logger
	runtime runtimeClient

	mu       sync.RWMutex
	snapshot Snapshot
}

func New(logger *slog.Logger, runtimeClient runtimeClient) *Bridge {
	if logger == nil {
		logger = slog.Default()
	}

	return &Bridge{
		logger:  logger,
		runtime: runtimeClient,
	}
}

func (b *Bridge) Snapshot() Snapshot {
	b.mu.RLock()
	defer b.mu.RUnlock()

	cloned := b.snapshot
	if b.snapshot.LastEventAt != nil {
		lastEventAt := *b.snapshot.LastEventAt
		cloned.LastEventAt = &lastEventAt
	}
	return cloned
}

func (b *Bridge) HandleAdapterEvent(ctx context.Context, event adapter.NormalizedEvent) Outcome {
	now := time.Now().UTC()

	if !isSupportedEvent(event) {
		b.recordIgnored(event, now)
		b.logger.Debug(
			"runtime bridge ignored adapter event",
			"component", "bridge",
			"event_kind", event.Kind,
			"event_type", event.EventType,
		)
		return OutcomeIgnored
	}

	if b.runtime == nil || b.runtime.Snapshot().State != runtime.StateRunning {
		b.recordRejected(event, now, codePlatformInvalidRequest, "runtime is not running")
		b.logger.Warn(
			"runtime bridge rejected adapter event",
			"component", "bridge",
			"event_kind", event.Kind,
			"event_type", event.EventType,
			"error_code", codePlatformInvalidRequest,
		)
		return OutcomeRejected
	}

	delivery, err := b.runtime.DeliverEvent(ctx, runtime.Event{
		EventID:        event.EventID,
		SourceProtocol: event.SourceProtocol,
		SourceAdapter:  event.SourceAdapter,
		EventType:      event.EventType,
		Timestamp:      event.Timestamp,
		Actor: &runtime.EventActor{
			ID: event.SenderID,
		},
		Target: &runtime.EventTarget{
			Type: event.ConversationType,
			ID:   event.ConversationID,
		},
		Message: &runtime.EventMessage{
			PlainText: event.PlainText,
		},
	})
	if err != nil {
		var runtimeErr *runtime.Error
		if errors.As(err, &runtimeErr) {
			if runtimeErr.Code == codePlatformInvalidRequest || runtimeErr.Code == codePluginStopping {
				b.recordRejected(event, now, runtimeErr.Code, runtimeErr.Message)
				b.logger.Warn(
					"runtime bridge rejected adapter event",
					"component", "bridge",
					"event_kind", event.Kind,
					"event_type", event.EventType,
					"error_code", runtimeErr.Code,
				)
				return OutcomeRejected
			}

			b.recordError(event, now, delivery.ErrorCode, delivery.ErrorMessage)
			b.logger.Warn(
				"runtime bridge received plugin error",
				"component", "bridge",
				"event_kind", event.Kind,
				"event_type", event.EventType,
				"error_code", runtimeErr.Code,
			)
			return OutcomeError
		}

		b.recordError(event, now, "plugin.internal_error", err.Error())
		b.logger.Warn(
			"runtime bridge failed during event delivery",
			"component", "bridge",
			"event_kind", event.Kind,
			"event_type", event.EventType,
			"error_code", "plugin.internal_error",
		)
		return OutcomeError
	}

	b.recordDelivered(event, now)
	b.logger.Info(
		"runtime bridge delivered adapter event",
		"component", "bridge",
		"event_kind", event.Kind,
		"event_type", event.EventType,
		"request_id", delivery.RequestID,
	)
	return OutcomeDelivered
}

func isSupportedEvent(event adapter.NormalizedEvent) bool {
	return event.Kind == adapter.EventKindMessageText &&
		event.EventID != "" &&
		event.SourceProtocol == "onebot11" &&
		event.SourceAdapter == "adapter.onebot11" &&
		isSupportedEventType(event) &&
		event.Timestamp > 0 &&
		event.ConversationType != "" &&
		event.ConversationID != "" &&
		event.SenderID != "" &&
		event.PlainText != ""
}

func isSupportedEventType(event adapter.NormalizedEvent) bool {
	switch event.EventType {
	case "message.group":
		return event.ConversationType == "group"
	case "message.private":
		return event.ConversationType == "private"
	default:
		return false
	}
}

func (b *Bridge) recordIgnored(event adapter.NormalizedEvent, observedAt time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.snapshot.IgnoredCount++
	b.snapshot.LastEventType = event.EventType
	b.snapshot.LastOutcome = OutcomeIgnored
	b.snapshot.LastEventAt = &observedAt
}

func (b *Bridge) recordRejected(event adapter.NormalizedEvent, observedAt time.Time, code, message string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.snapshot.AcceptedCount++
	b.snapshot.RejectedCount++
	b.snapshot.LastEventType = event.EventType
	b.snapshot.LastOutcome = OutcomeRejected
	b.snapshot.LastErrorCode = code
	b.snapshot.LastErrorText = message
	b.snapshot.LastEventAt = &observedAt
}

func (b *Bridge) recordError(event adapter.NormalizedEvent, observedAt time.Time, code, message string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.snapshot.AcceptedCount++
	b.snapshot.ErrorCount++
	b.snapshot.LastEventType = event.EventType
	b.snapshot.LastOutcome = OutcomeError
	b.snapshot.LastErrorCode = code
	b.snapshot.LastErrorText = message
	b.snapshot.LastEventAt = &observedAt
}

func (b *Bridge) recordDelivered(event adapter.NormalizedEvent, observedAt time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.snapshot.AcceptedCount++
	b.snapshot.DeliveredCount++
	b.snapshot.ResultCount++
	b.snapshot.LastEventType = event.EventType
	b.snapshot.LastOutcome = OutcomeDelivered
	b.snapshot.LastErrorCode = ""
	b.snapshot.LastErrorText = ""
	b.snapshot.LastEventAt = &observedAt
}
