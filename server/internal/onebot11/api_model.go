package onebot11

import (
	"context"
	"sort"
)

const ErrorCodeAPICallFailed = "adapter.api_call_failed"

type LoginInfo struct {
	ID       string
	Nickname string
}

// VersionInfo holds implementation metadata returned by get_version_info.
type VersionInfo struct {
	AppName         string
	ProtocolVersion string
	AppVersion      string
}

type GroupMemberInfo struct {
	Role     string
	Nickname string
	Card     string
	Title    string
}

type GroupInfo struct {
	Name string
}

// GroupTarget holds a selectable OneBot11 group target.
type GroupTarget struct {
	ID   string
	Name string
}

// FriendTarget holds a selectable OneBot11 private message target.
type FriendTarget struct {
	ID       string
	Nickname string
}

type StrangerInfo struct {
	Nickname string
}

type Caller interface {
	CallAPI(context.Context, string, map[string]any) (map[string]any, error)
	CallAPIAny(context.Context, string, map[string]any) (any, error)
	CallAPIOnTransport(context.Context, string, string, map[string]any) (map[string]any, error)
	TargetValue(string) any
	Errorf(string, string, error) error
}

type Client struct {
	caller Caller
}

func NewClient(caller Caller) Client {
	return Client{caller: caller}
}

func (c Client) GetLoginInfo(ctx context.Context) (LoginInfo, error) {
	data, err := c.caller.CallAPI(ctx, "get_login_info", nil)
	if err != nil {
		return LoginInfo{}, err
	}

	return LoginInfo{
		ID:       extractStringField(data, "user_id"),
		Nickname: extractStringField(data, "nickname"),
	}, nil
}

func (c Client) GetVersionInfo(ctx context.Context) (VersionInfo, error) {
	data, err := c.caller.CallAPI(ctx, "get_version_info", nil)
	if err != nil {
		return VersionInfo{}, err
	}

	return VersionInfo{
		AppName:         extractStringField(data, "app_name"),
		ProtocolVersion: extractStringField(data, "protocol_version"),
		AppVersion:      extractStringField(data, "app_version"),
	}, nil
}

func (c Client) GetVersionInfoOnTransport(ctx context.Context, transport string) (VersionInfo, error) {
	data, err := c.caller.CallAPIOnTransport(ctx, transport, "get_version_info", nil)
	if err != nil {
		return VersionInfo{}, err
	}

	return VersionInfo{
		AppName:         extractStringField(data, "app_name"),
		ProtocolVersion: extractStringField(data, "protocol_version"),
		AppVersion:      extractStringField(data, "app_version"),
	}, nil
}

func (c Client) GetLoginInfoOnTransport(ctx context.Context, transport string) (LoginInfo, error) {
	data, err := c.caller.CallAPIOnTransport(ctx, transport, "get_login_info", nil)
	if err != nil {
		return LoginInfo{}, err
	}

	return LoginInfo{
		ID:       extractStringField(data, "user_id"),
		Nickname: extractStringField(data, "nickname"),
	}, nil
}

func (c Client) GetGroupMemberInfo(ctx context.Context, groupID, userID string) (GroupMemberInfo, error) {
	data, err := c.caller.CallAPI(ctx, "get_group_member_info", map[string]any{
		"group_id": c.caller.TargetValue(groupID),
		"user_id":  c.caller.TargetValue(userID),
		"no_cache": true,
	})
	if err != nil {
		return GroupMemberInfo{}, err
	}

	return GroupMemberInfo{
		Role:     extractStringField(data, "role"),
		Nickname: extractStringField(data, "nickname"),
		Card:     extractStringField(data, "card"),
		Title:    extractStringField(data, "title"),
	}, nil
}

func (c Client) GetGroupInfo(ctx context.Context, groupID string) (GroupInfo, error) {
	data, err := c.caller.CallAPI(ctx, "get_group_info", map[string]any{
		"group_id": c.caller.TargetValue(groupID),
		"no_cache": true,
	})
	if err != nil {
		return GroupInfo{}, err
	}

	return GroupInfo{
		Name: extractStringField(data, "group_name"),
	}, nil
}

func (c Client) GetStrangerInfo(ctx context.Context, userID string) (StrangerInfo, error) {
	data, err := c.caller.CallAPI(ctx, "get_stranger_info", map[string]any{
		"user_id": c.caller.TargetValue(userID),
	})
	if err != nil {
		return StrangerInfo{}, err
	}

	return StrangerInfo{
		Nickname: extractStringField(data, "nickname"),
	}, nil
}

type TargetNameResolver interface {
	ResolveGroupName(context.Context, string) string
	ResolvePrivateName(context.Context, string) string
}

func ResolveTargetName(ctx context.Context, targetType, targetID string, resolver TargetNameResolver) string {
	if resolver == nil {
		return ""
	}
	switch targetType {
	case "group":
		return resolver.ResolveGroupName(ctx, targetID)
	case "private":
		return resolver.ResolvePrivateName(ctx, targetID)
	default:
		return ""
	}
}

func (c Client) ListGroups(ctx context.Context) ([]GroupTarget, error) {
	raw, err := c.caller.CallAPIAny(ctx, "get_group_list", nil)
	if err != nil {
		return nil, err
	}
	items, ok := normalizeAPIListWithKeys(raw, []string{"groups", "group_list", "items", "list", "data"})
	if !ok {
		return nil, c.caller.Errorf(ErrorCodeAPICallFailed, "get_group_list returned a non-list payload", nil)
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

func (c Client) ListFriends(ctx context.Context) ([]FriendTarget, error) {
	raw, err := c.caller.CallAPIAny(ctx, "get_friend_list", nil)
	if err != nil {
		return nil, err
	}
	items, ok := normalizeAPIListWithKeys(raw, []string{"friends", "private_users", "friend_list", "items", "list", "data"})
	if !ok {
		return nil, c.caller.Errorf(ErrorCodeAPICallFailed, "get_friend_list returned a non-list payload", nil)
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
