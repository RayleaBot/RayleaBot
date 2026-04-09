package adapter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

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
	EventKindNotice      = "onebot11.notice"
	EventKindRequest     = "onebot11.request"
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
	PlainText        string
	Segments         []MessageSegment
	MessageID        string
	ActorNickname    string
	ActorRole        string
	TargetName       string
	PayloadFields    map[string]any
}

type senderObject struct {
	UserID   int64  `json:"user_id"`
	Nickname string `json:"nickname"`
	Card     string `json:"card"`
	Role     string `json:"role"`
	Sex      string `json:"sex"`
	Age      int    `json:"age"`
}

type oneBotFrame struct {
	PostType      string          `json:"post_type"`
	MetaEventType string          `json:"meta_event_type"`
	RequestType   string          `json:"request_type"`
	SubType       string          `json:"sub_type"`
	NoticeType    string          `json:"notice_type"`
	Interval      int             `json:"interval"`
	MessageType   string          `json:"message_type"`
	MessageID     int64           `json:"message_id"`
	Time          int64           `json:"time"`
	SelfID        int64           `json:"self_id"`
	UserID        int64           `json:"user_id"`
	GroupID       int64           `json:"group_id"`
	OperatorID    int64           `json:"operator_id"`
	TargetID      int64           `json:"target_id"`
	RawMessage    string          `json:"raw_message"`
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

type classifiedFrame struct {
	Summary        FrameSummary
	Frame          oneBotFrame
	InvalidSummary string
	PayloadPreview any
}

func classifyFrame(messageType websocket.MessageType, payload []byte, observedAt time.Time) classifiedFrame {
	payloadPreview := previewFramePayload(payload)

	if messageType != websocket.MessageText && messageType != websocket.MessageBinary {
		return classifiedFrame{
			Summary: FrameSummary{
				Category:   FrameCategoryInvalid,
				Type:       string(FrameCategoryInvalid),
				ObservedAt: observedAt,
			},
			InvalidSummary: "unexpected websocket message type",
			PayloadPreview: payloadPreview,
		}
	}

	var frame oneBotFrame
	if err := json.Unmarshal(payload, &frame); err != nil {
		return classifiedFrame{
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
			return classifiedFrame{
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

	return classifiedFrame{
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

func frameStatusText(value any) string {
	status, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(status)
}

func previewFramePayload(payload []byte) any {
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 {
		return ""
	}

	var decoded any
	if err := json.Unmarshal(trimmed, &decoded); err == nil {
		return decoded
	}

	text := string(trimmed)
	if len(text) > 256 {
		return text[:256] + "...(truncated)"
	}
	return text
}

func applyFrameSummary(snapshot *Snapshot, frame classifiedFrame) {
	if snapshot == nil {
		return
	}
	summary := frame.Summary

	snapshot.TotalReceivedFrames++
	snapshot.LastFrameCategory = summary.Category
	snapshot.LastFrameType = summary.Type
	if frame.Frame.SelfID > 0 {
		snapshot.BotID = fmt.Sprintf("%d", frame.Frame.SelfID)
	}

	if summary.Category == FrameCategoryInvalid {
		snapshot.InvalidReceivedFrames++
	} else {
		snapshot.LastFrameAt = cloneTime(&summary.ObservedAt)
	}

	if summary.Category == FrameCategoryHeartbeat {
		snapshot.HeartbeatSeen = true
		snapshot.LastHeartbeatAt = cloneTime(&summary.ObservedAt)
		if summary.HeartbeatInterval > 0 {
			snapshot.HeartbeatInterval = summary.HeartbeatInterval
		}
	}
}

func isReadySummary(summary FrameSummary) bool {
	return summary.Category == FrameCategoryLifecycleReady || summary.Category == FrameCategoryHeartbeat
}

func isLifecycleDisable(frame oneBotFrame) bool {
	return frame.PostType == "meta_event" && frame.MetaEventType == "lifecycle" && frame.SubType == "disable"
}

func normalizeSupportedEvent(frame oneBotFrame, observedAt time.Time) (NormalizedEvent, bool) {
	switch frame.PostType {
	case "message":
		return normalizeMessageEvent(frame, observedAt)
	case "notice":
		return normalizeNoticeEvent(frame, observedAt)
	case "request":
		return normalizeRequestEvent(frame, observedAt)
	default:
		return NormalizedEvent{}, false
	}
}

func normalizeMessageEvent(frame oneBotFrame, observedAt time.Time) (NormalizedEvent, bool) {
	if frame.SelfID <= 0 || frame.UserID <= 0 {
		return NormalizedEvent{}, false
	}

	var eventType string
	var conversationType string
	var conversationID string
	switch frame.MessageType {
	case "private":
		eventType = "message.private"
		conversationType = "private"
		conversationID = fmt.Sprintf("%d", frame.UserID)
	case "group":
		if frame.GroupID <= 0 {
			return NormalizedEvent{}, false
		}
		eventType = "message.group"
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

	// Parse message segments from either JSON array or CQ string.
	segments := parseFrameMessage(frame)
	plainText := strings.TrimSpace(segmentsToPlainText(segments))
	if plainText == "" {
		plainText = strings.TrimSpace(frame.RawMessage)
	}
	if plainText == "" {
		return NormalizedEvent{}, false
	}

	// Extract sender info.
	var actorNickname, actorRole string
	if frame.Sender != nil {
		actorNickname = frame.Sender.Card
		if actorNickname == "" {
			actorNickname = frame.Sender.Nickname
		}
		actorRole = frame.Sender.Role
	}

	var messageID string
	if frame.MessageID > 0 {
		messageID = fmt.Sprintf("%d", frame.MessageID)
	}

	return NormalizedEvent{
		Kind:             EventKindMessage,
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
	}, true
}

func normalizeNoticeEvent(frame oneBotFrame, observedAt time.Time) (NormalizedEvent, bool) {
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

func normalizeNotifyEvent(frame oneBotFrame, observedAt time.Time) (NormalizedEvent, bool) {
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

func normalizeRequestEvent(frame oneBotFrame, observedAt time.Time) (NormalizedEvent, bool) {
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
	if comment := strings.TrimSpace(frame.Comment); comment != "" {
		payloadFields["comment"] = comment
	}
	if flag := strings.TrimSpace(frame.Flag); flag != "" {
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

func buildCommonPayloadFields(frame oneBotFrame) map[string]any {
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
	return payloadFields
}

func buildSenderPayload(sender *senderObject) map[string]any {
	if sender == nil {
		return map[string]any{}
	}
	payload := map[string]any{
		"user_id": fmt.Sprintf("%d", sender.UserID),
	}
	if sender.Nickname != "" {
		payload["nickname"] = sender.Nickname
	}
	if sender.Card != "" {
		payload["card"] = sender.Card
	}
	if sender.Role != "" {
		payload["role"] = sender.Role
	}
	if sender.Sex != "" {
		payload["sex"] = sender.Sex
	}
	if sender.Age > 0 {
		payload["age"] = sender.Age
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
		payload[key] = value
	}
	return payload
}

func messageIDString(messageID int64) string {
	if messageID <= 0 {
		return ""
	}
	return fmt.Sprintf("%d", messageID)
}

// parseFrameMessage extracts segments from the OneBot frame Message field,
// falling back to CQ code parsing from RawMessage.
func parseFrameMessage(frame oneBotFrame) []MessageSegment {
	if len(frame.Message) > 0 {
		// Try JSON array format first.
		trimmed := strings.TrimSpace(string(frame.Message))
		if len(trimmed) > 0 && trimmed[0] == '[' {
			if segments, err := parseMessageArray(frame.Message); err == nil && len(segments) > 0 {
				return segments
			}
		}
	}
	// Fall back to CQ string from raw_message.
	if frame.RawMessage != "" {
		return parseCQString(frame.RawMessage)
	}
	return nil
}
