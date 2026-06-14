package app

import (
	"context"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/outbound"
)

const (
	cooldownReplyText = "命令触发冷却，请稍后再试。"
)

func (s *eventIngressService) sendCooldownReply(ctx context.Context, event adapter.NormalizedEvent) {
	if s == nil || s.outboundSender == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}

	var (
		attempt outbound.SendAttempt
		result  outbound.SendResult
		err     error
	)

	switch strings.TrimSpace(event.ConversationType) {
	case "group":
		if messageID := strings.TrimSpace(event.MessageID); messageID != "" {
			segments := []adapter.OutboundMessageSegment{{
				Type: "text",
				Data: map[string]any{"text": cooldownReplyText},
			}}
			attempt = outbound.SendAttempt{
				ActionKind: "message.reply",
				TargetType: "group",
				TargetID:   strings.TrimSpace(event.ConversationID),
				Segments:   segments,
			}
			result = outbound.SendResult{
				DeliveryKind: "message.reply",
				TargetType:   "group",
				TargetID:     strings.TrimSpace(event.ConversationID),
			}
			if limitErr := s.waitOutboundLimit(ctx, outbound.MessageLimitRequest{
				TargetType: result.TargetType,
				TargetID:   result.TargetID,
			}); limitErr != nil {
				err = limitErr
				break
			}
			sendCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			sendResult, sendErr := s.outboundSender.SendReply(sendCtx, adapter.OutboundMessageReply{
				TargetType:       "group",
				TargetID:         strings.TrimSpace(event.ConversationID),
				ReplyToMessageID: messageID,
				Segments:         segments,
			})
			cancel()
			result.MessageID = sendResult.MessageID
			err = sendErr
			break
		}
		fallthrough
	case "private":
		if targetID := strings.TrimSpace(event.ConversationID); targetID != "" {
			segments := []adapter.OutboundMessageSegment{{
				Type: "text",
				Data: map[string]any{"text": cooldownReplyText},
			}}
			attempt = outbound.SendAttempt{
				ActionKind: "message.send",
				TargetType: strings.TrimSpace(event.ConversationType),
				TargetID:   targetID,
				Segments:   segments,
			}
			result = outbound.SendResult{
				DeliveryKind: "message.send",
				TargetType:   strings.TrimSpace(event.ConversationType),
				TargetID:     targetID,
			}
			if limitErr := s.waitOutboundLimit(ctx, outbound.MessageLimitRequest{
				TargetType: result.TargetType,
				TargetID:   result.TargetID,
			}); limitErr != nil {
				err = limitErr
				break
			}
			sendCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			sendResult, sendErr := s.outboundSender.SendMessage(sendCtx, adapter.OutboundMessageSend{
				TargetType: strings.TrimSpace(event.ConversationType),
				TargetID:   targetID,
				Segments:   segments,
			})
			cancel()
			result.MessageID = sendResult.MessageID
			err = sendErr
		}
	default:
		return
	}

	if s.state != nil && s.state.Logger != nil && strings.TrimSpace(attempt.ActionKind) != "" {
		outbound.LogSendOutcome(s.state.Logger, outbound.SendLogContext{
			TargetLabel: buildCooldownTargetLabel(ctx, event, s.outboundSender),
		}, attempt, result, err)
	}
}

func (s *eventIngressService) waitOutboundLimit(ctx context.Context, request outbound.MessageLimitRequest) error {
	if s == nil || s.outboundLimiter == nil {
		return nil
	}
	return s.outboundLimiter.Wait(ctx, request)
}

func buildCooldownTargetLabel(ctx context.Context, event adapter.NormalizedEvent, sender outboundActionSender) string {
	targetType := strings.TrimSpace(event.ConversationType)
	targetID := strings.TrimSpace(event.ConversationID)
	targetName := ""
	actorID := ""
	actorNickname := ""

	switch targetType {
	case "group":
		targetName = strings.TrimSpace(event.TargetName)
	case "private":
		actorID = strings.TrimSpace(event.SenderID)
		actorNickname = strings.TrimSpace(event.ActorNickname)
	}

	var resolver outbound.TargetDisplayResolver
	if candidate, ok := any(sender).(outbound.TargetDisplayResolver); ok {
		resolver = candidate
	}

	return outbound.BuildTargetLabel(ctx, targetType, targetID, targetName, actorID, actorNickname, resolver)
}
