package adapter

import (
	"context"
	"strconv"
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

func (s *Shell) invalidateIdentityCacheForEvent(event NormalizedEvent) {
	cache := s.currentIdentityCache()
	if cache == nil {
		return
	}

	groupID := strings.TrimSpace(event.ConversationID)
	userID := strings.TrimSpace(event.SenderID)

	switch strings.TrimSpace(event.EventType) {
	case "notice.group_card", "notice.group_title":
		if groupID != "" && userID != "" {
			cache.InvalidateGroupMemberInfo(groupID, userID)
		}
	case "notice.group_admin", "notice.member_decrease":
		if groupID != "" {
			cache.InvalidateGroupMembers(groupID)
		}
	case "notice.member_increase":
		if groupID != "" && userID != "" {
			cache.InvalidateGroupMemberInfo(groupID, userID)
		}
	case "notice.group_name", "notice.group_profile":
		if groupID != "" {
			cache.InvalidateGroupInfo(groupID)
		}
	}

	onebot := cloneOptionalMap(event.PayloadFields["onebot"])
	noticeType := strings.TrimSpace(payloadStringValue(onebot["notice_type"]))
	if noticeType == "" {
		noticeType = strings.TrimSpace(payloadStringValue(event.PayloadFields["notice_type"]))
	}
	subType := strings.TrimSpace(payloadStringValue(onebot["sub_type"]))
	if subType == "" {
		subType = strings.TrimSpace(payloadStringValue(event.PayloadFields["sub_type"]))
	}
	switch noticeType {
	case "group_name", "group_name_change", "group_profile":
		if groupID != "" {
			cache.InvalidateGroupInfo(groupID)
		}
	case "notify":
		switch subType {
		case "group_name", "group_name_change", "group_profile":
			if groupID != "" {
				cache.InvalidateGroupInfo(groupID)
			}
		}
	case "group_card", "group_title":
		if groupID != "" && userID != "" {
			cache.InvalidateGroupMemberInfo(groupID, userID)
		}
	}
}

func (s *Shell) invalidateIdentityCacheForFrame(frame oneBotFrame) {
	if strings.TrimSpace(frame.PostType) != "notice" {
		return
	}

	cache := s.currentIdentityCache()
	if cache == nil {
		return
	}

	groupID := positiveIDString(frame.GroupID)
	userID := positiveIDString(frame.UserID)

	switch strings.TrimSpace(frame.NoticeType) {
	case "group_name", "group_name_change", "group_profile":
		if groupID != "" {
			cache.InvalidateGroupInfo(groupID)
		}
	case "notify":
		switch strings.TrimSpace(frame.SubType) {
		case "group_name", "group_name_change", "group_profile":
			if groupID != "" {
				cache.InvalidateGroupInfo(groupID)
			}
		}
	case "group_card", "group_title":
		if groupID != "" && userID != "" {
			cache.InvalidateGroupMemberInfo(groupID, userID)
		}
	case "group_admin", "group_decrease":
		if groupID != "" {
			cache.InvalidateGroupMembers(groupID)
		}
	case "group_increase":
		if groupID != "" && userID != "" {
			cache.InvalidateGroupMemberInfo(groupID, userID)
		}
	}
}

func (s *Shell) invalidateIdentityCacheForAPICall(action string, params map[string]any) {
	cache := s.currentIdentityCache()
	if cache == nil {
		return
	}

	groupID := strings.TrimSpace(payloadStringValue(params["group_id"]))
	userID := strings.TrimSpace(payloadStringValue(params["user_id"]))

	switch strings.TrimSpace(action) {
	case "set_group_name":
		if groupID != "" {
			cache.InvalidateGroupInfo(groupID)
		}
	case "set_group_card", "set_group_special_title":
		if groupID != "" && userID != "" {
			cache.InvalidateGroupMemberInfo(groupID, userID)
		}
	case "set_group_admin":
		if groupID != "" {
			cache.InvalidateGroupMembers(groupID)
		}
	}
}

func positiveIDString(value int64) string {
	if value <= 0 {
		return ""
	}
	return strconv.FormatInt(value, 10)
}

func isMessageEventType(eventType string) bool {
	switch strings.TrimSpace(eventType) {
	case "message.group", "message.private", "message_sent.group", "message_sent.private":
		return true
	default:
		return false
	}
}

func (s *Shell) resolveGroupName(ctx context.Context, groupID string) string {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return ""
	}

	if cache := s.currentIdentityCache(); cache != nil {
		if info, ok := cache.GetGroupInfo(groupID); ok && strings.TrimSpace(info.Name) != "" {
			return info.Name
		}
	}

	lookupCtx, cancel := withIdentityLookupTimeout(ctx)
	defer cancel()

	info, err := s.GetGroupInfo(lookupCtx, groupID)
	if err != nil || strings.TrimSpace(info.Name) == "" {
		return ""
	}

	if cache := s.currentIdentityCache(); cache != nil {
		cache.SetGroupInfo(groupID, info)
	}
	return info.Name
}

func (s *Shell) resolveGroupMemberInfo(ctx context.Context, groupID, userID string) GroupMemberInfo {
	groupID = strings.TrimSpace(groupID)
	userID = strings.TrimSpace(userID)
	if groupID == "" || userID == "" {
		return GroupMemberInfo{}
	}

	if cache := s.currentIdentityCache(); cache != nil {
		if info, ok := cache.GetGroupMemberInfo(groupID, userID); ok {
			return info
		}
	}

	lookupCtx, cancel := withIdentityLookupTimeout(ctx)
	defer cancel()

	info, err := s.GetGroupMemberInfo(lookupCtx, groupID, userID)
	if err != nil || !hasGroupMemberInfo(info) {
		return GroupMemberInfo{}
	}

	if cache := s.currentIdentityCache(); cache != nil {
		cache.SetGroupMemberInfo(groupID, userID, info)
	}
	return info
}

func (s *Shell) resolveStrangerInfo(ctx context.Context, userID string) StrangerInfo {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return StrangerInfo{}
	}

	if cache := s.currentIdentityCache(); cache != nil {
		if info, ok := cache.GetStrangerInfo(userID); ok && strings.TrimSpace(info.Nickname) != "" {
			return info
		}
	}

	lookupCtx, cancel := withIdentityLookupTimeout(ctx)
	defer cancel()

	info, err := s.GetStrangerInfo(lookupCtx, userID)
	if err != nil || strings.TrimSpace(info.Nickname) == "" {
		return StrangerInfo{}
	}

	if cache := s.currentIdentityCache(); cache != nil {
		cache.SetStrangerInfo(userID, info)
	}
	return info
}

func (s *Shell) currentIdentityCache() *IdentityCache {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.identityCache
}

func withIdentityLookupTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, defaultIdentityLookupTimeout)
}

func hasGroupMemberInfo(info GroupMemberInfo) bool {
	return strings.TrimSpace(info.Card) != "" || strings.TrimSpace(info.Nickname) != "" || strings.TrimSpace(info.Role) != "" || strings.TrimSpace(info.Title) != ""
}

func groupNameFromPayload(payload map[string]any) string {
	if len(payload) == 0 {
		return ""
	}
	if groupName := payloadStringValue(payload["group_name"]); groupName != "" {
		return groupName
	}
	onebot := cloneOptionalMap(payload["onebot"])
	return payloadStringValue(onebot["group_name"])
}

func cloneNormalizedEvent(event NormalizedEvent) NormalizedEvent {
	cloned := event
	cloned.PayloadFields = cloneEventMap(event.PayloadFields)
	return cloned
}

func cloneEventMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return map[string]any{}
	}

	cloned := make(map[string]any, len(input))
	for key, value := range input {
		cloned[key] = cloneEventValue(value)
	}
	return cloned
}

func cloneEventValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneEventMap(typed)
	case []any:
		items := make([]any, 0, len(typed))
		for _, item := range typed {
			items = append(items, cloneEventValue(item))
		}
		return items
	default:
		return value
	}
}

func unifiedSenderPayload(payload map[string]any) map[string]any {
	if len(payload) == 0 {
		return map[string]any{}
	}

	sender := cloneOptionalMap(payload["sender"])
	onebot := cloneOptionalMap(payload["onebot"])
	onebotSender := cloneOptionalMap(onebot["sender"])

	if len(sender) == 0 {
		sender = onebotSender
	} else {
		mergeSenderFields(sender, onebotSender)
	}

	return sender
}

func cloneOptionalMap(value any) map[string]any {
	typed, _ := value.(map[string]any)
	return cloneEventMap(typed)
}

func mergeSenderFields(target map[string]any, source map[string]any) {
	if len(target) == 0 || len(source) == 0 {
		if len(target) == 0 && len(source) > 0 {
			for key, value := range source {
				target[key] = value
			}
		}
		return
	}

	for _, key := range []string{"user_id", "nickname", "card", "role", "title"} {
		if payloadStringValue(target[key]) != "" {
			continue
		}
		if payloadStringValue(source[key]) == "" {
			continue
		}
		target[key] = source[key]
	}
}

func syncSenderPayload(payload map[string]any, sender map[string]any) {
	if len(payload) == 0 || len(sender) == 0 {
		return
	}

	payload["sender"] = sender
	onebot := cloneOptionalMap(payload["onebot"])
	if len(onebot) == 0 {
		onebot = map[string]any{}
	}
	onebot["sender"] = sender
	payload["onebot"] = onebot
}

func mergeGroupMemberInfo(sender map[string]any, info GroupMemberInfo) {
	if payloadStringValue(sender["card"]) == "" && strings.TrimSpace(info.Card) != "" {
		sender["card"] = info.Card
	}
	if payloadStringValue(sender["nickname"]) == "" && strings.TrimSpace(info.Nickname) != "" {
		sender["nickname"] = info.Nickname
	}
	if payloadStringValue(sender["role"]) == "" && strings.TrimSpace(info.Role) != "" {
		sender["role"] = info.Role
	}
	if payloadStringValue(sender["title"]) == "" && strings.TrimSpace(info.Title) != "" {
		sender["title"] = info.Title
	}
}

func mergeStrangerInfo(sender map[string]any, info StrangerInfo) {
	if payloadStringValue(sender["nickname"]) == "" && strings.TrimSpace(info.Nickname) != "" {
		sender["nickname"] = info.Nickname
	}
}

func senderDisplayName(sender map[string]any) string {
	card := payloadStringValue(sender["card"])
	nickname := payloadStringValue(sender["nickname"])

	switch {
	case card != "" && nickname != "" && card != nickname:
		return card + "/" + nickname
	case card != "":
		return card
	case nickname != "":
		return nickname
	default:
		return ""
	}
}

func senderPrimaryName(sender map[string]any) string {
	card := payloadStringValue(sender["card"])
	if card != "" {
		return card
	}
	return payloadStringValue(sender["nickname"])
}

func payloadStringValue(value any) string {
	if value == nil {
		return ""
	}
	valueString := strings.TrimSpace(extractStringValue(value))
	if valueString == "<nil>" {
		return ""
	}
	return valueString
}
