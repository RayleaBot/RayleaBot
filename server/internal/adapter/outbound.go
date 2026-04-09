package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/coder/websocket"
)

const errorCodeSendFailed = "adapter.send_failed"
const errorCodeReplyTargetMissing = "adapter.reply_target_missing"

type Error struct {
	Code    string
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Err == nil {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func errorf(code, message string, err error) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

type OutboundMessageSend struct {
	TargetType string
	TargetID   string
	Segments   []OutboundMessageSegment
}

type OutboundMessageReply struct {
	TargetType       string
	TargetID         string
	ReplyToMessageID string
	Segments         []OutboundMessageSegment
}

type OutboundMessageSegment struct {
	Type string
	Data map[string]any
}

type SendMessageResult struct {
	MessageID string
}

type sendMsgRequest struct {
	Action string        `json:"action"`
	Params sendMsgParams `json:"params"`
	Echo   string        `json:"echo"`
}

type sendMsgParams struct {
	MessageType string `json:"message_type"`
	UserID      any    `json:"user_id,omitempty"`
	GroupID     any    `json:"group_id,omitempty"`
	Message     any    `json:"message"`
}

type oneBotMessageSegment struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data,omitempty"`
}

type apiResponse struct {
	Echo    string
	Status  string
	RetCode int
	Wording string
	Data    any
}

func (s *Shell) SendMessage(ctx context.Context, action OutboundMessageSend) (SendMessageResult, error) {
	targetType, targetID, err := validateOutboundTarget(action.TargetType, action.TargetID, "message.send")
	if err != nil {
		return SendMessageResult{}, err
	}

	segments, err := s.normalizeOutboundSegments("message.send", action.Segments, "")
	if err != nil {
		return SendMessageResult{}, err
	}
	return s.sendSegments(ctx, targetType, targetID, segments, false)
}

func (s *Shell) SendReply(ctx context.Context, action OutboundMessageReply) (SendMessageResult, error) {
	targetType, targetID, err := validateOutboundTarget(action.TargetType, action.TargetID, "message.reply")
	if err != nil {
		return SendMessageResult{}, err
	}

	replyToID := strings.TrimSpace(action.ReplyToMessageID)
	if replyToID == "" {
		return SendMessageResult{}, errorf(errorCodeSendFailed, "message.reply action is missing required fields", nil)
	}

	segments, err := s.normalizeOutboundSegments("message.reply", action.Segments, replyToID)
	if err != nil {
		return SendMessageResult{}, err
	}

	return s.sendSegments(ctx, targetType, targetID, segments, true)
}

func validateOutboundTarget(rawType, rawID, actionKind string) (string, string, error) {
	targetType := strings.TrimSpace(rawType)
	targetID := strings.TrimSpace(rawID)
	if targetID == "" {
		return "", "", errorf(errorCodeSendFailed, actionKind+" action is missing required fields", nil)
	}
	switch targetType {
	case "group", "private":
		return targetType, targetID, nil
	default:
		return "", "", errorf(errorCodeSendFailed, actionKind+" uses unsupported target_type", nil)
	}
}

func (s *Shell) normalizeOutboundSegments(actionKind string, declared []OutboundMessageSegment, replyToMessageID string) ([]oneBotMessageSegment, error) {
	segments := make([]OutboundMessageSegment, 0, len(declared)+1)
	for _, segment := range declared {
		segments = append(segments, OutboundMessageSegment{
			Type: segment.Type,
			Data: cloneOutboundSegmentData(segment.Data),
		})
	}
	if len(segments) == 0 {
		return nil, errorf(errorCodeSendFailed, actionKind+" action is missing required fields", nil)
	}
	if replyToMessageID != "" {
		reply := OutboundMessageSegment{
			Type: "reply",
			Data: map[string]any{"message_id": replyToMessageID},
		}
		segments = prependReplySegment(segments, reply)
	}

	converted := make([]oneBotMessageSegment, 0, len(segments))
	for _, segment := range segments {
		oneBotSegment, ok := convertOutboundSegment(segment)
		if !ok {
			s.logger.Warn(
				"dropping unsupported outbound message segment",
				"component", "adapter",
				"segment_type", segment.Type,
			)
			continue
		}
		converted = append(converted, oneBotSegment)
	}
	if len(converted) == 0 {
		return nil, errorf(errorCodeSendFailed, "outbound message became empty after segment normalization", nil)
	}
	return converted, nil
}

func prependReplySegment(segments []OutboundMessageSegment, reply OutboundMessageSegment) []OutboundMessageSegment {
	result := make([]OutboundMessageSegment, 0, len(segments)+1)
	result = append(result, reply)
	for _, segment := range segments {
		if strings.TrimSpace(segment.Type) == "reply" {
			continue
		}
		result = append(result, segment)
	}
	return result
}

func convertOutboundSegment(segment OutboundMessageSegment) (oneBotMessageSegment, bool) {
	switch strings.TrimSpace(segment.Type) {
	case "text":
		text, ok := outboundSegmentString(segment.Data, "text")
		if !ok || text == "" {
			return oneBotMessageSegment{}, false
		}
		return oneBotMessageSegment{
			Type: "text",
			Data: map[string]any{"text": text},
		}, true
	case "image":
		if file, ok := outboundSegmentString(segment.Data, "file"); ok && file != "" {
			return oneBotMessageSegment{
				Type: "image",
				Data: map[string]any{"file": file},
			}, true
		}
		if url, ok := outboundSegmentString(segment.Data, "url"); ok && url != "" {
			return oneBotMessageSegment{
				Type: "image",
				Data: map[string]any{"file": url},
			}, true
		}
		return oneBotMessageSegment{}, false
	case "at":
		userID, ok := outboundSegmentString(segment.Data, "user_id")
		if !ok || userID == "" {
			return oneBotMessageSegment{}, false
		}
		return oneBotMessageSegment{
			Type: "at",
			Data: map[string]any{"qq": userID},
		}, true
	case "at_all":
		return oneBotMessageSegment{
			Type: "at",
			Data: map[string]any{"qq": "all"},
		}, true
	case "face":
		faceID, ok := outboundSegmentString(segment.Data, "face_id")
		if !ok || faceID == "" {
			return oneBotMessageSegment{}, false
		}
		return oneBotMessageSegment{
			Type: "face",
			Data: map[string]any{"id": faceID},
		}, true
	case "reply":
		messageID, ok := outboundSegmentString(segment.Data, "message_id")
		if !ok || messageID == "" {
			return oneBotMessageSegment{}, false
		}
		return oneBotMessageSegment{
			Type: "reply",
			Data: map[string]any{"id": messageID},
		}, true
	case "record", "video", "file", "json", "xml", "markdown", "music", "contact", "forward", "node", "poke", "dice", "rps", "mface", "keyboard", "shake":
		return oneBotMessageSegment{
			Type: strings.TrimSpace(segment.Type),
			Data: cloneOutboundSegmentData(segment.Data),
		}, true
	default:
		return oneBotMessageSegment{}, false
	}
}

func cloneOutboundSegmentData(data map[string]any) map[string]any {
	if len(data) == 0 {
		return map[string]any{}
	}
	cloned := make(map[string]any, len(data))
	for key, value := range data {
		cloned[key] = value
	}
	return cloned
}

func outboundSegmentString(data map[string]any, key string) (string, bool) {
	if len(data) == 0 {
		return "", false
	}
	value, ok := data[key]
	if !ok {
		return "", false
	}
	text, ok := value.(string)
	if !ok {
		return "", false
	}
	return text, true
}

func (s *Shell) sendSegments(ctx context.Context, targetType, targetID string, segments []oneBotMessageSegment, replyAttempt bool) (SendMessageResult, error) {
	echo := s.nextRequestEcho()
	request := sendMsgRequest{
		Action: "send_msg",
		Params: sendMsgParams{
			MessageType: targetType,
			Message:     segments,
		},
		Echo: echo,
	}
	switch targetType {
	case "group":
		request.Params.GroupID = oneBotTargetValue(targetID)
	case "private":
		request.Params.UserID = oneBotTargetValue(targetID)
	}

	conn, _, snapshot := s.currentWSConn()
	if conn != nil && snapshot.State == StateConnected {
		responseCh := make(chan apiResponse, 1)
		s.registerPendingResponse(echo, responseCh)
		defer s.dropPendingResponse(echo)

		s.sendMu.Lock()
		writeErr := wsjsonWrite(ctx, conn, request)
		s.sendMu.Unlock()
		if writeErr != nil {
			return SendMessageResult{}, errorf(errorCodeSendFailed, "write send_msg request", writeErr)
		}

		select {
		case response := <-responseCh:
			return parseSendMessageResponse(response, replyAttempt)
		case <-ctx.Done():
			return SendMessageResult{}, errorf(errorCodeSendFailed, "adapter send_msg response timed out", ctx.Err())
		}
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
	response, err := s.doHTTPAPIRequest(ctx, apiCallRequest{
		Action: request.Action,
		Params: params,
		Echo:   request.Echo,
	})
	if err != nil {
		return SendMessageResult{}, err
	}
	return parseSendMessageResponse(response, replyAttempt)
}

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

func oneBotTargetValue(targetID string) any {
	if value, err := strconv.ParseInt(targetID, 10, 64); err == nil {
		return value
	}
	return targetID
}
