package onebot11

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/redact"
	"github.com/coder/websocket"
)

type FrameCategory string

const (
	FrameCategoryLifecycleReady FrameCategory = "lifecycle_ready"
	FrameCategoryHeartbeat      FrameCategory = "heartbeat"
	FrameCategoryEvent          FrameCategory = "event"
	FrameCategoryAPIResponse    FrameCategory = "api_response"
	FrameCategoryUnknown        FrameCategory = "unknown"
	FrameCategoryInvalid        FrameCategory = "invalid"
)

type FrameSummary struct {
	Category          FrameCategory
	Type              string
	ObservedAt        time.Time
	HeartbeatInterval time.Duration
}

const (
	EventKindMessageText = "onebot11.message_text"
	EventKindMessage     = "onebot11.message"
	EventKindMessageSent = "onebot11.message_sent"
	EventKindNotice      = "onebot11.notice"
	EventKindRequest     = "onebot11.request"
	EventKindMeta        = "onebot11.meta"
)

type NormalizedEvent struct {
	Kind             string
	EventID          string
	BotID            string
	SourceProtocol   string
	SourceAdapter    string
	EventType        string
	Timestamp        int64
	ConversationType string
	ConversationID   string
	SenderID         string
	TargetType       string
	TargetID         string
	PlainText        string
	Segments         []MessageSegment
	MessageID        string
	ActorNickname    string
	ActorRole        string
	TargetName       string
	PayloadFields    map[string]any
}

// MessageSegment represents a structured message segment from the OneBot11
// protocol, normalized into a protocol-agnostic form.
type MessageSegment struct {
	Type string
	Data map[string]any
}

type senderObject struct {
	UserID   int64  `json:"user_id"`
	Nickname string `json:"nickname"`
	Card     string `json:"card"`
	Role     string `json:"role"`
	Title    string `json:"title"`
	Sex      string `json:"sex"`
	Age      int    `json:"age"`
}

type OneBotFrame struct {
	PostType      string          `json:"post_type"`
	MetaEventType string          `json:"meta_event_type"`
	RequestType   string          `json:"request_type"`
	SubType       string          `json:"sub_type"`
	NoticeType    string          `json:"notice_type"`
	Interval      int             `json:"interval"`
	MessageType   string          `json:"message_type"`
	MessageID     int64           `json:"message_id"`
	RealID        int64           `json:"real_id"`
	MessageSeq    int64           `json:"message_seq"`
	GroupName     string          `json:"group_name"`
	Time          int64           `json:"time"`
	SelfID        int64           `json:"self_id"`
	UserID        int64           `json:"user_id"`
	GroupID       int64           `json:"group_id"`
	OperatorID    int64           `json:"operator_id"`
	TargetID      int64           `json:"target_id"`
	RawMessage    string          `json:"raw_message"`
	Font          int             `json:"font"`
	MessageFormat string          `json:"message_format"`
	Message       json.RawMessage `json:"message"`
	Sender        *senderObject   `json:"sender"`
	Status        any             `json:"status"`
	RetCode       int             `json:"retcode"`
	Wording       string          `json:"wording"`
	Data          any             `json:"data"`
	Echo          any             `json:"echo"`
	Comment       string          `json:"comment"`
	Flag          string          `json:"flag"`
}

type ClassifiedFrame struct {
	Summary        FrameSummary
	Frame          OneBotFrame
	InvalidSummary string
	PayloadPreview any
}

func PreviewFramePayload(payload []byte) any {
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 {
		return ""
	}

	var decoded any
	if err := json.Unmarshal(trimmed, &decoded); err == nil {
		return redact.SanitizeAny(decoded)
	}

	text := redact.SanitizeString(string(trimmed))
	if utf8SafeText := redact.TruncateRunes(text, 256, "...(truncated)"); utf8SafeText != text {
		return utf8SafeText
	}
	return text
}

func NormalizeSupportedEvent(frame OneBotFrame, observedAt time.Time) (NormalizedEvent, bool) {
	switch frame.PostType {
	case "message":
		return normalizeMessageEvent(frame, observedAt)
	case "message_sent":
		return normalizeMessageSentEvent(frame, observedAt)
	case "notice":
		return normalizeNoticeEvent(frame, observedAt)
	case "request":
		return normalizeRequestEvent(frame, observedAt)
	case "meta_event":
		return normalizeMetaEvent(frame, observedAt)
	default:
		return NormalizedEvent{}, false
	}
}

func normalizeRequestEvent(frame OneBotFrame, observedAt time.Time) (NormalizedEvent, bool) {
	if frame.SelfID <= 0 || frame.UserID <= 0 {
		return NormalizedEvent{}, false
	}

	var (
		eventType        string
		conversationType string
		conversationID   string
	)
	switch frame.RequestType {
	case "friend":
		eventType = "request.friend"
		conversationType = "private"
		conversationID = fmt.Sprintf("%d", frame.UserID)
	case "group":
		if frame.GroupID <= 0 {
			return NormalizedEvent{}, false
		}
		eventType = "request.group"
		conversationType = "group"
		conversationID = fmt.Sprintf("%d", frame.GroupID)
	default:
		return NormalizedEvent{}, false
	}

	timestamp := frame.Time
	if timestamp <= 0 {
		timestamp = observedAt.Unix()
	}

	eventID := fmt.Sprintf("onebot11-request-%s-%d-%d", strings.ReplaceAll(frame.RequestType, "_", "-"), timestamp, frame.UserID)
	payloadFields := buildCommonPayloadFields(frame)
	if comment := strings.TrimSpace(redact.SanitizeString(frame.Comment)); comment != "" {
		payloadFields["comment"] = comment
	}
	if flag := strings.TrimSpace(redact.SanitizeString(frame.Flag)); flag != "" {
		payloadFields["flag"] = flag
	}

	return NormalizedEvent{
		Kind:             EventKindRequest,
		EventID:          eventID,
		BotID:            fmt.Sprintf("%d", frame.SelfID),
		SourceProtocol:   "onebot11",
		SourceAdapter:    "adapter.onebot11",
		EventType:        eventType,
		Timestamp:        timestamp,
		ConversationType: conversationType,
		ConversationID:   conversationID,
		SenderID:         fmt.Sprintf("%d", frame.UserID),
		PayloadFields:    payloadFields,
	}, true
}

func normalizeMetaEvent(frame OneBotFrame, observedAt time.Time) (NormalizedEvent, bool) {
	if frame.SelfID <= 0 {
		return NormalizedEvent{}, false
	}

	var eventType string
	switch frame.MetaEventType {
	case "heartbeat":
		eventType = "meta.heartbeat"
	case "lifecycle":
		eventType = "meta.lifecycle"
	default:
		return NormalizedEvent{}, false
	}

	timestamp := frame.Time
	if timestamp <= 0 {
		timestamp = observedAt.Unix()
	}

	eventID := fmt.Sprintf("onebot11-meta-%s-%d", strings.ReplaceAll(frame.MetaEventType, "_", "-"), timestamp)
	if subType := strings.TrimSpace(frame.SubType); subType != "" {
		eventID = fmt.Sprintf("onebot11-meta-%s-%s-%d", strings.ReplaceAll(frame.MetaEventType, "_", "-"), strings.ReplaceAll(subType, "_", "-"), timestamp)
	}

	botID := fmt.Sprintf("%d", frame.SelfID)
	return NormalizedEvent{
		Kind:             EventKindMeta,
		EventID:          eventID,
		BotID:            botID,
		SourceProtocol:   "onebot11",
		SourceAdapter:    "adapter.onebot11",
		EventType:        eventType,
		Timestamp:        timestamp,
		ConversationType: "system",
		ConversationID:   "bot:" + botID,
		SenderID:         botID,
		TargetType:       "bot",
		TargetID:         botID,
		PayloadFields:    buildCommonPayloadFields(frame),
	}, true
}

func ClassifyFrame(messageType websocket.MessageType, payload []byte, observedAt time.Time) ClassifiedFrame {
	payloadPreview := PreviewFramePayload(payload)

	if messageType != websocket.MessageText && messageType != websocket.MessageBinary {
		return ClassifiedFrame{
			Summary: FrameSummary{
				Category:   FrameCategoryInvalid,
				Type:       string(FrameCategoryInvalid),
				ObservedAt: observedAt,
			},
			InvalidSummary: "unexpected websocket message type",
			PayloadPreview: payloadPreview,
		}
	}

	var frame OneBotFrame
	if err := json.Unmarshal(payload, &frame); err != nil {
		return ClassifiedFrame{
			Summary: FrameSummary{
				Category:   FrameCategoryInvalid,
				Type:       string(FrameCategoryInvalid),
				ObservedAt: observedAt,
			},
			InvalidSummary: summarizeError(err),
			PayloadPreview: payloadPreview,
		}
	}

	summary := FrameSummary{
		ObservedAt: observedAt,
	}

	switch {
	case frame.PostType == "meta_event" && frame.MetaEventType == "lifecycle" && frame.SubType == "enable":
		summary.Category = FrameCategoryLifecycleReady
		summary.Type = "meta.lifecycle.enable"
	case frame.PostType == "meta_event" && frame.MetaEventType == "lifecycle" && frame.SubType == "connect":
		summary.Category = FrameCategoryLifecycleReady
		summary.Type = "meta.lifecycle.connect"
	case frame.PostType == "meta_event" && frame.MetaEventType == "heartbeat":
		summary.Category = FrameCategoryHeartbeat
		summary.Type = "meta.heartbeat"
		if frame.Interval > 0 {
			summary.HeartbeatInterval = time.Duration(frame.Interval) * time.Millisecond
		}
	case frame.Echo != nil:
		if _, ok := frameEcho(frame.Echo); !ok {
			return ClassifiedFrame{
				Summary: FrameSummary{
					Category:   FrameCategoryUnknown,
					Type:       "api.response.ignored",
					ObservedAt: observedAt,
				},
				InvalidSummary: "api response echo must be a non-empty string",
				Frame:          frame,
				PayloadPreview: payloadPreview,
			}
		}
		summary.Category = FrameCategoryAPIResponse
		summary.Type = "api.response"
	case frame.PostType != "":
		summary.Category = FrameCategoryEvent
		summary.Type = frame.PostType
	default:
		summary.Category = FrameCategoryUnknown
		summary.Type = string(FrameCategoryUnknown)
	}

	return ClassifiedFrame{
		Summary:        summary,
		Frame:          frame,
		PayloadPreview: payloadPreview,
	}
}

func frameEcho(value any) (string, bool) {
	echo, ok := value.(string)
	if !ok {
		return "", false
	}
	echo = strings.TrimSpace(echo)
	if echo == "" {
		return "", false
	}
	return echo, true
}

func FrameEcho(value any) (string, bool) {
	return frameEcho(value)
}

func frameStatusText(value any) string {
	status, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(status)
}

func FrameStatusText(value any) string {
	return frameStatusText(value)
}

func summarizeError(err error) string {
	if err == nil {
		return ""
	}

	return strings.Join(strings.Fields(err.Error()), " ")
}

func normalizeMessageEvent(frame OneBotFrame, observedAt time.Time) (NormalizedEvent, bool) {
	return normalizeMessageLikeEvent(frame, observedAt, false)
}

func normalizeMessageSentEvent(frame OneBotFrame, observedAt time.Time) (NormalizedEvent, bool) {
	return normalizeMessageLikeEvent(frame, observedAt, true)
}

func normalizeMessageLikeEvent(frame OneBotFrame, observedAt time.Time, sent bool) (NormalizedEvent, bool) {
	if frame.SelfID <= 0 || frame.UserID <= 0 {
		return NormalizedEvent{}, false
	}

	var eventType string
	var conversationType string
	var conversationID string
	switch frame.MessageType {
	case "private":
		if sent {
			eventType = "message_sent.private"
		} else {
			eventType = "message.private"
		}
		conversationType = "private"
		conversationID = fmt.Sprintf("%d", frame.UserID)
	case "group":
		if frame.GroupID <= 0 {
			return NormalizedEvent{}, false
		}
		if sent {
			eventType = "message_sent.group"
		} else {
			eventType = "message.group"
		}
		conversationType = "group"
		conversationID = fmt.Sprintf("%d", frame.GroupID)
	default:
		return NormalizedEvent{}, false
	}

	timestamp := frame.Time
	if timestamp <= 0 {
		timestamp = observedAt.Unix()
	}

	eventID := fmt.Sprintf("onebot11-message-%d-%d", timestamp, frame.UserID)
	if frame.MessageID > 0 {
		eventID = fmt.Sprintf("onebot11-message-%d", frame.MessageID)
	}
	if sent {
		eventID = fmt.Sprintf("onebot11-message-sent-%d-%d", timestamp, frame.UserID)
		if frame.MessageID > 0 {
			eventID = fmt.Sprintf("onebot11-message-sent-%d", frame.MessageID)
		}
	}

	segments := parseFrameMessage(frame)
	plainText := strings.TrimSpace(ToPlainText(segments))
	if plainText == "" {
		plainText = strings.TrimSpace(redact.SanitizeString(frame.RawMessage))
	}
	if plainText == "" && len(segments) == 0 {
		return NormalizedEvent{}, false
	}

	var actorNickname, actorRole string
	if frame.Sender != nil {
		actorNickname = redact.SanitizeString(frame.Sender.Card)
		if actorNickname == "" {
			actorNickname = redact.SanitizeString(frame.Sender.Nickname)
		}
		actorRole = strings.TrimSpace(redact.SanitizeString(frame.Sender.Role))
	}

	var messageID string
	if frame.MessageID > 0 {
		messageID = fmt.Sprintf("%d", frame.MessageID)
	}

	payloadFields := buildCommonPayloadFields(frame)

	return NormalizedEvent{
		Kind: func() string {
			if sent {
				return EventKindMessageSent
			}
			return EventKindMessage
		}(),
		EventID:          eventID,
		BotID:            fmt.Sprintf("%d", frame.SelfID),
		SourceProtocol:   "onebot11",
		SourceAdapter:    "adapter.onebot11",
		EventType:        eventType,
		Timestamp:        timestamp,
		ConversationType: conversationType,
		ConversationID:   conversationID,
		SenderID:         fmt.Sprintf("%d", frame.UserID),
		PlainText:        plainText,
		Segments:         segments,
		MessageID:        messageID,
		ActorNickname:    actorNickname,
		ActorRole:        actorRole,
		PayloadFields:    payloadFields,
	}, true
}

func parseFrameMessage(frame OneBotFrame) []MessageSegment {
	if len(frame.Message) > 0 {
		trimmed := strings.TrimSpace(string(frame.Message))
		if len(trimmed) > 0 && trimmed[0] == '[' {
			if segments, err := ParseMessageArray(frame.Message); err == nil && len(segments) > 0 {
				return segments
			}
		}
	}
	if frame.RawMessage != "" {
		return ParseCQString(frame.RawMessage)
	}
	return nil
}

func buildCommonPayloadFields(frame OneBotFrame) map[string]any {
	payloadFields := map[string]any{}
	if frame.SubType != "" {
		payloadFields["sub_type"] = frame.SubType
	}
	if frame.NoticeType != "" {
		payloadFields["notice_type"] = frame.NoticeType
	}
	if frame.RequestType != "" {
		payloadFields["request_type"] = frame.RequestType
	}
	if frame.OperatorID > 0 {
		payloadFields["operator_id"] = fmt.Sprintf("%d", frame.OperatorID)
	}
	if frame.TargetID > 0 {
		payloadFields["target_id"] = fmt.Sprintf("%d", frame.TargetID)
	}
	if frame.Sender != nil {
		payloadFields["sender"] = buildSenderPayload(frame.Sender)
	}
	if data := buildDataPayload(frame.Data); len(data) > 0 {
		payloadFields["data"] = data
	}
	if onebot := buildOneBotPayload(frame); len(onebot) > 0 {
		payloadFields["onebot"] = onebot
	}
	return payloadFields
}

func buildSenderPayload(sender *senderObject) map[string]any {
	if sender == nil {
		return map[string]any{}
	}
	payload := map[string]any{}
	if sender.UserID > 0 {
		payload["user_id"] = fmt.Sprintf("%d", sender.UserID)
	}
	if nickname := redact.SanitizeString(sender.Nickname); nickname != "" {
		payload["nickname"] = nickname
	}
	if card := redact.SanitizeString(sender.Card); card != "" {
		payload["card"] = card
	}
	if role := strings.TrimSpace(redact.SanitizeString(sender.Role)); role != "" {
		payload["role"] = role
	}
	if title := redact.SanitizeString(sender.Title); title != "" {
		payload["title"] = title
	}
	if sex := strings.TrimSpace(redact.SanitizeString(sender.Sex)); sex != "" {
		payload["sex"] = sex
	}
	if sender.Age > 0 {
		payload["age"] = sender.Age
	}
	return payload
}

func buildOneBotPayload(frame OneBotFrame) map[string]any {
	payload := map[string]any{}
	if frame.PostType != "" {
		payload["post_type"] = frame.PostType
	}
	if frame.MessageType != "" {
		payload["message_type"] = frame.MessageType
	}
	if frame.RequestType != "" {
		payload["request_type"] = frame.RequestType
	}
	if frame.NoticeType != "" {
		payload["notice_type"] = frame.NoticeType
	}
	if frame.MetaEventType != "" {
		payload["meta_event_type"] = frame.MetaEventType
	}
	if frame.SubType != "" {
		payload["sub_type"] = frame.SubType
	}
	if frame.SelfID > 0 {
		payload["self_id"] = fmt.Sprintf("%d", frame.SelfID)
	}
	if frame.UserID > 0 {
		payload["user_id"] = fmt.Sprintf("%d", frame.UserID)
	}
	if frame.GroupID > 0 {
		payload["group_id"] = fmt.Sprintf("%d", frame.GroupID)
	}
	if groupName := redact.SanitizeString(frame.GroupName); groupName != "" {
		payload["group_name"] = groupName
	}
	if frame.TargetID > 0 {
		payload["target_id"] = fmt.Sprintf("%d", frame.TargetID)
	}
	if frame.Time > 0 {
		payload["time"] = frame.Time
	}
	if frame.Interval > 0 {
		payload["interval"] = frame.Interval
	}
	if frame.MessageID > 0 {
		payload["message_id"] = fmt.Sprintf("%d", frame.MessageID)
	}
	if frame.RealID > 0 {
		payload["real_id"] = fmt.Sprintf("%d", frame.RealID)
	}
	if frame.MessageSeq > 0 {
		payload["message_seq"] = fmt.Sprintf("%d", frame.MessageSeq)
	}
	if rawMessage := redact.SanitizeString(frame.RawMessage); rawMessage != "" {
		payload["raw_message"] = rawMessage
	}
	if frame.Font > 0 {
		payload["font"] = frame.Font
	}
	if messageFormat := strings.TrimSpace(redact.SanitizeString(frame.MessageFormat)); messageFormat != "" {
		payload["message_format"] = messageFormat
	}
	if sender := buildSenderPayload(frame.Sender); len(sender) > 0 {
		payload["sender"] = sender
	}
	if comment := strings.TrimSpace(redact.SanitizeString(frame.Comment)); comment != "" {
		payload["comment"] = comment
	}
	if flag := strings.TrimSpace(redact.SanitizeString(frame.Flag)); flag != "" {
		payload["flag"] = flag
	}
	if status := buildDataPayload(frame.Status); len(status) > 0 {
		payload["status"] = status
	}
	return payload
}

func buildDataPayload(raw any) map[string]any {
	decoded, ok := raw.(map[string]any)
	if !ok || len(decoded) == 0 {
		return map[string]any{}
	}
	payload := make(map[string]any, len(decoded))
	for key, value := range decoded {
		payload[key] = redact.SanitizeAny(value)
	}
	return payload
}

func messageIDString(messageID int64) string {
	if messageID <= 0 {
		return ""
	}
	return fmt.Sprintf("%d", messageID)
}

func normalizeNoticeEvent(frame OneBotFrame, observedAt time.Time) (NormalizedEvent, bool) {
	if frame.SelfID <= 0 {
		return NormalizedEvent{}, false
	}

	var eventType string
	conversationType := "group"
	conversationID := fmt.Sprintf("%d", frame.GroupID)
	senderID := fmt.Sprintf("%d", frame.UserID)
	switch frame.NoticeType {
	case "group_increase":
		if frame.UserID <= 0 || frame.GroupID <= 0 {
			return NormalizedEvent{}, false
		}
		eventType = "notice.member_increase"
	case "group_decrease":
		if frame.UserID <= 0 || frame.GroupID <= 0 {
			return NormalizedEvent{}, false
		}
		eventType = "notice.member_decrease"
	case "group_admin":
		if frame.UserID <= 0 || frame.GroupID <= 0 {
			return NormalizedEvent{}, false
		}
		eventType = "notice.group_admin"
	case "group_ban":
		if frame.UserID <= 0 || frame.GroupID <= 0 {
			return NormalizedEvent{}, false
		}
		eventType = "notice.group_ban"
	case "group_recall":
		if frame.UserID <= 0 || frame.GroupID <= 0 {
			return NormalizedEvent{}, false
		}
		eventType = "notice.group_recall"
	case "group_upload":
		if frame.UserID <= 0 || frame.GroupID <= 0 {
			return NormalizedEvent{}, false
		}
		eventType = "notice.group_upload"
	case "group_card":
		if frame.UserID <= 0 || frame.GroupID <= 0 {
			return NormalizedEvent{}, false
		}
		eventType = "notice.group_card"
	case "group_title":
		if frame.UserID <= 0 || frame.GroupID <= 0 {
			return NormalizedEvent{}, false
		}
		eventType = "notice.group_title"
	case "essence":
		if frame.UserID <= 0 || frame.GroupID <= 0 {
			return NormalizedEvent{}, false
		}
		eventType = "notice.group_essence"
	case "friend_add":
		if frame.UserID <= 0 {
			return NormalizedEvent{}, false
		}
		eventType = "notice.friend_add"
		conversationType = "private"
		conversationID = fmt.Sprintf("%d", frame.UserID)
	case "friend_recall":
		if frame.UserID <= 0 {
			return NormalizedEvent{}, false
		}
		eventType = "notice.friend_recall"
		conversationType = "private"
		conversationID = fmt.Sprintf("%d", frame.UserID)
	case "notify":
		return normalizeNotifyEvent(frame, observedAt)
	case "flash_file":
		if frame.UserID <= 0 {
			return NormalizedEvent{}, false
		}
		eventType = "notice.flash_file"
		if frame.GroupID <= 0 {
			conversationType = "private"
			conversationID = fmt.Sprintf("%d", frame.UserID)
		}
	default:
		return NormalizedEvent{}, false
	}

	if conversationID == "0" || senderID == "0" {
		return NormalizedEvent{}, false
	}

	timestamp := frame.Time
	if timestamp <= 0 {
		timestamp = observedAt.Unix()
	}

	eventID := fmt.Sprintf("onebot11-notice-%s-%d-%d", strings.ReplaceAll(frame.NoticeType, "_", "-"), timestamp, frame.UserID)
	if frame.MessageID > 0 {
		eventID = fmt.Sprintf("onebot11-notice-%s-%d", strings.ReplaceAll(frame.NoticeType, "_", "-"), frame.MessageID)
	}

	payloadFields := buildCommonPayloadFields(frame)

	return NormalizedEvent{
		Kind:             EventKindNotice,
		EventID:          eventID,
		BotID:            fmt.Sprintf("%d", frame.SelfID),
		SourceProtocol:   "onebot11",
		SourceAdapter:    "adapter.onebot11",
		EventType:        eventType,
		Timestamp:        timestamp,
		ConversationType: conversationType,
		ConversationID:   conversationID,
		SenderID:         senderID,
		MessageID:        messageIDString(frame.MessageID),
		PayloadFields:    payloadFields,
	}, true
}

func normalizeNotifyEvent(frame OneBotFrame, observedAt time.Time) (NormalizedEvent, bool) {
	if frame.SelfID <= 0 || frame.UserID <= 0 {
		return NormalizedEvent{}, false
	}

	conversationType := "private"
	conversationID := fmt.Sprintf("%d", frame.UserID)
	if frame.GroupID > 0 {
		conversationType = "group"
		conversationID = fmt.Sprintf("%d", frame.GroupID)
	}

	var eventType string
	switch frame.SubType {
	case "poke":
		eventType = "notice.poke"
	case "poke_recall":
		eventType = "notice.poke_recall"
	case "profile_like":
		eventType = "notice.profile_like"
		conversationType = "private"
		conversationID = fmt.Sprintf("%d", frame.UserID)
	case "input_status":
		eventType = "notice.input_status"
	case "group_msg_emoji_like":
		eventType = "notice.group_message_emoji_like"
	default:
		return NormalizedEvent{}, false
	}

	timestamp := frame.Time
	if timestamp <= 0 {
		timestamp = observedAt.Unix()
	}

	payloadFields := buildCommonPayloadFields(frame)
	eventID := fmt.Sprintf("onebot11-notify-%s-%d-%d", strings.ReplaceAll(frame.SubType, "_", "-"), timestamp, frame.UserID)
	if frame.MessageID > 0 {
		eventID = fmt.Sprintf("onebot11-notify-%s-%d", strings.ReplaceAll(frame.SubType, "_", "-"), frame.MessageID)
	}

	return NormalizedEvent{
		Kind:             EventKindNotice,
		EventID:          eventID,
		BotID:            fmt.Sprintf("%d", frame.SelfID),
		SourceProtocol:   "onebot11",
		SourceAdapter:    "adapter.onebot11",
		EventType:        eventType,
		Timestamp:        timestamp,
		ConversationType: conversationType,
		ConversationID:   conversationID,
		SenderID:         fmt.Sprintf("%d", frame.UserID),
		MessageID:        messageIDString(frame.MessageID),
		PayloadFields:    payloadFields,
	}, true
}
