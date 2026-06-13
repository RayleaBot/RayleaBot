package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/coder/websocket"
)

func (s *Shell) routeAPIResponse(frame classifiedFrame) {
	if frame.Summary.Category != FrameCategoryAPIResponse {
		return
	}

	response, ok := apiResponseFromFrame(frame.Frame)
	if !ok {
		return
	}

	pending, found := s.takePendingResponse(response.Echo)
	if !found {
		s.logger.Warn(
			"adapter api response had no pending request",
			"component", "adapter",
			"adapter_state", s.Snapshot().State,
			"direction", "inbound",
			"echo", response.Echo,
			"status", response.Status,
			"retcode", response.RetCode,
			"wording", response.Wording,
			"payload_preview", response.Data,
		)
		return
	}

	select {
	case pending <- response:
	default:
	}
}

func wsjsonWrite(ctx context.Context, conn websocketWriter, value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return conn.Write(ctx, websocket.MessageText, encoded)
}

type websocketWriter interface {
	Write(context.Context, websocket.MessageType, []byte) error
}

func (s *Shell) nextRequestEcho() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextEcho++
	return fmt.Sprintf("adapter-%d", s.nextEcho)
}

func (s *Shell) registerPendingResponse(echo string, responseCh chan apiResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.pendingResponses[echo] = responseCh
}

func (s *Shell) dropPendingResponse(echo string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.pendingResponses, echo)
}

func (s *Shell) takePendingResponse(echo string) (chan apiResponse, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	responseCh, ok := s.pendingResponses[echo]
	if ok {
		delete(s.pendingResponses, echo)
	}
	return responseCh, ok
}

func apiResponseFromFrame(frame oneBotFrame) (apiResponse, bool) {
	echo, ok := frameEcho(frame.Echo)
	if !ok {
		return apiResponse{}, false
	}

	return apiResponse{
		Echo:    echo,
		Status:  frameStatusText(frame.Status),
		RetCode: frame.RetCode,
		Wording: strings.TrimSpace(frame.Wording),
		Data:    frame.Data,
	}, true
}

func parseSendMessageResponse(response apiResponse, replyAttempt bool) (SendMessageResult, error) {
	if response.Status != "ok" || response.RetCode != 0 {
		message := "adapter send_msg failed"
		if response.Wording != "" {
			message = response.Wording
		}
		if replyAttempt && isReplyTargetMissing(message) {
			return SendMessageResult{}, errorf(errorCodeReplyTargetMissing, message, nil)
		}
		return SendMessageResult{}, errorf(errorCodeSendFailed, message, nil)
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
