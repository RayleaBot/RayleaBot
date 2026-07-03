package outbound

import (
	"context"
	"errors"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/onebot11"
	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/action"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
)

const (
	codeAdapterReplyTargetMissing = "adapter.reply_target_missing"
	codeAdapterSendFailed         = "adapter.send_failed"
	codePluginProtocolViolation   = "plugin.protocol_violation"
)

type ActionSender interface {
	SendMessage(context.Context, onebot11.OutboundMessageSend) (onebot11.SendMessageResult, error)
	SendReply(context.Context, onebot11.OutboundMessageReply) (onebot11.SendMessageResult, error)
}

type ReplyTarget struct {
	MessageID  string
	TargetType string
	TargetID   string
}

type SendResult struct {
	MessageID    string
	DeliveryKind string
	TargetType   string
	TargetID     string
}

type ReplyTargetResolver interface {
	ResolveReplyTarget(eventID string) (ReplyTarget, bool)
}

func SendAction(ctx context.Context, sender ActionSender, resolver ReplyTargetResolver, origin runtimeprotocol.Event, action runtimeaction.Action) (SendResult, error) {
	if sender == nil {
		return SendResult{DeliveryKind: action.Kind}, &onebot11.Error{
			Code:    codeAdapterSendFailed,
			Message: "adapter outbound sender is not available",
		}
	}

	switch action.Kind {
	case "message.send":
		result, err := sender.SendMessage(ctx, onebot11.OutboundMessageSend{
			TargetType: action.TargetType,
			TargetID:   action.TargetID,
			Segments:   toAdapterSegments(action.MessageSegments),
		})
		return SendResult{
			MessageID:    result.MessageID,
			DeliveryKind: "message.send",
			TargetType:   action.TargetType,
			TargetID:     action.TargetID,
		}, err
	case "message.reply":
		return sendReplyAction(ctx, sender, resolver, origin, action)
	default:
		return SendResult{DeliveryKind: action.Kind}, &onebot11.Error{
			Code:    codePluginProtocolViolation,
			Message: "received unsupported outbound action kind",
		}
	}
}

func sendReplyAction(ctx context.Context, sender ActionSender, resolver ReplyTargetResolver, _ runtimeprotocol.Event, action runtimeaction.Action) (SendResult, error) {
	replyTarget, ok := resolveReplyTarget(action, resolver)
	if !ok {
		return SendResult{DeliveryKind: "message.reply"}, &onebot11.Error{
			Code:    codeAdapterReplyTargetMissing,
			Message: "reply target is not available in the current event window",
		}
	}

	replyRequest := onebot11.OutboundMessageReply{
		TargetType:       replyTarget.TargetType,
		TargetID:         replyTarget.TargetID,
		ReplyToMessageID: replyTarget.MessageID,
		Segments:         toAdapterSegments(action.MessageSegments),
	}
	result, err := sender.SendReply(ctx, replyRequest)
	if err == nil {
		return SendResult{
			MessageID:    result.MessageID,
			DeliveryKind: "message.reply",
			TargetType:   replyTarget.TargetType,
			TargetID:     replyTarget.TargetID,
		}, nil
	}

	var adapterErr *onebot11.Error
	if !action.FallbackToSendIfMissing || !errors.As(err, &adapterErr) || adapterErr.Code != codeAdapterReplyTargetMissing {
		return SendResult{
			DeliveryKind: "message.reply",
			TargetType:   replyTarget.TargetType,
			TargetID:     replyTarget.TargetID,
		}, err
	}

	fallbackResult, fallbackErr := sender.SendMessage(ctx, onebot11.OutboundMessageSend{
		TargetType: replyTarget.TargetType,
		TargetID:   replyTarget.TargetID,
		Segments:   stripReplySegments(toAdapterSegments(action.MessageSegments)),
	})
	return SendResult{
		MessageID:    fallbackResult.MessageID,
		DeliveryKind: "message.send",
		TargetType:   replyTarget.TargetType,
		TargetID:     replyTarget.TargetID,
	}, fallbackErr
}

func resolveReplyTarget(action runtimeaction.Action, resolver ReplyTargetResolver) (ReplyTarget, bool) {
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

func toAdapterSegments(segments []runtimeaction.ActionSegment) []onebot11.OutboundMessageSegment {
	if len(segments) == 0 {
		return nil
	}
	items := make([]onebot11.OutboundMessageSegment, 0, len(segments))
	for _, segment := range segments {
		data := make(map[string]any, len(segment.Data))
		for key, value := range segment.Data {
			data[key] = value
		}
		items = append(items, onebot11.OutboundMessageSegment{
			Type: segment.Type,
			Data: data,
		})
	}
	return items
}

func stripReplySegments(segments []onebot11.OutboundMessageSegment) []onebot11.OutboundMessageSegment {
	if len(segments) == 0 {
		return nil
	}
	items := make([]onebot11.OutboundMessageSegment, 0, len(segments))
	for _, segment := range segments {
		if strings.TrimSpace(segment.Type) == "reply" {
			continue
		}
		items = append(items, segment)
	}
	return items
}
