package shell

import (
	"context"

	adapterapi "github.com/RayleaBot/RayleaBot/server/internal/adapter/api"
)

const errorCodeAPICallFailed = adapterapi.ErrorCodeAPICallFailed

type LoginInfo = adapterapi.LoginInfo
type VersionInfo = adapterapi.VersionInfo
type GroupMemberInfo = adapterapi.GroupMemberInfo
type GroupInfo = adapterapi.GroupInfo
type GroupTarget = adapterapi.GroupTarget
type FriendTarget = adapterapi.FriendTarget
type StrangerInfo = adapterapi.StrangerInfo

func (s *Shell) GetLoginInfo(ctx context.Context) (LoginInfo, error) {
	return adapterapi.NewClient(shellAPICaller{s: s}).GetLoginInfo(ctx)
}

func (s *Shell) GetVersionInfo(ctx context.Context) (VersionInfo, error) {
	return adapterapi.NewClient(shellAPICaller{s: s}).GetVersionInfo(ctx)
}

func (s *Shell) getVersionInfoOnTransport(ctx context.Context, transport TransportKey) (VersionInfo, error) {
	return adapterapi.NewClient(shellAPICaller{s: s}).GetVersionInfoOnTransport(ctx, string(transport))
}

func (s *Shell) getLoginInfoOnTransport(ctx context.Context, transport TransportKey) (LoginInfo, error) {
	return adapterapi.NewClient(shellAPICaller{s: s}).GetLoginInfoOnTransport(ctx, string(transport))
}

func (s *Shell) GetGroupMemberInfo(ctx context.Context, groupID, userID string) (GroupMemberInfo, error) {
	return adapterapi.NewClient(shellAPICaller{s: s}).GetGroupMemberInfo(ctx, groupID, userID)
}

func (s *Shell) GetGroupInfo(ctx context.Context, groupID string) (GroupInfo, error) {
	return adapterapi.NewClient(shellAPICaller{s: s}).GetGroupInfo(ctx, groupID)
}

func (s *Shell) GetStrangerInfo(ctx context.Context, userID string) (StrangerInfo, error) {
	return adapterapi.NewClient(shellAPICaller{s: s}).GetStrangerInfo(ctx, userID)
}

func (s *Shell) ListGroups(ctx context.Context) ([]GroupTarget, error) {
	return adapterapi.NewClient(shellAPICaller{s: s}).ListGroups(ctx)
}

func (s *Shell) ListFriends(ctx context.Context) ([]FriendTarget, error) {
	return adapterapi.NewClient(shellAPICaller{s: s}).ListFriends(ctx)
}

type shellAPICaller struct {
	s *Shell
}

func (c shellAPICaller) CallAPI(ctx context.Context, action string, params map[string]any) (map[string]any, error) {
	return c.s.callAPI(ctx, action, params)
}

func (c shellAPICaller) CallAPIAny(ctx context.Context, action string, params map[string]any) (any, error) {
	return c.s.CallAPIAny(ctx, action, params)
}

func (c shellAPICaller) CallAPIOnTransport(ctx context.Context, transport string, action string, params map[string]any) (map[string]any, error) {
	return c.s.callAPIOnTransport(ctx, TransportKey(transport), action, params)
}

func (c shellAPICaller) TargetValue(targetID string) any {
	return oneBotTargetValue(targetID)
}

func (c shellAPICaller) Errorf(code, message string, err error) error {
	return errorf(code, message, err)
}

func extractStringField(data map[string]any, key string) string {
	return adapterapi.ExtractStringField(data, key)
}

func normalizeAPIList(value any) ([]any, bool) {
	return adapterapi.NormalizeAPIList(value)
}

func normalizeAPIResult(value any) any {
	return adapterapi.NormalizeAPIResult(value)
}

func extractStringValue(value any) string {
	return adapterapi.ExtractStringValue(value)
}
