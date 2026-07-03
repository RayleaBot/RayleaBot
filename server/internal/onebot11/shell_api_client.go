package onebot11

import (
	"context"
)

func (s *Shell) GetLoginInfo(ctx context.Context) (LoginInfo, error) {
	return NewClient(shellAPICaller{s: s}).GetLoginInfo(ctx)
}

func (s *Shell) GetVersionInfo(ctx context.Context) (VersionInfo, error) {
	return NewClient(shellAPICaller{s: s}).GetVersionInfo(ctx)
}

func (s *Shell) getVersionInfoOnTransport(ctx context.Context, transport TransportKey) (VersionInfo, error) {
	return NewClient(shellAPICaller{s: s}).GetVersionInfoOnTransport(ctx, string(transport))
}

func (s *Shell) getLoginInfoOnTransport(ctx context.Context, transport TransportKey) (LoginInfo, error) {
	return NewClient(shellAPICaller{s: s}).GetLoginInfoOnTransport(ctx, string(transport))
}

func (s *Shell) GetGroupMemberInfo(ctx context.Context, groupID, userID string) (GroupMemberInfo, error) {
	return NewClient(shellAPICaller{s: s}).GetGroupMemberInfo(ctx, groupID, userID)
}

func (s *Shell) GetGroupInfo(ctx context.Context, groupID string) (GroupInfo, error) {
	return NewClient(shellAPICaller{s: s}).GetGroupInfo(ctx, groupID)
}

func (s *Shell) GetStrangerInfo(ctx context.Context, userID string) (StrangerInfo, error) {
	return NewClient(shellAPICaller{s: s}).GetStrangerInfo(ctx, userID)
}

func (s *Shell) ListGroups(ctx context.Context) ([]GroupTarget, error) {
	return NewClient(shellAPICaller{s: s, bestEffort: true}).ListGroups(ctx)
}

func (s *Shell) ListFriends(ctx context.Context) ([]FriendTarget, error) {
	return NewClient(shellAPICaller{s: s, bestEffort: true}).ListFriends(ctx)
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

func (s *Shell) ResolveTargetName(ctx context.Context, targetType, targetID string) string {
	return ResolveTargetName(ctx, targetType, targetID, shellTargetNameResolver{s: s})
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
