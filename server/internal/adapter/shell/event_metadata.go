package shell

import (
	"context"
	"strings"
	"time"
)

const (
	defaultIdentityCacheTTL      = 5 * time.Minute
	defaultIdentityLookupTimeout = 1500 * time.Millisecond
)

func (s *Shell) EnrichEventMetadata(ctx context.Context, event NormalizedEvent) NormalizedEvent {
	if s == nil || strings.TrimSpace(event.SourceProtocol) != "onebot11" {
		return event
	}
	s.invalidateIdentityCacheForEvent(event)
	if !isMessageEventType(event.EventType) {
		return event
	}

	enriched := cloneNormalizedEvent(event)
	sender := unifiedSenderPayload(enriched.PayloadFields)

	switch strings.TrimSpace(enriched.ConversationType) {
	case "group":
		groupID := strings.TrimSpace(enriched.ConversationID)
		if groupName := groupNameFromPayload(enriched.PayloadFields); groupID != "" && groupName != "" {
			enriched.TargetName = groupName
			if cache := s.currentIdentityCache(); cache != nil {
				cache.SetGroupInfo(groupID, GroupInfo{Name: groupName})
			}
		}
		if strings.TrimSpace(enriched.TargetName) == "" {
			if groupName := s.resolveGroupName(ctx, groupID); groupName != "" {
				enriched.TargetName = groupName
			}
		}

		if groupID != "" && strings.TrimSpace(enriched.SenderID) != "" && senderDisplayName(sender) == "" {
			if info := s.resolveGroupMemberInfo(ctx, groupID, enriched.SenderID); hasGroupMemberInfo(info) {
				mergeGroupMemberInfo(sender, info)
			}
		}
	case "private":
		if strings.TrimSpace(enriched.SenderID) != "" && senderDisplayName(sender) == "" {
			if info := s.resolveStrangerInfo(ctx, enriched.SenderID); strings.TrimSpace(info.Nickname) != "" {
				mergeStrangerInfo(sender, info)
			}
		}
	}

	syncSenderPayload(enriched.PayloadFields, sender)
	if actorNickname := senderPrimaryName(sender); actorNickname != "" {
		enriched.ActorNickname = actorNickname
	}
	if actorRole := payloadStringValue(sender["role"]); actorRole != "" {
		enriched.ActorRole = actorRole
	}

	return enriched
}

func isMessageEventType(eventType string) bool {
	switch strings.TrimSpace(eventType) {
	case "message.group", "message.private", "message_sent.group", "message_sent.private":
		return true
	default:
		return false
	}
}
