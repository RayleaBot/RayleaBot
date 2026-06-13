package adapter

import "fmt"

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
