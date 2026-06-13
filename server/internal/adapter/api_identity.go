package adapter

import "context"

// GetLoginInfo calls the OneBot11 get_login_info API and returns the bot's user ID and nickname.
func (s *Shell) GetLoginInfo(ctx context.Context) (LoginInfo, error) {
	data, err := s.callAPI(ctx, "get_login_info", nil)
	if err != nil {
		return LoginInfo{}, err
	}

	return LoginInfo{
		ID:       extractStringField(data, "user_id"),
		Nickname: extractStringField(data, "nickname"),
	}, nil
}

// GetVersionInfo calls the OneBot11 get_version_info API and returns implementation metadata.
func (s *Shell) GetVersionInfo(ctx context.Context) (VersionInfo, error) {
	data, err := s.callAPI(ctx, "get_version_info", nil)
	if err != nil {
		return VersionInfo{}, err
	}

	return VersionInfo{
		AppName:         extractStringField(data, "app_name"),
		ProtocolVersion: extractStringField(data, "protocol_version"),
		AppVersion:      extractStringField(data, "app_version"),
	}, nil
}

func (s *Shell) getVersionInfoOnTransport(ctx context.Context, transport TransportKey) (VersionInfo, error) {
	data, err := s.callAPIOnTransport(ctx, transport, "get_version_info", nil)
	if err != nil {
		return VersionInfo{}, err
	}

	return VersionInfo{
		AppName:         extractStringField(data, "app_name"),
		ProtocolVersion: extractStringField(data, "protocol_version"),
		AppVersion:      extractStringField(data, "app_version"),
	}, nil
}

func (s *Shell) getLoginInfoOnTransport(ctx context.Context, transport TransportKey) (LoginInfo, error) {
	data, err := s.callAPIOnTransport(ctx, transport, "get_login_info", nil)
	if err != nil {
		return LoginInfo{}, err
	}

	return LoginInfo{
		ID:       extractStringField(data, "user_id"),
		Nickname: extractStringField(data, "nickname"),
	}, nil
}

// GetGroupMemberInfo calls the OneBot11 get_group_member_info API.
func (s *Shell) GetGroupMemberInfo(ctx context.Context, groupID, userID string) (GroupMemberInfo, error) {
	data, err := s.callAPI(ctx, "get_group_member_info", map[string]any{
		"group_id": oneBotTargetValue(groupID),
		"user_id":  oneBotTargetValue(userID),
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

// GetGroupInfo calls the OneBot11 get_group_info API.
func (s *Shell) GetGroupInfo(ctx context.Context, groupID string) (GroupInfo, error) {
	data, err := s.callAPI(ctx, "get_group_info", map[string]any{
		"group_id": oneBotTargetValue(groupID),
		"no_cache": true,
	})
	if err != nil {
		return GroupInfo{}, err
	}

	return GroupInfo{
		Name: extractStringField(data, "group_name"),
	}, nil
}

// GetStrangerInfo calls the OneBot11 get_stranger_info API.
func (s *Shell) GetStrangerInfo(ctx context.Context, userID string) (StrangerInfo, error) {
	data, err := s.callAPI(ctx, "get_stranger_info", map[string]any{
		"user_id": oneBotTargetValue(userID),
	})
	if err != nil {
		return StrangerInfo{}, err
	}

	return StrangerInfo{
		Nickname: extractStringField(data, "nickname"),
	}, nil
}
