package chatevent

// BotIdentity is scoped to a configured adapter instance, including its protocol.
type BotIdentity struct {
	SourceAdapter  string `json:"source_adapter"`
	SourceProtocol string `json:"source_protocol"`
	ID             string `json:"id"`
	Nickname       string `json:"nickname,omitempty"`
}
