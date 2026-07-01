package protocolapi

import (
	"context"
	"errors"
	"strings"
	"time"

	adapterapi "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/api"
)

func (s *ProtocolService) currentOneBot11ProtocolTargets(ctx context.Context) oneBot11ProtocolTargetsResponse {
	response := oneBot11ProtocolTargetsResponse{
		Protocol:     "onebot11",
		Groups:       []oneBot11GroupTargetResponse{},
		PrivateUsers: []oneBot11PrivateTargetResponse{},
		Issues:       []oneBot11TargetIssueResponse{},
	}
	if s == nil || s.adapter == nil {
		response.Issues = append(response.Issues, oneBot11TargetIssueResponse{Scope: "protocol", Message: "OneBot11 协议不可用"})
		return response
	}

	groupsResult, friendsResult := s.readOneBot11ProtocolTargets(ctx)
	if groupsResult.err != nil {
		response.Issues = append(response.Issues, oneBot11TargetIssue("groups", "群聊列表读取失败", groupsResult.err))
	} else {
		for _, group := range groupsResult.groups {
			response.Groups = append(response.Groups, oneBot11GroupTargetResponse{
				TargetType: "group",
				TargetID:   group.ID,
				TargetName: group.Name,
				AvatarURL:  oneBot11GroupAvatarURL(group.ID),
			})
		}
	}

	if friendsResult.err != nil {
		response.Issues = append(response.Issues, oneBot11TargetIssue("private_users", "私聊对象列表读取失败", friendsResult.err))
	} else {
		for _, friend := range friendsResult.friends {
			response.PrivateUsers = append(response.PrivateUsers, oneBot11PrivateTargetResponse{
				TargetType: "private",
				TargetID:   friend.ID,
				Nickname:   friend.Nickname,
				AvatarURL:  oneBot11AvatarURL(friend.ID),
			})
		}
	}

	response.Available = groupsResult.err == nil && friendsResult.err == nil
	return response
}

type oneBot11GroupsResult struct {
	groups []adapterapi.GroupTarget
	err    error
}

type oneBot11FriendsResult struct {
	friends []adapterapi.FriendTarget
	err     error
}

func (s *ProtocolService) readOneBot11ProtocolTargets(ctx context.Context) (oneBot11GroupsResult, oneBot11FriendsResult) {
	timeout := s.oneBot11TargetTimeout()
	groupCtx, cancelGroups := context.WithTimeout(ctx, timeout)
	defer cancelGroups()
	friendCtx, cancelFriends := context.WithTimeout(ctx, timeout)
	defer cancelFriends()

	groupsCh := make(chan oneBot11GroupsResult, 1)
	friendsCh := make(chan oneBot11FriendsResult, 1)
	groupDone := groupCtx.Done()
	friendDone := friendCtx.Done()
	go func(ch chan<- oneBot11GroupsResult) {
		groups, err := s.adapter.ListGroups(groupCtx)
		ch <- oneBot11GroupsResult{groups: groups, err: err}
	}(groupsCh)
	go func(ch chan<- oneBot11FriendsResult) {
		friends, err := s.adapter.ListFriends(friendCtx)
		ch <- oneBot11FriendsResult{friends: friends, err: err}
	}(friendsCh)

	var groupsResult oneBot11GroupsResult
	var friendsResult oneBot11FriendsResult
	for groupsCh != nil || friendsCh != nil {
		select {
		case result := <-groupsCh:
			groupsResult = result
			groupsCh = nil
			groupDone = nil
		case result := <-friendsCh:
			friendsResult = result
			friendsCh = nil
			friendDone = nil
		case <-groupDone:
			if groupsCh != nil {
				groupsResult.err = groupCtx.Err()
				groupsCh = nil
				groupDone = nil
			}
		case <-friendDone:
			if friendsCh != nil {
				friendsResult.err = friendCtx.Err()
				friendsCh = nil
				friendDone = nil
			}
		case <-ctx.Done():
			if groupsCh != nil {
				groupsResult.err = ctx.Err()
				groupsCh = nil
			}
			if friendsCh != nil {
				friendsResult.err = ctx.Err()
				friendsCh = nil
			}
		}
	}
	return groupsResult, friendsResult
}

func oneBot11TargetIssue(scope, fallback string, err error) oneBot11TargetIssueResponse {
	return oneBot11TargetIssueResponse{
		Scope:   scope,
		Message: oneBot11TargetIssueMessage(fallback, err),
	}
}

func oneBot11TargetIssueMessage(fallback string, err error) string {
	if err == nil {
		return fallback
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return strings.TrimSuffix(fallback, "失败") + "超时"
	}
	normalized := strings.ToLower(err.Error())
	switch {
	case strings.Contains(normalized, "timed out"):
		return strings.TrimSuffix(fallback, "失败") + "超时"
	case strings.Contains(normalized, "not connected"):
		return "OneBot 协议未连接"
	case strings.Contains(normalized, "non-list payload"):
		return strings.TrimSuffix(fallback, "失败") + "返回格式不支持"
	default:
		return fallback
	}
}

func (s *ProtocolService) oneBot11TargetTimeout() time.Duration {
	if s != nil && s.oneBot11TargetReadTimeout > 0 {
		return s.oneBot11TargetReadTimeout
	}
	return 3 * time.Second
}

func (s *ProtocolService) resolveOneBot11Identities(ctx context.Context, items []oneBot11IdentityResolveItem) oneBot11IdentityResolveResponse {
	response := oneBot11IdentityResolveResponse{
		Items:  []oneBot11IdentityResponse{},
		Issues: []oneBot11TargetIssueResponse{},
	}
	if s == nil || s.adapter == nil {
		response.Issues = append(response.Issues, oneBot11TargetIssueResponse{Scope: "protocol", Message: "OneBot11 协议不可用"})
		return response
	}

	seen := map[string]struct{}{}
	for _, item := range items {
		targetType := strings.TrimSpace(item.TargetType)
		targetID := strings.TrimSpace(item.TargetID)
		userID := strings.TrimSpace(item.UserID)
		if (targetType != "group" && targetType != "private") || !isDigits(targetID) || !isDigits(userID) {
			response.Issues = append(response.Issues, oneBot11TargetIssueResponse{Scope: "identity", Message: "身份解析参数不合法"})
			continue
		}
		key := targetType + ":" + targetID + ":" + userID
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		switch targetType {
		case "group":
			member, err := s.adapter.GetGroupMemberInfo(ctx, targetID, userID)
			if err != nil {
				response.Issues = append(response.Issues, oneBot11TargetIssueResponse{Scope: "identity", Message: "群成员身份读取失败"})
				continue
			}
			nickname := member.Nickname
			if nickname == "" {
				nickname = userID
			}
			response.Items = append(response.Items, oneBot11IdentityResponse{
				TargetType:    "group",
				TargetID:      targetID,
				UserID:        userID,
				Nickname:      nickname,
				GroupNickname: member.Card,
				Title:         member.Title,
				Role:          member.Role,
				RoleLabel:     oneBot11RoleLabel(member.Role),
				AvatarURL:     oneBot11AvatarURL(userID),
			})
		case "private":
			stranger, err := s.adapter.GetStrangerInfo(ctx, userID)
			if err != nil {
				response.Issues = append(response.Issues, oneBot11TargetIssueResponse{Scope: "identity", Message: "私聊身份读取失败"})
				continue
			}
			nickname := stranger.Nickname
			if nickname == "" {
				nickname = userID
			}
			response.Items = append(response.Items, oneBot11IdentityResponse{
				TargetType: "private",
				TargetID:   targetID,
				UserID:     userID,
				Nickname:   nickname,
				AvatarURL:  oneBot11AvatarURL(userID),
			})
		}
	}
	return response
}
