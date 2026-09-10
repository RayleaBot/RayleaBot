package chatevent

// MessageCommand requests an outbound message independently of the producer.
type MessageCommand struct {
	Kind                    string
	SourceProtocol          string
	SourceAdapter           string
	TargetType              string
	TargetID                string
	ReplyToEventID          string
	FallbackToSendIfMissing bool
	MessageSegments         []MessageSegment
}
