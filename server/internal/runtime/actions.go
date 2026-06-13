package runtime

import (
	"encoding/json"
	"strings"
)

func parseMessageSendAction(raw json.RawMessage) (*Action, error) {
	var frame protocolActionMessageSendFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		return nil, errorf(codePluginProtocolViolation, "plugin returned malformed message.send data", err)
	}

	targetType, targetID, err := validateActionTarget(frame.TargetType, frame.TargetID, "message.send")
	if err != nil {
		return nil, err
	}

	if frame.Message == nil {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required message.send fields", nil)
	}
	segments, err := parseOutboundActionSegments(frame.Message.Segments)
	if err != nil {
		return nil, err
	}
	return &Action{
		Kind:            "message.send",
		TargetType:      targetType,
		TargetID:        targetID,
		MessageSegments: segments,
	}, nil
}

func parseMessageReplyAction(raw json.RawMessage) (*Action, error) {
	var frame protocolActionMessageReplyFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		return nil, errorf(codePluginProtocolViolation, "plugin returned malformed message.reply data", err)
	}

	if frame.ReplyToEventID == nil || frame.Message == nil {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required message.reply fields", nil)
	}
	replyToEventID := strings.TrimSpace(*frame.ReplyToEventID)
	if replyToEventID == "" {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required message.reply fields", nil)
	}
	segments, err := parseOutboundActionSegments(frame.Message.Segments)
	if err != nil {
		return nil, err
	}
	return &Action{
		Kind:                    "message.reply",
		ReplyToEventID:          replyToEventID,
		FallbackToSendIfMissing: frame.FallbackToSendIfMissing,
		MessageSegments:         segments,
	}, nil
}
