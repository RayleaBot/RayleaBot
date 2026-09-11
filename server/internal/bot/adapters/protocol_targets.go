package adapters

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/adapters/onebot11"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
)

var ErrOneBotInstanceUnavailable = errors.New("指定的 OneBot11 实例不存在、未启用或不可用")

func (s *Service) CurrentOneBot11ProtocolTargets(ctx context.Context, adapterID string) (OneBot11ProtocolTargets, error) {
	response := OneBot11ProtocolTargets{
		Protocol:     "onebot11",
		Groups:       []OneBot11GroupTarget{},
		PrivateUsers: []OneBot11PrivateTarget{},
		Issues:       []OneBot11TargetIssue{},
	}
	ingress, ok := s.OneBot11Ingress(adapterID)
	if !ok {
		return response, ErrOneBotInstanceUnavailable
	}
	adapter := ingress.shell

	groupsResult, friendsResult := s.readOneBot11ProtocolTargets(ctx, adapter)
	if groupsResult.err != nil {
		response.Issues = append(response.Issues, oneBot11TargetIssue("groups", "群聊列表读取失败", groupsResult.err))
	} else {
		for _, group := range groupsResult.groups {
			response.Groups = append(response.Groups, OneBot11GroupTarget{
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
			response.PrivateUsers = append(response.PrivateUsers, OneBot11PrivateTarget{
				TargetType: "private",
				TargetID:   friend.ID,
				Nickname:   friend.Nickname,
				AvatarURL:  oneBot11AvatarURL(friend.ID),
			})
		}
	}

	response.Available = groupsResult.err == nil && friendsResult.err == nil
	return response, nil
}

type oneBot11GroupsResult struct {
	groups []onebot11.GroupTarget
	err    error
}

type oneBot11FriendsResult struct {
	friends []onebot11.FriendTarget
	err     error
}

func (s *Service) readOneBot11ProtocolTargets(ctx context.Context, adapter *onebot11.Shell) (oneBot11GroupsResult, oneBot11FriendsResult) {
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
		groups, err := adapter.ListGroups(groupCtx)
		ch <- oneBot11GroupsResult{groups: groups, err: err}
	}(groupsCh)
	go func(ch chan<- oneBot11FriendsResult) {
		friends, err := adapter.ListFriends(friendCtx)
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

func oneBot11TargetIssue(scope, fallback string, err error) OneBot11TargetIssue {
	return OneBot11TargetIssue{
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
	var adapterError *onebot11.Error
	if errors.As(err, &adapterError) && (adapterError.Code == errorcodes.AdapterConnectionLost || adapterError.Code == errorcodes.AdapterConnectionFailed) {
		return "OneBot 协议未连接"
	}
	if errors.Is(err, onebot11.ErrInvalidListPayload) {
		return strings.TrimSuffix(fallback, "失败") + "返回格式不支持"
	}
	return fallback
}

func (s *Service) oneBot11TargetTimeout() time.Duration {
	if s != nil && s.oneBot11TargetReadTimeout > 0 {
		return s.oneBot11TargetReadTimeout
	}
	return 3 * time.Second
}

func (s *Service) ResolveOneBot11Identities(ctx context.Context, adapterID string, items []OneBot11IdentityResolveItem) (OneBot11IdentityResolveResult, error) {
	response := OneBot11IdentityResolveResult{
		Items:  []OneBot11Identity{},
		Issues: []OneBot11TargetIssue{},
	}
	ingress, ok := s.OneBot11Ingress(adapterID)
	if !ok {
		return response, ErrOneBotInstanceUnavailable
	}
	adapter := ingress.shell

	seen := map[string]struct{}{}
	for _, item := range items {
		targetType := strings.TrimSpace(item.TargetType)
		targetID := strings.TrimSpace(item.TargetID)
		userID := strings.TrimSpace(item.UserID)
		if (targetType != "group" && targetType != "private") || !isDigits(targetID) || !isDigits(userID) {
			response.Issues = append(response.Issues, OneBot11TargetIssue{Scope: "identity", Message: "身份解析参数不合法"})
			continue
		}
		key := targetType + ":" + targetID + ":" + userID
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		switch targetType {
		case "group":
			member, err := adapter.GetGroupMemberInfo(ctx, targetID, userID)
			if err != nil {
				response.Issues = append(response.Issues, OneBot11TargetIssue{Scope: "identity", Message: "群成员身份读取失败"})
				continue
			}
			nickname := member.Nickname
			if nickname == "" {
				nickname = userID
			}
			response.Items = append(response.Items, OneBot11Identity{
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
			stranger, err := adapter.GetStrangerInfo(ctx, userID)
			if err != nil {
				response.Issues = append(response.Issues, OneBot11TargetIssue{Scope: "identity", Message: "私聊身份读取失败"})
				continue
			}
			nickname := stranger.Nickname
			if nickname == "" {
				nickname = userID
			}
			response.Items = append(response.Items, OneBot11Identity{
				TargetType: "private",
				TargetID:   targetID,
				UserID:     userID,
				Nickname:   nickname,
				AvatarURL:  oneBot11AvatarURL(userID),
			})
		}
	}
	return response, nil
}
