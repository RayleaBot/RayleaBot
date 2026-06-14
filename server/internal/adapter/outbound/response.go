package outbound

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/coder/websocket"
)

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
