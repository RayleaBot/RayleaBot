package segments

// MessageSegment represents a structured message segment from the OneBot11
// protocol, normalized into a protocol-agnostic form.
type MessageSegment struct {
	Type string
	Data map[string]any
}
