package api

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
