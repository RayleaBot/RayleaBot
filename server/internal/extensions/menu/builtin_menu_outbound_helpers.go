package menu

import (
	"context"
	"strings"

	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/intake"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/outbound"
)

func (s *Service) waitLimit(ctx context.Context, request outbound.MessageLimitRequest) error {
	if s == nil || s.waitOutbound == nil {
		return nil
	}
	return s.waitOutbound(ctx, request)
}

func (s *Service) logBuiltinMenuError(err error) {
	if err == nil || s == nil || s.logger == nil {
		return
	}
	s.logger.Warn("builtin menu response failed", "component", "app", "error", err)
}

func (s *Service) logBuiltinMenuTrigger(_ context.Context, event adapterintake.NormalizedEvent, request Request) {
	if s == nil || s.logger == nil {
		return
	}
	summary, ok := logging.OneBotInboundMessageSummary(logging.OneBotInboundMessageSummaryInput{
		SourceProtocol:   event.SourceProtocol,
		BotID:            event.BotID,
		EventType:        event.EventType,
		ConversationType: event.ConversationType,
		ConversationID:   event.ConversationID,
		SenderID:         event.SenderID,
		TargetName:       event.TargetName,
		ActorNickname:    event.ActorNickname,
		PlainText:        event.PlainText,
		PayloadFields:    event.PayloadFields,
	})
	if !ok {
		summary = "builtin menu command received"
	}
	fields := []any{
		"component", "bridge",
		"protocol", logging.ProtocolOneBot11,
		"event_id", strings.TrimSpace(event.EventID),
		"command_name", strings.TrimSpace(request.Command),
		"target_type", strings.TrimSpace(event.ConversationType),
		"target_id", strings.TrimSpace(event.ConversationID),
		"sender_id", strings.TrimSpace(event.SenderID),
		"plain_text", strings.TrimSpace(event.PlainText),
		"builtin_menu", true,
	}
	s.logger.Info(summary, fields...)
}

func (s *Service) builtinMenuTargetLabel(ctx context.Context, event adapterintake.NormalizedEvent) string {
	targetType := strings.TrimSpace(event.ConversationType)
	targetID := strings.TrimSpace(event.ConversationID)
	targetName := strings.TrimSpace(event.TargetName)
	actorID := strings.TrimSpace(event.SenderID)
	actorNickname := strings.TrimSpace(event.ActorNickname)
	var resolver outbound.TargetDisplayResolver
	if candidate, ok := any(s.sender).(outbound.TargetDisplayResolver); ok {
		resolver = candidate
	}
	return outbound.BuildTargetLabel(ctx, targetType, targetID, targetName, actorID, actorNickname, resolver)
}
