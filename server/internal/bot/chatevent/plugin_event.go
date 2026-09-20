package chatevent

type Event struct {
	EventID        string
	BotID          string
	Session        *SessionRef
	SourceProtocol string
	SourceAdapter  string
	EventType      string
	Timestamp      int64
	Actor          *Actor
	Target         *Target
	Message        *Message
	Webhook        *Webhook
	PayloadFields  map[string]any
	MessageID      string
	RawPayload     any
	CommandTargets []CommandTarget
	// CommandResolved marks CommandTargets as authoritative, including when empty:
	// delivery then never falls back to matching command names without prefixes.
	CommandResolved bool
}

// CommandTarget is one plugin's own parse of a command message. It is host
// routing metadata: delivery copies Command and Args into that plugin's payload,
// and the list itself is never sent to a plugin.
type CommandTarget struct {
	PluginID string
	Command  string
	Args     []string
}

// SessionRef is host routing metadata. Business state remains in the plugin.
type SessionRef struct {
	SessionID   string `json:"session_id"`
	Scope       string `json:"scope"`
	ExpiresAtMS int64  `json:"expires_at_ms"`
}

type Actor struct {
	ID       string
	Nickname string
	Role     string
}

type Target struct {
	Type string
	ID   string
	Name string
}

type Message struct {
	PlainText string
	Segments  []MessageSegment
}

type Webhook struct {
	Route      string
	ReceivedAt int64
}
