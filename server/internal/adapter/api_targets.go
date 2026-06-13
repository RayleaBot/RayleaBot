package adapter

import (
	"context"
	"sort"
)

// ListGroups calls get_group_list and returns selectable group targets.
func (s *Shell) ListGroups(ctx context.Context) ([]GroupTarget, error) {
	raw, err := s.CallAPIAny(ctx, "get_group_list", nil)
	if err != nil {
		return nil, err
	}
	items, ok := normalizeAPIList(raw)
	if !ok {
		return nil, errorf(errorCodeAPICallFailed, "get_group_list returned a non-list payload", nil)
	}

	groups := make([]GroupTarget, 0, len(items))
	for _, item := range items {
		data, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id := extractStringField(data, "group_id")
		name := extractStringField(data, "group_name")
		if id == "" {
			continue
		}
		if name == "" {
			name = id
		}
		groups = append(groups, GroupTarget{ID: id, Name: name})
	}
	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].Name == groups[j].Name {
			return groups[i].ID < groups[j].ID
		}
		return groups[i].Name < groups[j].Name
	})
	return groups, nil
}

// ListFriends calls get_friend_list and returns selectable private targets.
func (s *Shell) ListFriends(ctx context.Context) ([]FriendTarget, error) {
	raw, err := s.CallAPIAny(ctx, "get_friend_list", nil)
	if err != nil {
		return nil, err
	}
	items, ok := normalizeAPIList(raw)
	if !ok {
		return nil, errorf(errorCodeAPICallFailed, "get_friend_list returned a non-list payload", nil)
	}

	friends := make([]FriendTarget, 0, len(items))
	for _, item := range items {
		data, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id := extractStringField(data, "user_id")
		nickname := extractStringField(data, "nickname")
		if nickname == "" {
			nickname = extractStringField(data, "remark")
		}
		if id == "" {
			continue
		}
		if nickname == "" {
			nickname = id
		}
		friends = append(friends, FriendTarget{ID: id, Nickname: nickname})
	}
	sort.SliceStable(friends, func(i, j int) bool {
		if friends[i].Nickname == friends[j].Nickname {
			return friends[i].ID < friends[j].ID
		}
		return friends[i].Nickname < friends[j].Nickname
	})
	return friends, nil
}
