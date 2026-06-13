package adapter

import (
	"context"
	"strings"
)

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
