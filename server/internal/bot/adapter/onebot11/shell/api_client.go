package shell

import (
	"context"

	adapterapi "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/api"
)

func (s *Shell) GetLoginInfo(ctx context.Context) (adapterapi.LoginInfo, error) {
	return adapterapi.NewClient(shellAPICaller{s: s}).GetLoginInfo(ctx)
}

func (s *Shell) GetVersionInfo(ctx context.Context) (adapterapi.VersionInfo, error) {
	return adapterapi.NewClient(shellAPICaller{s: s}).GetVersionInfo(ctx)
}

func (s *Shell) getVersionInfoOnTransport(ctx context.Context, transport TransportKey) (adapterapi.VersionInfo, error) {
	return adapterapi.NewClient(shellAPICaller{s: s}).GetVersionInfoOnTransport(ctx, string(transport))
}

func (s *Shell) getLoginInfoOnTransport(ctx context.Context, transport TransportKey) (adapterapi.LoginInfo, error) {
	return adapterapi.NewClient(shellAPICaller{s: s}).GetLoginInfoOnTransport(ctx, string(transport))
}

func (s *Shell) GetGroupMemberInfo(ctx context.Context, groupID, userID string) (adapterapi.GroupMemberInfo, error) {
	return adapterapi.NewClient(shellAPICaller{s: s}).GetGroupMemberInfo(ctx, groupID, userID)
}

func (s *Shell) GetGroupInfo(ctx context.Context, groupID string) (adapterapi.GroupInfo, error) {
	return adapterapi.NewClient(shellAPICaller{s: s}).GetGroupInfo(ctx, groupID)
}

func (s *Shell) GetStrangerInfo(ctx context.Context, userID string) (adapterapi.StrangerInfo, error) {
	return adapterapi.NewClient(shellAPICaller{s: s}).GetStrangerInfo(ctx, userID)
}

func (s *Shell) ListGroups(ctx context.Context) ([]adapterapi.GroupTarget, error) {
	return adapterapi.NewClient(shellAPICaller{s: s, bestEffort: true}).ListGroups(ctx)
}

func (s *Shell) ListFriends(ctx context.Context) ([]adapterapi.FriendTarget, error) {
	return adapterapi.NewClient(shellAPICaller{s: s, bestEffort: true}).ListFriends(ctx)
}

type shellAPICaller struct {
	s          *Shell
	bestEffort bool
}

func (c shellAPICaller) CallAPI(ctx context.Context, action string, params map[string]any) (map[string]any, error) {
	return c.s.callAPI(ctx, action, params)
}

func (c shellAPICaller) CallAPIAny(ctx context.Context, action string, params map[string]any) (any, error) {
	if c.bestEffort {
		return c.s.callAPIAnyBestEffort(ctx, action, params)
	}
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

func normalizeAPIResult(value any) any {
	return adapterapi.NormalizeAPIResult(value)
}

func extractStringValue(value any) string {
	return adapterapi.ExtractStringValue(value)
}

func (s *Shell) ResolveTargetName(ctx context.Context, targetType, targetID string) string {
	return adapterapi.ResolveTargetName(ctx, targetType, targetID, shellTargetNameResolver{s: s})
}

type shellTargetNameResolver struct {
	s *Shell
}

func (r shellTargetNameResolver) ResolveGroupName(ctx context.Context, targetID string) string {
	return r.s.resolveGroupName(ctx, targetID)
}

func (r shellTargetNameResolver) ResolvePrivateName(ctx context.Context, targetID string) string {
	return r.s.resolveStrangerInfo(ctx, targetID).Nickname
}
