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
	Text       string
}

type OutboundMessageReply struct {
	ReplyToMessageID string
	Text             string
}

type OutboundMessageSendImage struct {
	TargetType string
	TargetID   string
	File       string
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
	Message     string `json:"message"`
}

type apiResponse struct {
	Echo    string
	Status  string
	RetCode int
	Wording string
	Data    map[string]any
}

func (s *Shell) SendMessage(ctx context.Context, action OutboundMessageSend) (SendMessageResult, error) {
	targetType := strings.TrimSpace(action.TargetType)
	targetID := strings.TrimSpace(action.TargetID)
	text := strings.TrimSpace(action.Text)
	if targetID == "" || text == "" {
		return SendMessageResult{}, errorf(errorCodeSendFailed, "message.send action is missing required fields", nil)
	}
	switch targetType {
	case "group", "private":
	default:
		return SendMessageResult{}, errorf(errorCodeSendFailed, "message.send uses unsupported target_type", nil)
	}

	echo := s.nextRequestEcho()
	responseCh := make(chan apiResponse, 1)
	s.registerPendingResponse(echo, responseCh)
	defer s.dropPendingResponse(echo)

	request := sendMsgRequest{
		Action: "send_msg",
		Params: sendMsgParams{
			MessageType: targetType,
			Message:     text,
		},
		Echo: echo,
	}
	switch targetType {
	case "group":
		request.Params.GroupID = oneBotTargetValue(targetID)
	case "private":
		request.Params.UserID = oneBotTargetValue(targetID)
	}

	conn, snapshot := s.currentConn()
	if conn == nil || snapshot.State != StateConnected {
		return SendMessageResult{}, errorf(errorCodeConnectionLost, "adapter websocket is not connected", nil)
	}

	s.sendMu.Lock()
	writeErr := wsjsonWrite(ctx, conn, request)
	s.sendMu.Unlock()
	if writeErr != nil {
		return SendMessageResult{}, errorf(errorCodeSendFailed, "write send_msg request", writeErr)
	}

	select {
	case response := <-responseCh:
		return parseSendMessageResponse(response)
	case <-ctx.Done():
		return SendMessageResult{}, errorf(errorCodeSendFailed, "adapter send_msg response timed out", ctx.Err())
	}
}

// SendImage sends an image message via the OneBot11 adapter using the
// [CQ:image,file=<file>] segment format.
func (s *Shell) SendImage(ctx context.Context, action OutboundMessageSendImage) (SendMessageResult, error) {
	targetType := strings.TrimSpace(action.TargetType)
	targetID := strings.TrimSpace(action.TargetID)
	file := strings.TrimSpace(action.File)
	if targetID == "" || file == "" {
		return SendMessageResult{}, errorf(errorCodeSendFailed, "message.send_image action is missing required fields", nil)
	}
	switch targetType {
	case "group", "private":
	default:
		return SendMessageResult{}, errorf(errorCodeSendFailed, "message.send_image uses unsupported target_type", nil)
	}

	echo := s.nextRequestEcho()
	responseCh := make(chan apiResponse, 1)
	s.registerPendingResponse(echo, responseCh)
	defer s.dropPendingResponse(echo)

	cqMessage := fmt.Sprintf("[CQ:image,file=%s]", file)

	request := sendMsgRequest{
		Action: "send_msg",
		Params: sendMsgParams{
			MessageType: targetType,
			Message:     cqMessage,
		},
		Echo: echo,
	}
	switch targetType {
	case "group":
		request.Params.GroupID = oneBotTargetValue(targetID)
	case "private":
		request.Params.UserID = oneBotTargetValue(targetID)
	}

	conn, snapshot := s.currentConn()
	if conn == nil || snapshot.State != StateConnected {
		return SendMessageResult{}, errorf(errorCodeConnectionLost, "adapter websocket is not connected", nil)
	}

	s.sendMu.Lock()
	writeErr := wsjsonWrite(ctx, conn, request)
	s.sendMu.Unlock()
	if writeErr != nil {
		return SendMessageResult{}, errorf(errorCodeSendFailed, "write send_msg image request", writeErr)
	}

	select {
	case response := <-responseCh:
		return parseSendMessageResponse(response)
	case <-ctx.Done():
		return SendMessageResult{}, errorf(errorCodeSendFailed, "adapter send_msg image response timed out", ctx.Err())
	}
}

// SendReply sends a quote-reply message via the OneBot11 adapter using the
// [CQ:reply,id=<reply_to_message_id>] segment prepended to the text.
func (s *Shell) SendReply(ctx context.Context, action OutboundMessageReply) (SendMessageResult, error) {
	replyToID := strings.TrimSpace(action.ReplyToMessageID)
	text := strings.TrimSpace(action.Text)
	if replyToID == "" || text == "" {
		return SendMessageResult{}, errorf(errorCodeSendFailed, "message.reply action is missing required fields", nil)
	}

	echo := s.nextRequestEcho()
	responseCh := make(chan apiResponse, 1)
	s.registerPendingResponse(echo, responseCh)
	defer s.dropPendingResponse(echo)

	// OneBot11 quote-reply: prepend [CQ:reply,id=<id>] to the message text.
	cqMessage := fmt.Sprintf("[CQ:reply,id=%s]%s", replyToID, text)

	request := sendMsgRequest{
		Action: "send_msg",
		Params: sendMsgParams{
			MessageType: "group",
			Message:     cqMessage,
		},
		Echo: echo,
	}

	conn, snapshot := s.currentConn()
	if conn == nil || snapshot.State != StateConnected {
		return SendMessageResult{}, errorf(errorCodeConnectionLost, "adapter websocket is not connected", nil)
	}

	s.sendMu.Lock()
	writeErr := wsjsonWrite(ctx, conn, request)
	s.sendMu.Unlock()
	if writeErr != nil {
		return SendMessageResult{}, errorf(errorCodeSendFailed, "write send_msg reply request", writeErr)
	}

	select {
	case response := <-responseCh:
		return parseSendMessageResponse(response)
	case <-ctx.Done():
		return SendMessageResult{}, errorf(errorCodeSendFailed, "adapter send_msg reply response timed out", ctx.Err())
	}
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
			"echo", response.Echo,
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

func (s *Shell) currentConn() (*websocket.Conn, Snapshot) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.conn, cloneSnapshot(s.snapshot)
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

	data := frame.Data
	if data == nil {
		data = map[string]any{}
	}

	return apiResponse{
		Echo:    echo,
		Status:  strings.TrimSpace(frame.Status),
		RetCode: frame.RetCode,
		Wording: strings.TrimSpace(frame.Wording),
		Data:    data,
	}, true
}

func parseSendMessageResponse(response apiResponse) (SendMessageResult, error) {
	if response.Status != "ok" || response.RetCode != 0 {
		message := "adapter send_msg failed"
		if response.Wording != "" {
			message = response.Wording
		}
		return SendMessageResult{}, errorf(errorCodeSendFailed, message, nil)
	}

	return SendMessageResult{
		MessageID: extractMessageID(response.Data),
	}, nil
}

func extractMessageID(data map[string]any) string {
	if data == nil {
		return ""
	}

	switch value := data["message_id"].(type) {
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
