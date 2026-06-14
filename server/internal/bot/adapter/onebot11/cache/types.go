package cache

type LoginInfo struct {
	ID       string
	Nickname string
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

type StrangerInfo struct {
	Nickname string
}
