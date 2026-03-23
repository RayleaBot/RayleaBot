package outbound

import (
	"context"
	"errors"
	"strings"

	"rayleabot/server/internal/adapter"
	"rayleabot/server/internal/runtime"
)

const (
	codeAdapterReplyTargetMissing = "adapter.reply_target_missing"
	codeAdapterSendFailed         = "adapter.send_failed"
	codePluginProtocolViolation   = "plugin.protocol_violation"
)

type ActionSender interface {
	SendMessage(context.Context, adapter.OutboundMessageSend) (adapter.SendMessageResult, error)
	SendReply(context.Context, adapter.OutboundMessageReply) (adapter.SendMessageResult, error)
}

type ReplyTarget struct {
	MessageID  string
	TargetType string
	TargetID   string
}

type ReplyTargetResolver interface {
	ResolveReplyTarget(eventID string) (ReplyTarget, bool)
}

func SendAction(ctx context.Context, sender ActionSender, resolver ReplyTargetResolver, origin runtime.Event, action runtime.Action) (adapter.SendMessageResult, error) {
	if sender == nil {
		return adapter.SendMessageResult{}, &adapter.Error{
			Code:    codeAdapterSendFailed,
			Message: "adapter outbound sender is not available",
		}
	}

	switch action.Kind {
	case "message.send":
		return sender.SendMessage(ctx, adapter.OutboundMessageSend{
			TargetType: action.TargetType,
			TargetID:   action.TargetID,
			Segments:   toAdapterSegments(action.MessageSegments),
		})
	case "message.reply":
		return sendReplyAction(ctx, sender, resolver, origin, action)
	default:
		return adapter.SendMessageResult{}, &adapter.Error{
			Code:    codePluginProtocolViolation,
			Message: "received unsupported outbound action kind",
		}
	}
}

func sendReplyAction(ctx context.Context, sender ActionSender, resolver ReplyTargetResolver, _ runtime.Event, action runtime.Action) (adapter.SendMessageResult, error) {
	replyTarget, ok := resolveReplyTarget(action, resolver)
	if !ok {
		return adapter.SendMessageResult{}, &adapter.Error{
			Code:    codeAdapterReplyTargetMissing,
			Message: "reply target is not available in the current event window",
		}
	}

	replyRequest := adapter.OutboundMessageReply{
		TargetType:       replyTarget.TargetType,
		TargetID:         replyTarget.TargetID,
		ReplyToMessageID: replyTarget.MessageID,
		Segments:         toAdapterSegments(action.MessageSegments),
	}
	result, err := sender.SendReply(ctx, replyRequest)
	if err == nil {
		return result, nil
	}

	var adapterErr *adapter.Error
	if !action.FallbackToSendIfMissing || !errors.As(err, &adapterErr) || adapterErr.Code != codeAdapterReplyTargetMissing {
		return adapter.SendMessageResult{}, err
	}

	return sender.SendMessage(ctx, adapter.OutboundMessageSend{
		TargetType: replyTarget.TargetType,
		TargetID:   replyTarget.TargetID,
		Segments:   stripReplySegments(toAdapterSegments(action.MessageSegments)),
	})
}

func resolveReplyTarget(action runtime.Action, resolver ReplyTargetResolver) (ReplyTarget, bool) {
	replyToEventID := strings.TrimSpace(action.ReplyToEventID)
	if replyToEventID == "" || resolver == nil {
		return ReplyTarget{}, false
	}
	target, ok := resolver.ResolveReplyTarget(replyToEventID)
	if !ok {
		return ReplyTarget{}, false
	}
	return target, target.MessageID != "" && target.TargetType != "" && target.TargetID != ""
}

func toAdapterSegments(segments []runtime.ActionSegment) []adapter.OutboundMessageSegment {
	if len(segments) == 0 {
		return nil
	}
	items := make([]adapter.OutboundMessageSegment, 0, len(segments))
	for _, segment := range segments {
		data := make(map[string]any, len(segment.Data))
		for key, value := range segment.Data {
			data[key] = value
		}
		items = append(items, adapter.OutboundMessageSegment{
			Type: segment.Type,
			Data: data,
		})
	}
	return items
}

func stripReplySegments(segments []adapter.OutboundMessageSegment) []adapter.OutboundMessageSegment {
	if len(segments) == 0 {
		return nil
	}
	items := make([]adapter.OutboundMessageSegment, 0, len(segments))
	for _, segment := range segments {
		if strings.TrimSpace(segment.Type) == "reply" {
			continue
		}
		items = append(items, segment)
	}
	return items
}
