package adapter

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
