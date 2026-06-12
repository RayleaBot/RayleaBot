package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/textsafe"
)

const errorCodeAPICallFailed = "adapter.api_call_failed"

// LoginInfo holds the bot's login identity returned by get_login_info.
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

// GroupMemberInfo holds a group member's role and display names.
type GroupMemberInfo struct {
	Role     string
	Nickname string
	Card     string
	Title    string
}

// GroupInfo holds basic group metadata.
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

// StrangerInfo holds a stranger's nickname.
type StrangerInfo struct {
	Nickname string
}

// apiCallRequest is the generic JSON envelope for OneBot11 API calls.
type apiCallRequest struct {
	Action string         `json:"action"`
	Params map[string]any `json:"params,omitempty"`
	Echo   string         `json:"echo"`
}

// callAPI sends a generic OneBot11 API request and waits for the matched
// response. It reuses the same echo-based request/response infrastructure
// that outbound.go uses for send_msg.
func (s *Shell) callAPI(ctx context.Context, action string, params map[string]any) (map[string]any, error) {
	responseData, err := s.CallAPIAny(ctx, action, params)
	if err != nil {
		return nil, err
	}
	data, ok := responseData.(map[string]any)
	if !ok {
		return nil, errorf(errorCodeAPICallFailed, fmt.Sprintf("%s returned a non-object payload", action), nil)
	}
	return data, nil
}

func (s *Shell) callAPIOnTransport(ctx context.Context, transport TransportKey, action string, params map[string]any) (map[string]any, error) {
	responseData, err := s.callAPIAnyOnTransport(ctx, transport, action, params)
	if err != nil {
		return nil, err
	}
	data, ok := responseData.(map[string]any)
	if !ok {
		return nil, errorf(errorCodeAPICallFailed, fmt.Sprintf("%s returned a non-object payload", action), nil)
	}
	return data, nil
}

func (s *Shell) CallAPIAny(ctx context.Context, action string, params map[string]any) (any, error) {
	return s.callAPIAnyOnTransport(ctx, "", action, params)
}

func (s *Shell) callAPIAnyOnTransport(ctx context.Context, transport TransportKey, action string, params map[string]any) (any, error) {
	echo := s.nextRequestEcho()
	request := apiCallRequest{
		Action: action,
		Params: params,
		Echo:   echo,
	}

	if transport != "" {
		switch transport {
		case TransportForwardWS, TransportReverseWS:
			conn, _, snapshot := s.currentWSConnForTransport(transport)
			if conn == nil || snapshot.State != StateConnected {
				return nil, errorf(errorCodeConnectionLost, "adapter transport is not connected", nil)
			}
			responseCh := make(chan apiResponse, 1)
			s.registerPendingResponse(echo, responseCh)
			defer s.dropPendingResponse(echo)

			s.sendMu.Lock()
			writeErr := wsjsonWrite(ctx, conn, request)
			s.sendMu.Unlock()
			if writeErr != nil {
				return nil, errorf(errorCodeAPICallFailed, fmt.Sprintf("%s request failed", action), writeErr)
			}

			select {
			case response := <-responseCh:
				if response.Status != "ok" || response.RetCode != 0 {
					message := fmt.Sprintf("%s call failed", action)
					if response.Wording != "" {
						message = response.Wording
					}
					return nil, errorf(errorCodeAPICallFailed, message, nil)
				}
				result := normalizeAPIResult(response.Data)
				s.invalidateIdentityCacheForAPICall(action, params)
				return result, nil
			case <-ctx.Done():
				return nil, errorf(errorCodeAPICallFailed, fmt.Sprintf("%s response timed out", action), ctx.Err())
			}
		case TransportHTTPAPI:
			response, err := s.doHTTPAPIRequest(ctx, request)
			if err != nil {
				return nil, err
			}
			if response.Status != "ok" || response.RetCode != 0 {
				message := fmt.Sprintf("%s call failed", action)
				if response.Wording != "" {
					message = response.Wording
				}
				return nil, errorf(errorCodeAPICallFailed, message, nil)
			}
			result := normalizeAPIResult(response.Data)
			s.invalidateIdentityCacheForAPICall(action, params)
			return result, nil
		default:
			return nil, errorf(errorCodeConnectionLost, "adapter transport is not connected", nil)
		}
	}

	conn, _, snapshot := s.currentWSConn()
	if conn != nil && snapshot.State == StateConnected {
		responseCh := make(chan apiResponse, 1)
		s.registerPendingResponse(echo, responseCh)
		defer s.dropPendingResponse(echo)

		s.sendMu.Lock()
		writeErr := wsjsonWrite(ctx, conn, request)
		s.sendMu.Unlock()
		if writeErr != nil {
			return nil, errorf(errorCodeAPICallFailed, fmt.Sprintf("write %s request", action), writeErr)
		}

		select {
		case response := <-responseCh:
			if response.Status != "ok" || response.RetCode != 0 {
				message := fmt.Sprintf("%s call failed", action)
				if response.Wording != "" {
					message = response.Wording
				}
				return nil, errorf(errorCodeAPICallFailed, message, nil)
			}
			result := normalizeAPIResult(response.Data)
			s.invalidateIdentityCacheForAPICall(action, params)
			return result, nil
		case <-ctx.Done():
			return nil, errorf(errorCodeAPICallFailed, fmt.Sprintf("%s response timed out", action), ctx.Err())
		}
	}

	response, err := s.doHTTPAPIRequest(ctx, request)
	if err != nil {
		return nil, err
	}
	if response.Status != "ok" || response.RetCode != 0 {
		message := fmt.Sprintf("%s call failed", action)
		if response.Wording != "" {
			message = response.Wording
		}
		return nil, errorf(errorCodeAPICallFailed, message, nil)
	}
	result := normalizeAPIResult(response.Data)
	s.invalidateIdentityCacheForAPICall(action, params)
	return result, nil
}

// GetLoginInfo calls the OneBot11 get_login_info API and returns the bot's
// user ID and nickname.
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

// GetVersionInfo calls the OneBot11 get_version_info API and returns the
// implementation metadata used for provider detection.
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

// extractStringField extracts a string value from a data map, handling both
// string and numeric JSON values (float64, json.Number).
func extractStringField(data map[string]any, key string) string {
	if data == nil {
		return ""
	}

	switch value := data[key].(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(textsafe.SanitizeString(value))
	case float64:
		return strconv.FormatInt(int64(value), 10)
	default:
		return textsafe.SanitizeString(fmt.Sprint(value))
	}
}

func normalizeAPIList(value any) ([]any, bool) {
	switch typed := value.(type) {
	case []any:
		return typed, true
	case map[string]any:
		for _, key := range []string{"items", "list", "data"} {
			if items, ok := typed[key].([]any); ok {
				return items, true
			}
		}
	}
	return nil, false
}

func normalizeAPIResult(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			if item == nil {
				result[key] = nil
				continue
			}
			if isIdentifierKey(key) {
				result[key] = extractStringValue(item)
				continue
			}
			result[key] = normalizeAPIResult(item)
		}
		return result
	case []any:
		items := make([]any, 0, len(typed))
		for _, item := range typed {
			items = append(items, normalizeAPIResult(item))
		}
		return items
	default:
		return normalizeScalarValue(typed)
	}
}

func normalizeScalarValue(value any) any {
	switch typed := value.(type) {
	case string:
		return textsafe.SanitizeString(typed)
	case json.Number:
		return typed.String()
	case float64:
		if typed == float64(int64(typed)) {
			return int64(typed)
		}
		return typed
	default:
		return value
	}
}

func extractStringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(textsafe.SanitizeString(typed))
	case json.Number:
		return typed.String()
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	default:
		return strings.TrimSpace(textsafe.SanitizeString(fmt.Sprint(typed)))
	}
}

func isIdentifierKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	return key == "id" || strings.HasSuffix(key, "_id") || strings.HasSuffix(key, "_seq")
}
