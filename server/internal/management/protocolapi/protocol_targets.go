package protocolapi

import (
	"context"
	"strings"
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

	groups, groupErr := s.adapter.ListGroups(ctx)
	if groupErr != nil {
		response.Issues = append(response.Issues, oneBot11TargetIssueResponse{Scope: "groups", Message: "群聊列表读取失败"})
	} else {
		for _, group := range groups {
			response.Groups = append(response.Groups, oneBot11GroupTargetResponse{
				TargetType: "group",
				TargetID:   group.ID,
				TargetName: group.Name,
				AvatarURL:  oneBot11GroupAvatarURL(group.ID),
			})
		}
	}

	friends, friendErr := s.adapter.ListFriends(ctx)
	if friendErr != nil {
		response.Issues = append(response.Issues, oneBot11TargetIssueResponse{Scope: "private_users", Message: "私聊对象列表读取失败"})
	} else {
		for _, friend := range friends {
			response.PrivateUsers = append(response.PrivateUsers, oneBot11PrivateTargetResponse{
				TargetType: "private",
				TargetID:   friend.ID,
				Nickname:   friend.Nickname,
				AvatarURL:  oneBot11AvatarURL(friend.ID),
			})
		}
	}

	response.Available = groupErr == nil && friendErr == nil
	return response
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
