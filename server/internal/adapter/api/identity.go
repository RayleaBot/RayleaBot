package api

import "context"

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
