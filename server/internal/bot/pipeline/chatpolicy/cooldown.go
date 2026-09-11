package chatpolicy

import (
	"context"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/outbound"
)

const (
	CooldownReplyText = "命令触发冷却，请稍后再试。"
)

func (s *Service) sendCooldownReply(ctx context.Context, event chatevent.NormalizedEvent) {
	if s.outboundSender == nil {
		return
	}

	var (
		attempt outbound.SendAttempt
		result  outbound.SendResult
		err     error
	)

	targetType := strings.TrimSpace(event.ConversationType)
	switch targetType {
	case "group", "private":
		if messageID := strings.TrimSpace(event.MessageID); messageID != "" && (targetType == "group" || event.SourceProtocol == "qqofficial") {
			segments := []chatevent.MessageSegment{{
				Type: "text",
				Data: map[string]any{"text": CooldownReplyText},
			}}
			attempt = outbound.SendAttempt{
				SourceAdapter:  event.SourceAdapter,
				SourceProtocol: event.SourceProtocol,
				ActionKind:     "message.reply",
				TargetType:     targetType,
				TargetID:       strings.TrimSpace(event.ConversationID),
				Segments:       segments,
			}
			result = outbound.SendResult{
				DeliveryKind: "message.reply",
				TargetType:   targetType,
				TargetID:     strings.TrimSpace(event.ConversationID),
			}
			if limitErr := s.waitOutboundLimit(ctx, outbound.MessageLimitRequest{
				Scope:      event.IdentityScope(),
				TargetType: result.TargetType,
				TargetID:   result.TargetID,
			}); limitErr != nil {
				err = limitErr
				break
			}
			sendCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			sendResult, sendErr := s.outboundSender.SendReply(sendCtx, chatevent.OutboundMessageReply{
				SourceAdapter:    event.SourceAdapter,
				SourceProtocol:   event.SourceProtocol,
				TargetType:       targetType,
				TargetID:         strings.TrimSpace(event.ConversationID),
				ReplyToMessageID: messageID,
				Segments:         segments,
			})
			cancel()
			result.MessageID = sendResult.MessageID
			err = sendErr
			break
		}
		if targetID := strings.TrimSpace(event.ConversationID); targetID != "" {
			segments := []chatevent.MessageSegment{{
				Type: "text",
				Data: map[string]any{"text": CooldownReplyText},
			}}
			attempt = outbound.SendAttempt{
				SourceAdapter:  event.SourceAdapter,
				SourceProtocol: event.SourceProtocol,
				ActionKind:     "message.send",
				TargetType:     strings.TrimSpace(event.ConversationType),
				TargetID:       targetID,
				Segments:       segments,
			}
			result = outbound.SendResult{
				DeliveryKind: "message.send",
				TargetType:   strings.TrimSpace(event.ConversationType),
				TargetID:     targetID,
			}
			if limitErr := s.waitOutboundLimit(ctx, outbound.MessageLimitRequest{
				Scope:      event.IdentityScope(),
				TargetType: result.TargetType,
				TargetID:   result.TargetID,
			}); limitErr != nil {
				err = limitErr
				break
			}
			sendCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			sendResult, sendErr := s.outboundSender.SendMessage(sendCtx, chatevent.OutboundMessageSend{
				SourceAdapter:  event.SourceAdapter,
				SourceProtocol: event.SourceProtocol,
				TargetType:     strings.TrimSpace(event.ConversationType),
				TargetID:       targetID,
				Segments:       segments,
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
	if s.outboundLimiter == nil {
		return nil
	}
	return s.outboundLimiter.Wait(ctx, request)
}

func buildCooldownTargetLabel(ctx context.Context, event chatevent.NormalizedEvent, sender OutboundSender) string {
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

	return outbound.BuildTargetLabel(ctx, event.SourceAdapter, targetType, targetID, targetName, actorID, actorNickname, resolver)
}
