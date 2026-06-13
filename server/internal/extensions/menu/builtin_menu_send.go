package menu

import (
	"context"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/outbound"
)

func (s *Service) sendBuiltinMenuImage(ctx context.Context, event adapter.NormalizedEvent, commandName string, imagePath string) {
	segments := []adapter.OutboundMessageSegment{{
		Type: "image",
		Data: map[string]any{"file": imagePath},
	}}
	s.sendBuiltinMenuSegments(ctx, event, commandName, segments)
}

func (s *Service) sendBuiltinMenuText(ctx context.Context, event adapter.NormalizedEvent, commandName string, text string) {
	segments := []adapter.OutboundMessageSegment{{
		Type: "text",
		Data: map[string]any{"text": text},
	}}
	s.sendBuiltinMenuSegments(ctx, event, commandName, segments)
}

func (s *Service) sendBuiltinMenuSegments(ctx context.Context, event adapter.NormalizedEvent, commandName string, segments []adapter.OutboundMessageSegment) {
	if s == nil || s.sender == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	targetType := strings.TrimSpace(event.ConversationType)
	targetID := strings.TrimSpace(event.ConversationID)
	if targetID == "" {
		return
	}
	if targetType != "group" && targetType != "private" {
		return
	}
	label := s.builtinMenuTargetLabel(ctx, event)
	commandName = strings.TrimSpace(commandName)
	attempt := outbound.SendAttempt{
		ActionKind: "message.reply",
		TargetType: targetType,
		TargetID:   targetID,
		Segments:   segments,
	}
	if targetType != "group" || strings.TrimSpace(event.MessageID) == "" {
		attempt.ActionKind = "message.send"
	}
	logOutcome := func(result outbound.SendResult, err error) {
		if s.logger == nil {
			return
		}
		outbound.LogSendOutcome(s.logger, outbound.SendLogContext{
			TargetLabel: label,
			CommandName: commandName,
		}, attempt, result, err)
	}
	if err := s.waitLimit(ctx, outbound.MessageLimitRequest{
		TargetType: targetType,
		TargetID:   targetID,
	}); err != nil {
		s.logBuiltinMenuError(err)
		logOutcome(outbound.SendResult{
			DeliveryKind: strings.TrimSpace(attempt.ActionKind),
			TargetType:   targetType,
			TargetID:     targetID,
		}, err)
		return
	}
	sendCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if targetType == "group" && strings.TrimSpace(event.MessageID) != "" {
		result, err := s.sender.SendReply(sendCtx, adapter.OutboundMessageReply{
			TargetType:       targetType,
			TargetID:         targetID,
			ReplyToMessageID: strings.TrimSpace(event.MessageID),
			Segments:         segments,
		})
		s.logBuiltinMenuError(err)
		logOutcome(outbound.SendResult{
			MessageID:    result.MessageID,
			DeliveryKind: "message.reply",
			TargetType:   targetType,
			TargetID:     targetID,
		}, err)
		return
	}
	result, err := s.sender.SendMessage(sendCtx, adapter.OutboundMessageSend{
		TargetType: targetType,
		TargetID:   targetID,
		Segments:   segments,
	})
	s.logBuiltinMenuError(err)
	logOutcome(outbound.SendResult{
		MessageID:    result.MessageID,
		DeliveryKind: "message.send",
		TargetType:   targetType,
		TargetID:     targetID,
	}, err)
}
