package chatpolicy

import (
	"context"
	"strings"
	"time"

	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/adapter/intake"
	adapteroutbound "github.com/RayleaBot/RayleaBot/server/internal/adapter/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/outbound"
)

const (
	CooldownReplyText = "命令触发冷却，请稍后再试。"
)

func (s *Service) sendCooldownReply(ctx context.Context, event adapterintake.NormalizedEvent) {
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
			segments := []adapteroutbound.OutboundMessageSegment{{
				Type: "text",
				Data: map[string]any{"text": CooldownReplyText},
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
			sendResult, sendErr := s.outboundSender.SendReply(sendCtx, adapteroutbound.OutboundMessageReply{
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
			segments := []adapteroutbound.OutboundMessageSegment{{
				Type: "text",
				Data: map[string]any{"text": CooldownReplyText},
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
			sendResult, sendErr := s.outboundSender.SendMessage(sendCtx, adapteroutbound.OutboundMessageSend{
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

	if s.logger != nil && strings.TrimSpace(attempt.ActionKind) != "" {
		outbound.LogSendOutcome(s.logger, outbound.SendLogContext{
			TargetLabel: buildCooldownTargetLabel(ctx, event, s.outboundSender),
		}, attempt, result, err)
	}
}

func (s *Service) waitOutboundLimit(ctx context.Context, request outbound.MessageLimitRequest) error {
	if s == nil || s.outboundLimiter == nil {
		return nil
	}
	return s.outboundLimiter.Wait(ctx, request)
}

func buildCooldownTargetLabel(ctx context.Context, event adapterintake.NormalizedEvent, sender OutboundSender) string {
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
