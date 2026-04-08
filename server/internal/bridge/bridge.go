package bridge

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

const (
	codePlatformInvalidRequest = "platform.invalid_request"
	codePluginStopping         = "plugin.stopping"
	eventsChannel              = "events"
	eventsTypeReceived         = "events.received"
	observabilityScopeBridge   = "bridge_runtime"
	summaryBridgeRuntime       = "bridge delivered recent adapter events while keeping bridge/runtime observability aggregate-only"
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
	LastEventKind  string
	LastOutcome    Outcome
	LastErrorCode  string
	LastErrorText  string
	LastEventAt    *time.Time
}

type ObservabilityFrame struct {
	Channel   string            `json:"channel"`
	Type      string            `json:"type"`
	Timestamp string            `json:"timestamp"`
	Data      ObservabilityData `json:"data"`
}

type ObservabilityData struct {
	ObservabilityScope  string  `json:"observability_scope"`
	Summary             string  `json:"summary"`
	LastSupportedKind   string  `json:"last_supported_event_kind,omitempty"`
	LastDeliveryOutcome Outcome `json:"last_delivery_outcome,omitempty"`
	DeliveredCount      uint64  `json:"delivered_count"`
	ResultCount         uint64  `json:"result_count"`
	ErrorCount          uint64  `json:"error_count"`
}

type runtimeClient interface {
	Snapshot() runtime.Snapshot
	DeliverEvent(context.Context, runtime.Event) (runtime.Delivery, error)
}

type Bridge struct {
	logger   *slog.Logger
	runtime  runtimeClient
	sender   outbound.ActionSender
	resolver outbound.ReplyTargetResolver

	mu               sync.RWMutex
	snapshot         Snapshot
	nextSubscriberID uint64
	subscribers      map[uint64]chan ObservabilityFrame
}

func New(logger *slog.Logger, runtimeClient runtimeClient, sender outbound.ActionSender, resolver outbound.ReplyTargetResolver) *Bridge {
	if logger == nil {
		logger = slog.Default()
	}

	return &Bridge{
		logger:      logger,
		runtime:     runtimeClient,
		sender:      sender,
		resolver:    resolver,
		subscribers: make(map[uint64]chan ObservabilityFrame),
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

	runtimeEvent := runtime.Event{
		EventID:        event.EventID,
		SourceProtocol: event.SourceProtocol,
		SourceAdapter:  event.SourceAdapter,
		EventType:      event.EventType,
		Timestamp:      event.Timestamp,
		Actor: &runtime.EventActor{
			ID:       event.SenderID,
			Nickname: event.ActorNickname,
			Role:     event.ActorRole,
		},
		Target: &runtime.EventTarget{
			Type: event.ConversationType,
			ID:   event.ConversationID,
			Name: event.TargetName,
		},
		PayloadFields: event.PayloadFields,
		MessageID:     event.MessageID,
	}
	if event.PlainText != "" {
		var segments []runtime.EventSegment
		for _, seg := range event.Segments {
			segments = append(segments, runtime.EventSegment{
				Type: seg.Type,
				Data: seg.Data,
			})
		}
		runtimeEvent.Message = &runtime.EventMessage{
			PlainText: event.PlainText,
			Segments:  segments,
		}
	}

	delivery, err := b.runtime.DeliverEvent(ctx, runtimeEvent)
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

			errorCode := delivery.ErrorCode
			errorMessage := delivery.ErrorMessage
			if errorCode == "" {
				errorCode = runtimeErr.Code
			}
			if errorMessage == "" {
				errorMessage = runtimeErr.Message
			}

			b.recordError(event, now, errorCode, errorMessage)
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

	if delivery.Action != nil {
		result, sendErr := b.sendOutboundAction(ctx, runtimeEvent, *delivery.Action)
		if sendErr != nil {
			code := "adapter.send_failed"
			message := sendErr.Error()
			var adapterErr *adapter.Error
			if errors.As(sendErr, &adapterErr) {
				code = adapterErr.Code
				message = adapterErr.Message
			}

			b.recordError(event, now, code, message)
			b.logger.Warn(
				"runtime bridge failed to execute outbound adapter action",
				"component", "bridge",
				"event_kind", event.Kind,
				"event_type", event.EventType,
				"error_code", code,
			)
			return OutcomeError
		}

		b.recordDelivered(event, now)
		b.logger.Info(
			"runtime bridge executed outbound adapter action",
			"component", "bridge",
			"event_kind", event.Kind,
			"event_type", event.EventType,
			"request_id", delivery.RequestID,
			"message_id", result.MessageID,
		)
		return OutcomeDelivered
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
	if event.EventID == "" || event.SourceProtocol != "onebot11" || event.SourceAdapter != "adapter.onebot11" {
		return false
	}
	if event.Timestamp <= 0 || event.ConversationType == "" || event.ConversationID == "" || event.SenderID == "" {
		return false
	}
	if !isSupportedEventKind(event.Kind) {
		return false
	}
	if !isSupportedEventType(event) {
		return false
	}
	// Message events require non-empty PlainText; notice events do not.
	if isMessageEventKind(event.Kind) && event.PlainText == "" {
		return false
	}
	return true
}

func isSupportedEventKind(kind string) bool {
	switch kind {
	case adapter.EventKindMessageText, adapter.EventKindMessage, adapter.EventKindNotice:
		return true
	default:
		return false
	}
}

func isMessageEventKind(kind string) bool {
	return kind == adapter.EventKindMessageText || kind == adapter.EventKindMessage
}

func isSupportedEventType(event adapter.NormalizedEvent) bool {
	switch event.EventType {
	case "message.group":
		return event.ConversationType == "group"
	case "message.private":
		return event.ConversationType == "private"
	case "notice.member_increase", "notice.member_decrease":
		return event.ConversationType == "group"
	default:
		return false
	}
}

func (b *Bridge) sendOutboundAction(ctx context.Context, origin runtime.Event, action runtime.Action) (adapter.SendMessageResult, error) {
	return outbound.SendAction(ctx, b.sender, b.resolver, origin, action)
}

func (b *Bridge) recordIgnored(event adapter.NormalizedEvent, observedAt time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.snapshot.IgnoredCount++
	b.snapshot.LastEventType = event.EventType
	b.snapshot.LastEventKind = event.Kind
	b.snapshot.LastOutcome = OutcomeIgnored
	b.snapshot.LastEventAt = &observedAt
}

func (b *Bridge) recordRejected(event adapter.NormalizedEvent, observedAt time.Time, code, message string) {
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
}

func (b *Bridge) recordError(event adapter.NormalizedEvent, observedAt time.Time, code, message string) {
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
	b.emitObservabilityLocked(observedAt, OutcomeError)
}

func (b *Bridge) recordDelivered(event adapter.NormalizedEvent, observedAt time.Time) {
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
	b.emitObservabilityLocked(observedAt, OutcomeDelivered)
}

func (b *Bridge) emitObservabilityLocked(observedAt time.Time, outcome Outcome) {
	lastKind := b.snapshot.LastEventKind
	if lastKind == "" {
		lastKind = adapter.EventKindMessageText
	}
	frame := ObservabilityFrame{
		Channel:   eventsChannel,
		Type:      eventsTypeReceived,
		Timestamp: observedAt.UTC().Format(time.RFC3339),
		Data: ObservabilityData{
			ObservabilityScope:  observabilityScopeBridge,
			Summary:             summaryBridgeRuntime,
			LastSupportedKind:   lastKind,
			LastDeliveryOutcome: outcome,
			DeliveredCount:      b.snapshot.DeliveredCount,
			ResultCount:         b.snapshot.ResultCount,
			ErrorCount:          b.snapshot.ErrorCount,
		},
	}

	for _, subscriber := range b.subscribers {
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
}
