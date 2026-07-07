package onebot11

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/coder/websocket"
)

type Transport interface {
	NextEcho() string
	SendWebSocket(context.Context, SendMsgRequest) (APIResponse, bool, error)
	DoHTTPAPI(context.Context, APICallRequest) (APIResponse, error)
	LogUnsupportedSegment(string)
}

type Sender struct {
	transport Transport
}

func NewSender(transport Transport) Sender {
	return Sender{transport: transport}
}

func (s Sender) SendMessage(ctx context.Context, action OutboundMessageSend) (SendMessageResult, error) {
	targetType, targetID, err := ValidateTarget(action.TargetType, action.TargetID, "message.send")
	if err != nil {
		return SendMessageResult{}, err
	}

	segments, err := NormalizeSegments("message.send", action.Segments, "")
	if err != nil {
		return SendMessageResult{}, err
	}
	s.logUnsupportedSegments(segments.UnsupportedSegment)
	return s.sendSegments(ctx, targetType, targetID, segments.Segments, false)
}

func (s Sender) SendReply(ctx context.Context, action OutboundMessageReply) (SendMessageResult, error) {
	targetType, targetID, err := ValidateTarget(action.TargetType, action.TargetID, "message.reply")
	if err != nil {
		return SendMessageResult{}, err
	}

	replyToID := strings.TrimSpace(action.ReplyToMessageID)
	if replyToID == "" {
		return SendMessageResult{}, Errorf(ErrorCodeSendFailed, "message.reply action is missing required fields", nil)
	}

	segments, err := NormalizeSegments("message.reply", action.Segments, replyToID)
	if err != nil {
		return SendMessageResult{}, err
	}
	s.logUnsupportedSegments(segments.UnsupportedSegment)

	return s.sendSegments(ctx, targetType, targetID, segments.Segments, true)
}

func (s Sender) logUnsupportedSegments(segmentTypes []string) {
	if s.transport == nil {
		return
	}
	for _, segmentType := range segmentTypes {
		s.transport.LogUnsupportedSegment(segmentType)
	}
}

func (s Sender) sendSegments(ctx context.Context, targetType, targetID string, segments []OneBotMessageSegment, replyAttempt bool) (SendMessageResult, error) {
	if s.transport == nil {
		return SendMessageResult{}, Errorf(ErrorCodeSendFailed, "adapter transport is not connected", nil)
	}

	echo := s.transport.NextEcho()
	request := SendMsgRequest{
		Action: "send_msg",
		Params: SendMsgParams{
			MessageType: targetType,
			Message:     segments,
		},
		Echo: echo,
	}
	switch targetType {
	case "group":
		request.Params.GroupID = OneBotTargetValue(targetID)
	case "private":
		request.Params.UserID = OneBotTargetValue(targetID)
	}

	if response, ok, err := s.transport.SendWebSocket(ctx, request); ok || err != nil {
		if err != nil {
			return SendMessageResult{}, err
		}
		return ParseSendMessageResponse(response, replyAttempt)
	}

	params := map[string]any{
		"message_type": targetType,
		"message":      segments,
	}
	if request.Params.UserID != nil {
		params["user_id"] = request.Params.UserID
	}
	if request.Params.GroupID != nil {
		params["group_id"] = request.Params.GroupID
	}
	response, err := s.transport.DoHTTPAPI(ctx, APICallRequest{
		Action: request.Action,
		Params: params,
		Echo:   request.Echo,
	})
	if err != nil {
		return SendMessageResult{}, err
	}
	return ParseSendMessageResponse(response, replyAttempt)
}

func OneBotTargetValue(targetID string) any {
	if value, err := strconv.ParseInt(targetID, 10, 64); err == nil {
		return value
	}
	return targetID
}

func WriteJSON(ctx context.Context, conn WebSocketWriter, value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return conn.Write(ctx, websocket.MessageText, encoded)
}

type WebSocketWriter interface {
	Write(context.Context, websocket.MessageType, []byte) error
}

type FrameResponse struct {
	Echo    any
	Status  any
	RetCode int
	Wording string
	Data    any
}

func APIResponseFromFrame(frame FrameResponse) (APIResponse, bool) {
	echo, ok := frameEcho(frame.Echo)
	if !ok {
		return APIResponse{}, false
	}

	return APIResponse{
		Echo:    echo,
		Status:  frameStatusText(frame.Status),
		RetCode: frame.RetCode,
		Wording: strings.TrimSpace(frame.Wording),
		Data:    frame.Data,
	}, true
}

func ParseSendMessageResponse(response APIResponse, replyAttempt bool) (SendMessageResult, error) {
	if response.Status != "ok" || response.RetCode != 0 {
		message := "adapter send_msg failed"
		if response.Wording != "" {
			message = response.Wording
		}
		if replyAttempt && isReplyTargetMissing(message) {
			return SendMessageResult{}, Errorf(ErrorCodeReplyTargetMissing, message, nil)
		}
		return SendMessageResult{}, Errorf(ErrorCodeSendFailed, message, nil)
	}

	return SendMessageResult{
		MessageID: extractMessageID(response.Data),
	}, nil
}

func isReplyTargetMissing(message string) bool {
	message = strings.TrimSpace(strings.ToLower(message))
	if message == "" {
		return false
	}

	needles := []string{
		"reply target",
		"reply message",
		"reply to message",
		"quoted message",
		"message not found",
		"message not exist",
		"message is not exist",
		"message has been recalled",
		"引用消息不存在",
		"回复目标不存在",
		"回复消息不存在",
		"消息不存在",
		"消息已撤回",
		"目标消息不存在",
	}
	for _, needle := range needles {
		if strings.Contains(message, needle) {
			return true
		}
	}
	return false
}

func extractMessageID(data any) string {
	decoded, ok := data.(map[string]any)
	if !ok || decoded == nil {
		return ""
	}

	switch value := decoded["message_id"].(type) {
	case string:
		return strings.TrimSpace(value)
	case float64:
		return strconv.FormatInt(int64(value), 10)
	case json.Number:
		return value.String()
	default:
		return ""
	}
}
