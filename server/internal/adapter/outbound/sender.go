package outbound

import (
	"context"
	"strings"
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
