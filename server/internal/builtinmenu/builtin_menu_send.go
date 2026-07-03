package menu

import (
	"context"
	"strings"
	"time"

	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/intake"
	adapteroutbound "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/outbound"
)

func (s *Service) sendBuiltinMenuImage(ctx context.Context, event adapterintake.NormalizedEvent, commandName string, imagePath string) {
	segments := []adapteroutbound.OutboundMessageSegment{{
		Type: "image",
		Data: map[string]any{"file": imagePath},
	}}
	s.sendBuiltinMenuSegments(ctx, event, commandName, segments)
}

func (s *Service) sendBuiltinMenuText(ctx context.Context, event adapterintake.NormalizedEvent, commandName string, text string) {
	segments := []adapteroutbound.OutboundMessageSegment{{
		Type: "text",
		Data: map[string]any{"text": text},
	}}
	s.sendBuiltinMenuSegments(ctx, event, commandName, segments)
}

func (s *Service) sendBuiltinMenuSegments(ctx context.Context, event adapterintake.NormalizedEvent, commandName string, segments []adapteroutbound.OutboundMessageSegment) {
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
		result, err := s.sender.SendReply(sendCtx, adapteroutbound.OutboundMessageReply{
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
	result, err := s.sender.SendMessage(sendCtx, adapteroutbound.OutboundMessageSend{
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
