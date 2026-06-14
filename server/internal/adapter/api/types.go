package api

import adaptercache "github.com/RayleaBot/RayleaBot/server/internal/adapter/cache"

const ErrorCodeAPICallFailed = "adapter.api_call_failed"

type LoginInfo = adaptercache.LoginInfo

// VersionInfo holds implementation metadata returned by get_version_info.
type VersionInfo struct {
	AppName         string
	ProtocolVersion string
	AppVersion      string
}

type GroupMemberInfo = adaptercache.GroupMemberInfo

type GroupInfo = adaptercache.GroupInfo

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

type StrangerInfo = adaptercache.StrangerInfo
