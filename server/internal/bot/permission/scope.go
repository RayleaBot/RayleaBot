package permission

// Scope binds a persisted access-list rule to a protocol and bot account.
// Only OneBot11 global rules have an empty BotID.
type Scope struct {
	Kind           string `json:"kind"`
	SourceProtocol string `json:"source_protocol"`
	BotID          string `json:"bot_id"`
}

func (s Scope) Valid() bool {
	switch s.Kind {
	case "global":
		return s.SourceProtocol == "onebot11" && s.BotID == ""
	case "account":
		return (s.SourceProtocol == "onebot11" || s.SourceProtocol == "qqofficial") && s.BotID != ""
	default:
		return false
	}
}
