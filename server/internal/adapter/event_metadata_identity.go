package adapter

import (
	"context"
	"strings"
)

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
