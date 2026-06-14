package shell

import (
	"context"
	"strings"

	adapterapi "github.com/RayleaBot/RayleaBot/server/internal/adapter/api"
	adaptercache "github.com/RayleaBot/RayleaBot/server/internal/adapter/cache"
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
		cache.SetGroupInfo(groupID, adaptercache.GroupInfo{Name: info.Name})
	}
	return info.Name
}

func (s *Shell) resolveGroupMemberInfo(ctx context.Context, groupID, userID string) adapterapi.GroupMemberInfo {
	groupID = strings.TrimSpace(groupID)
	userID = strings.TrimSpace(userID)
	if groupID == "" || userID == "" {
		return adapterapi.GroupMemberInfo{}
	}

	if cache := s.currentIdentityCache(); cache != nil {
		if info, ok := cache.GetGroupMemberInfo(groupID, userID); ok {
			return apiGroupMemberInfo(info)
		}
	}

	lookupCtx, cancel := withIdentityLookupTimeout(ctx)
	defer cancel()

	info, err := s.GetGroupMemberInfo(lookupCtx, groupID, userID)
	if err != nil || !hasGroupMemberInfo(info) {
		return adapterapi.GroupMemberInfo{}
	}

	if cache := s.currentIdentityCache(); cache != nil {
		cache.SetGroupMemberInfo(groupID, userID, cacheGroupMemberInfo(info))
	}
	return info
}

func (s *Shell) resolveStrangerInfo(ctx context.Context, userID string) adapterapi.StrangerInfo {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return adapterapi.StrangerInfo{}
	}

	if cache := s.currentIdentityCache(); cache != nil {
		if info, ok := cache.GetStrangerInfo(userID); ok && strings.TrimSpace(info.Nickname) != "" {
			return apiStrangerInfo(info)
		}
	}

	lookupCtx, cancel := withIdentityLookupTimeout(ctx)
	defer cancel()

	info, err := s.GetStrangerInfo(lookupCtx, userID)
	if err != nil || strings.TrimSpace(info.Nickname) == "" {
		return adapterapi.StrangerInfo{}
	}

	if cache := s.currentIdentityCache(); cache != nil {
		cache.SetStrangerInfo(userID, cacheStrangerInfo(info))
	}
	return info
}

func (s *Shell) currentIdentityCache() *adaptercache.IdentityCache {
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

func hasGroupMemberInfo(info adapterapi.GroupMemberInfo) bool {
	return strings.TrimSpace(info.Card) != "" || strings.TrimSpace(info.Nickname) != "" || strings.TrimSpace(info.Role) != "" || strings.TrimSpace(info.Title) != ""
}

func apiGroupMemberInfo(info adaptercache.GroupMemberInfo) adapterapi.GroupMemberInfo {
	return adapterapi.GroupMemberInfo{
		Role:     info.Role,
		Nickname: info.Nickname,
		Card:     info.Card,
		Title:    info.Title,
	}
}

func cacheGroupMemberInfo(info adapterapi.GroupMemberInfo) adaptercache.GroupMemberInfo {
	return adaptercache.GroupMemberInfo{
		Role:     info.Role,
		Nickname: info.Nickname,
		Card:     info.Card,
		Title:    info.Title,
	}
}

func apiStrangerInfo(info adaptercache.StrangerInfo) adapterapi.StrangerInfo {
	return adapterapi.StrangerInfo{Nickname: info.Nickname}
}

func cacheStrangerInfo(info adapterapi.StrangerInfo) adaptercache.StrangerInfo {
	return adaptercache.StrangerInfo{Nickname: info.Nickname}
}
