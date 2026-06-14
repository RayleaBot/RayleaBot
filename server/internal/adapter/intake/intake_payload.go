package intake

import (
	"fmt"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/textsafe"
)

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
	if nickname := textsafe.SanitizeString(sender.Nickname); nickname != "" {
		payload["nickname"] = nickname
	}
	if card := textsafe.SanitizeString(sender.Card); card != "" {
		payload["card"] = card
	}
	if role := strings.TrimSpace(textsafe.SanitizeString(sender.Role)); role != "" {
		payload["role"] = role
	}
	if title := textsafe.SanitizeString(sender.Title); title != "" {
		payload["title"] = title
	}
	if sex := strings.TrimSpace(textsafe.SanitizeString(sender.Sex)); sex != "" {
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
	if groupName := textsafe.SanitizeString(frame.GroupName); groupName != "" {
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
	if rawMessage := textsafe.SanitizeString(frame.RawMessage); rawMessage != "" {
		payload["raw_message"] = rawMessage
	}
	if frame.Font > 0 {
		payload["font"] = frame.Font
	}
	if messageFormat := strings.TrimSpace(textsafe.SanitizeString(frame.MessageFormat)); messageFormat != "" {
		payload["message_format"] = messageFormat
	}
	if sender := buildSenderPayload(frame.Sender); len(sender) > 0 {
		payload["sender"] = sender
	}
	if comment := strings.TrimSpace(textsafe.SanitizeString(frame.Comment)); comment != "" {
		payload["comment"] = comment
	}
	if flag := strings.TrimSpace(textsafe.SanitizeString(frame.Flag)); flag != "" {
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
		payload[key] = textsafe.SanitizeAny(value)
	}
	return payload
}

func messageIDString(messageID int64) string {
	if messageID <= 0 {
		return ""
	}
	return fmt.Sprintf("%d", messageID)
}
