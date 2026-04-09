package bridge

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
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
		attrs := append([]any{"component", "bridge"}, bridgeEventLogAttrs(event)...)
		b.logger.Debug(bridgeEventSummary("ignored", event), attrs...)
		return OutcomeIgnored
	}

	if b.runtime == nil || b.runtime.Snapshot().State != runtime.StateRunning {
		b.recordRejected(event, now, codePlatformInvalidRequest, "runtime is not running")
		attrs := append([]any{"component", "bridge"}, bridgeEventLogAttrs(event)...)
		attrs = append(attrs, "error_code", codePlatformInvalidRequest)
		b.logger.Warn(bridgeEventSummary("rejected", event), attrs...)
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
				attrs := append([]any{"component", "bridge"}, bridgeEventLogAttrs(event)...)
				attrs = append(attrs, "error_code", runtimeErr.Code)
				b.logger.Warn(bridgeEventSummary("rejected", event), attrs...)
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
			attrs := append([]any{"component", "bridge"}, bridgeEventLogAttrs(event)...)
			attrs = append(attrs, "error_code", runtimeErr.Code)
			b.logger.Warn(bridgeEventSummary("received plugin error for", event), attrs...)
			return OutcomeError
		}

		b.recordError(event, now, "plugin.internal_error", err.Error())
		attrs := append([]any{"component", "bridge"}, bridgeEventLogAttrs(event)...)
		attrs = append(attrs, "error_code", "plugin.internal_error")
		b.logger.Warn(bridgeEventSummary("failed during delivery for", event), attrs...)
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
			attrs := append([]any{"component", "bridge"}, bridgeEventLogAttrs(event)...)
			attrs = append(attrs, bridgeActionLogAttrs(*delivery.Action)...)
			attrs = append(attrs, "error_code", code)
			b.logger.Warn(bridgeEventSummary("failed outbound action for", event), attrs...)
			return OutcomeError
		}

		b.recordDelivered(event, now)
		attrs := append([]any{"component", "bridge"}, bridgeEventLogAttrs(event)...)
		attrs = append(attrs, bridgeActionLogAttrs(*delivery.Action)...)
		attrs = append(attrs,
			"request_id", delivery.RequestID,
			"outbound_message_id", result.MessageID,
		)
		b.logger.Info(bridgeEventSummary("executed outbound action for", event), attrs...)
		return OutcomeDelivered
	}

	b.recordDelivered(event, now)
	attrs := append([]any{"component", "bridge"}, bridgeEventLogAttrs(event)...)
	attrs = append(attrs, "request_id", delivery.RequestID)
	b.logger.Info(bridgeEventSummary("delivered", event), attrs...)
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
	case adapter.EventKindMessageText, adapter.EventKindMessage, adapter.EventKindNotice, adapter.EventKindRequest:
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
	case "notice.member_increase",
		"notice.member_decrease",
		"notice.group_admin",
		"notice.group_ban",
		"notice.group_recall",
		"notice.group_upload",
		"notice.group_card",
		"notice.group_title",
		"notice.group_essence",
		"notice.group_message_emoji_like":
		return event.ConversationType == "group"
	case "notice.friend_add", "notice.friend_recall", "notice.profile_like", "notice.input_status":
		return event.ConversationType == "private"
	case "notice.poke", "notice.poke_recall", "notice.flash_file":
		return event.ConversationType == "group" || event.ConversationType == "private"
	case "request.friend":
		return event.ConversationType == "private"
	case "request.group":
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

func bridgeEventSummary(action string, event adapter.NormalizedEvent) string {
	base := "adapter event"
	switch event.EventType {
	case "message.group":
		base = "group message"
	case "message.private":
		base = "private message"
	case "notice.member_increase":
		base = "group member increase notice"
	case "notice.member_decrease":
		base = "group member decrease notice"
	case "notice.group_admin":
		base = "group admin notice"
	case "notice.group_ban":
		base = "group ban notice"
	case "notice.group_recall":
		base = "group recall notice"
	case "notice.group_upload":
		base = "group upload notice"
	case "notice.group_card":
		base = "group card notice"
	case "notice.group_title":
		base = "group title notice"
	case "notice.group_essence":
		base = "group essence notice"
	case "notice.group_message_emoji_like":
		base = "group emoji reaction notice"
	case "notice.friend_add":
		base = "friend add notice"
	case "notice.friend_recall":
		base = "friend recall notice"
	case "notice.profile_like":
		base = "profile like notice"
	case "notice.poke":
		base = "poke notice"
	case "notice.poke_recall":
		base = "poke recall notice"
	case "notice.flash_file":
		base = "flash file notice"
	case "request.friend":
		base = "friend request"
	case "request.group":
		base = "group request"
	}

	summary := fmt.Sprintf("runtime bridge %s %s", action, base)
	if text := strings.TrimSpace(event.PlainText); text != "" {
		summary += ": " + summarizeBridgeText(text)
	}
	return summary
}

func summarizeBridgeText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if len(text) > 72 {
		return text[:72] + "..."
	}
	return text
}

func bridgeEventLogAttrs(event adapter.NormalizedEvent) []any {
	attrs := []any{
		"direction", "inbound",
		"event_kind", event.Kind,
		"event_type", event.EventType,
		"conversation_type", event.ConversationType,
		"conversation_id", event.ConversationID,
		"sender_id", event.SenderID,
	}
	if event.MessageID != "" {
		attrs = append(attrs, "message_id", event.MessageID)
	}
	if event.PlainText != "" {
		attrs = append(attrs, "plain_text", event.PlainText)
	}
	if len(event.Segments) > 0 {
		attrs = append(attrs, "segments", bridgeSegmentsToAny(event.Segments))
	}
	return attrs
}

func bridgeActionLogAttrs(action runtime.Action) []any {
	attrs := []any{
		"target_type", action.TargetType,
		"target_id", action.TargetID,
	}
	if len(action.MessageSegments) > 0 {
		attrs = append(attrs, "segments", bridgeActionSegmentsToAny(action.MessageSegments))
	}
	if text := bridgeActionPlainText(action.MessageSegments); text != "" {
		attrs = append(attrs, "plain_text", text)
	}
	return attrs
}

func bridgeSegmentsToAny(segments []adapter.MessageSegment) []any {
	items := make([]any, 0, len(segments))
	for _, segment := range segments {
		items = append(items, map[string]any{
			"type": segment.Type,
			"data": cloneBridgeData(segment.Data),
		})
	}
	return items
}

func bridgeActionSegmentsToAny(segments []runtime.ActionSegment) []any {
	items := make([]any, 0, len(segments))
	for _, segment := range segments {
		items = append(items, map[string]any{
			"type": segment.Type,
			"data": cloneBridgeData(segment.Data),
		})
	}
	return items
}

func bridgeActionPlainText(segments []runtime.ActionSegment) string {
	if len(segments) == 0 {
		return ""
	}

	var builder strings.Builder
	for _, segment := range segments {
		if strings.TrimSpace(segment.Type) != "text" {
			continue
		}
		text, _ := segment.Data["text"].(string)
		builder.WriteString(text)
	}
	return strings.TrimSpace(builder.String())
}

func cloneBridgeData(data map[string]any) map[string]any {
	if len(data) == 0 {
		return map[string]any{}
	}

	cloned := make(map[string]any, len(data))
	for key, value := range data {
		cloned[key] = value
	}
	return cloned
}
