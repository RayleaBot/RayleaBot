package bridge

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
	"github.com/RayleaBot/RayleaBot/server/internal/textsafe"
)

const (
	codePlatformInvalidRequest = "platform.invalid_request"
	codePluginInternalError    = "plugin.internal_error"
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

type dispatcherClient interface {
	HasDeliverablePlugins() bool
	Dispatch(context.Context, runtime.Event, string) []dispatch.DeliveryResult
}

type Bridge struct {
	logger     *slog.Logger
	dispatcher dispatcherClient

	mu               sync.RWMutex
	snapshot         Snapshot
	nextSubscriberID uint64
	subscribers      map[uint64]chan ObservabilityFrame
}

func New(logger *slog.Logger, dispatcher dispatcherClient) *Bridge {
	if logger == nil {
		logger = slog.Default()
	}

	return &Bridge{
		logger:      logger,
		dispatcher:  dispatcher,
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

	if b.dispatcher == nil || !b.dispatcher.HasDeliverablePlugins() {
		b.recordRejected(event, now, codePlatformInvalidRequest, "no deliverable plugin runtime is registered")
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
	if event.PlainText != "" || len(event.Segments) > 0 {
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

	commandName := bridgeCommandName(runtimeEvent)
	results := b.dispatcher.Dispatch(ctx, runtimeEvent, commandName)
	if len(results) == 0 {
		b.recordRejected(event, now, codePlatformInvalidRequest, "no deliverable plugin runtime accepted the event")
		attrs := append([]any{"component", "bridge"}, bridgeEventLogAttrs(event)...)
		attrs = append(attrs, "error_code", codePlatformInvalidRequest)
		if commandName != "" {
			attrs = append(attrs, "command_name", commandName)
		}
		b.logger.Warn(bridgeEventSummary("rejected", event), attrs...)
		return OutcomeRejected
	}

	if bridgeDispatchDelivered(results) {
		b.recordDelivered(event, now)
		attrs := append([]any{"component", "bridge"}, bridgeEventLogAttrs(event)...)
		attrs = append(attrs, bridgeDispatchLogAttrs(results)...)
		if commandName != "" {
			attrs = append(attrs, "command_name", commandName)
		}
		b.logger.Info(bridgeEventSummary("queued for dispatcher", event), attrs...)
		return OutcomeDelivered
	}

	b.recordError(event, now, codePluginInternalError, "eligible plugin runtimes did not accept the event")
	attrs := append([]any{"component", "bridge"}, bridgeEventLogAttrs(event)...)
	attrs = append(attrs, bridgeDispatchLogAttrs(results)...)
	attrs = append(attrs, "error_code", codePluginInternalError)
	if commandName != "" {
		attrs = append(attrs, "command_name", commandName)
	}
	b.logger.Warn(bridgeEventSummary("failed to queue for dispatcher", event), attrs...)
	return OutcomeError
}

func bridgeCommandName(event runtime.Event) string {
	if event.PayloadFields == nil {
		return ""
	}
	command, _ := event.PayloadFields["command"].(string)
	return strings.TrimSpace(command)
}

func bridgeDispatchDelivered(results []dispatch.DeliveryResult) bool {
	for _, result := range results {
		if result.Outcome == dispatch.OutcomeDelivered {
			return true
		}
	}
	return false
}

func bridgeDispatchLogAttrs(results []dispatch.DeliveryResult) []any {
	targetCount := len(results)
	deliveredCount := 0
	droppedCount := 0
	errorCount := 0
	lastErrorCode := ""

	for _, result := range results {
		switch result.Outcome {
		case dispatch.OutcomeDelivered:
			deliveredCount++
		case dispatch.OutcomeDropped:
			droppedCount++
		case dispatch.OutcomeError:
			errorCount++
			if lastErrorCode == "" && strings.TrimSpace(result.ErrorCode) != "" {
				lastErrorCode = result.ErrorCode
			}
		}
	}

	attrs := []any{
		"target_count", targetCount,
		"queued_count", deliveredCount,
		"dropped_count", droppedCount,
		"failed_count", errorCount,
	}
	if lastErrorCode != "" {
		attrs = append(attrs, "dispatch_error_code", lastErrorCode)
	}
	return attrs
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
	if isMessageEventKind(event.Kind) && event.PlainText == "" && len(event.Segments) == 0 {
		return false
	}
	return true
}

func isSupportedEventKind(kind string) bool {
	switch kind {
	case adapter.EventKindMessageText, adapter.EventKindMessage, adapter.EventKindMessageSent, adapter.EventKindNotice, adapter.EventKindRequest:
		return true
	default:
		return false
	}
}

func isMessageEventKind(kind string) bool {
	return kind == adapter.EventKindMessageText || kind == adapter.EventKindMessage || kind == adapter.EventKindMessageSent
}

func isSupportedEventType(event adapter.NormalizedEvent) bool {
	switch event.EventType {
	case "message.group":
		return event.ConversationType == "group"
	case "message.private":
		return event.ConversationType == "private"
	case "message_sent.group":
		return event.ConversationType == "group"
	case "message_sent.private":
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
	if summary, ok := formattedBridgeInboundMessageSummary(event); ok {
		return summary
	}

	base := "adapter event"
	switch event.EventType {
	case "message.group":
		base = "group message"
	case "message.private":
		base = "private message"
	case "message_sent.group":
		base = "sent group message"
	case "message_sent.private":
		base = "sent private message"
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
	text = strings.TrimSpace(textsafe.SanitizeString(text))
	if text == "" {
		return ""
	}
	return textsafe.TruncateRunes(text, 160, "...")
}

func formattedBridgeInboundMessageSummary(event adapter.NormalizedEvent) (string, bool) {
	if strings.TrimSpace(event.SourceProtocol) != "onebot11" {
		return "", false
	}

	messageText := summarizeBridgeText(event.PlainText)
	if messageText == "" {
		return "", false
	}

	botID := strings.TrimSpace(event.BotID)
	if botID == "" {
		return "", false
	}

	senderID := strings.TrimSpace(event.SenderID)
	if senderID == "" {
		return "", false
	}

	senderDisplay := bridgeSenderDisplay(event)
	if senderDisplay == "" {
		senderDisplay = senderID
	}

	switch strings.TrimSpace(event.EventType) {
	case "message.group":
		return fmt.Sprintf("%s: %s%s%s(%s): %s",
			botID,
			bridgeGroupDisplay(event),
			bridgeSenderTitle(event),
			senderDisplay,
			senderID,
			messageText,
		), true
	case "message.private":
		return fmt.Sprintf("%s: %s(%s): %s", botID, senderDisplay, senderID, messageText), true
	default:
		return "", false
	}
}

func bridgeGroupDisplay(event adapter.NormalizedEvent) string {
	groupID := strings.TrimSpace(event.ConversationID)
	groupName := strings.TrimSpace(textsafe.SanitizeString(event.TargetName))
	if groupName == "" {
		return fmt.Sprintf("[%s]", groupID)
	}
	return fmt.Sprintf("[%s(%s)]", groupName, groupID)
}

func bridgeSenderTitle(event adapter.NormalizedEvent) string {
	onebot := bridgeEventOneBotPayload(event)
	if sender, ok := onebot["sender"].(map[string]any); ok {
		if title := strings.TrimSpace(textsafe.SanitizeString(fmt.Sprint(sender["title"]))); title != "" && title != "<nil>" {
			return fmt.Sprintf("[%s]", title)
		}
	}
	return ""
}

func bridgeSenderDisplay(event adapter.NormalizedEvent) string {
	onebot := bridgeEventOneBotPayload(event)
	if sender, ok := onebot["sender"].(map[string]any); ok {
		card := strings.TrimSpace(textsafe.SanitizeString(fmt.Sprint(sender["card"])))
		if card == "<nil>" {
			card = ""
		}
		nickname := strings.TrimSpace(textsafe.SanitizeString(fmt.Sprint(sender["nickname"])))
		if nickname == "<nil>" {
			nickname = ""
		}

		switch {
		case card != "" && nickname != "" && card != nickname:
			return card + "/" + nickname
		case card != "":
			return card
		case nickname != "":
			return nickname
		}
	}

	return strings.TrimSpace(textsafe.SanitizeString(event.ActorNickname))
}

func bridgeEventLogAttrs(event adapter.NormalizedEvent) []any {
	attrs := []any{
		"direction", "inbound",
		"event_kind", event.Kind,
		"event_type", event.EventType,
		"event_timestamp", event.Timestamp,
		"conversation_type", event.ConversationType,
		"conversation_id", event.ConversationID,
		"sender_id", event.SenderID,
	}
	if event.BotID != "" {
		attrs = append(attrs, "self_id", event.BotID)
	}
	if event.TargetName != "" && event.ConversationType == "group" {
		attrs = append(attrs, "group_name", textsafe.SanitizeString(event.TargetName))
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
	if onebot := bridgeEventOneBotPayload(event); len(onebot) > 0 {
		if value, ok := onebot["post_type"]; ok {
			attrs = append(attrs, "post_type", value)
		}
		if value, ok := onebot["message_type"]; ok {
			attrs = append(attrs, "message_type", value)
		}
		if value, ok := onebot["time"]; ok {
			attrs = append(attrs, "time", value)
		}
		if value, ok := onebot["user_id"]; ok {
			attrs = append(attrs, "user_id", value)
		}
		if value, ok := onebot["group_id"]; ok {
			attrs = append(attrs, "group_id", value)
		}
		if value, ok := onebot["real_id"]; ok {
			attrs = append(attrs, "real_id", value)
		}
		if value, ok := onebot["message_seq"]; ok {
			attrs = append(attrs, "message_seq", value)
		}
		if value, ok := onebot["raw_message"]; ok {
			attrs = append(attrs, "raw_message", value)
		}
		if value, ok := onebot["message_format"]; ok {
			attrs = append(attrs, "message_format", value)
		}
		if value, ok := onebot["font"]; ok {
			attrs = append(attrs, "font", value)
		}
		if sender, ok := onebot["sender"].(map[string]any); ok && len(sender) > 0 {
			attrs = append(attrs, "sender", cloneBridgeData(sender))
			if value, ok := sender["nickname"]; ok {
				attrs = append(attrs, "sender_nickname", value)
			}
			if value, ok := sender["card"]; ok {
				attrs = append(attrs, "sender_card", value)
			}
			if value, ok := sender["role"]; ok {
				attrs = append(attrs, "sender_role", value)
			}
			if value, ok := sender["title"]; ok {
				attrs = append(attrs, "sender_title", value)
			}
		}
	}
	return attrs
}

func bridgeEventOneBotPayload(event adapter.NormalizedEvent) map[string]any {
	if event.PayloadFields == nil {
		return map[string]any{}
	}
	raw, ok := event.PayloadFields["onebot"].(map[string]any)
	if !ok || len(raw) == 0 {
		return map[string]any{}
	}
	return cloneBridgeData(raw)
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
