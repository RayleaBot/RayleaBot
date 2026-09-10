package chatevent

type Event struct {
	EventID        string
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
	Route           string
	ReceivedAt      int64
	ClientTimestamp *int64
	ClientEventID   string
}
