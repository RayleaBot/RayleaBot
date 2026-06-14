package outbound

import "fmt"

const ErrorCodeSendFailed = "adapter.send_failed"
const ErrorCodeReplyTargetMissing = "adapter.reply_target_missing"

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

func Errorf(code, message string, err error) *Error {
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

type APICallRequest struct {
	Action string         `json:"action"`
	Params map[string]any `json:"params,omitempty"`
	Echo   string         `json:"echo"`
}

type SendMsgRequest struct {
	Action string        `json:"action"`
	Params SendMsgParams `json:"params"`
	Echo   string        `json:"echo"`
}

type SendMsgParams struct {
	MessageType string `json:"message_type"`
	UserID      any    `json:"user_id,omitempty"`
	GroupID     any    `json:"group_id,omitempty"`
	Message     any    `json:"message"`
}

type OneBotMessageSegment struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data,omitempty"`
}

type APIResponse struct {
	Echo    string
	Status  string
	RetCode int
	Wording string
	Data    any
}
