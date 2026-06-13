package runtimeprotocol

type PingFrame struct {
	ProtocolVersion string `json:"protocol_version"`
	Type            string `json:"type"`
	Timestamp       int64  `json:"timestamp"`
	PluginID        string `json:"plugin_id"`
	RequestID       string `json:"request_id"`
}

type InitFrame struct {
	ProtocolVersion string            `json:"protocol_version"`
	Type            string            `json:"type"`
	Timestamp       int64             `json:"timestamp"`
	PluginID        string            `json:"plugin_id"`
	RequestID       string            `json:"request_id"`
	Bot             *BotFrame         `json:"bot,omitempty"`
	Capabilities    []string          `json:"capabilities,omitempty"`
	Permissions     *PermissionsFrame `json:"permissions,omitempty"`
	CommandPrefixes []string          `json:"command_prefixes"`
}

type BotFrame struct {
	ID       string `json:"id"`
	Nickname string `json:"nickname,omitempty"`
}

type PermissionsFrame struct {
	SuperAdmins []string `json:"super_admins,omitempty"`
}

type ShutdownFrame struct {
	ProtocolVersion string `json:"protocol_version"`
	Type            string `json:"type"`
	Timestamp       int64  `json:"timestamp"`
	PluginID        string `json:"plugin_id"`
	RequestID       string `json:"request_id"`
	Reason          string `json:"reason"`
}

type FrameEnvelope struct {
	ProtocolVersion string `json:"protocol_version"`
	Type            string `json:"type"`
	Timestamp       int64  `json:"timestamp"`
	PluginID        string `json:"plugin_id"`
	RequestID       string `json:"request_id"`
}

type InitProgressFrame struct {
	ProtocolVersion string `json:"protocol_version"`
	Type            string `json:"type"`
	Timestamp       int64  `json:"timestamp"`
	PluginID        string `json:"plugin_id"`
	RequestID       string `json:"request_id"`
	Summary         string `json:"summary"`
}

type InitAckFrame struct {
	Type          string   `json:"type"`
	RequestID     string   `json:"request_id"`
	Status        string   `json:"status"`
	Subscriptions []string `json:"subscriptions,omitempty"`
	ErrorMessage  string   `json:"error_message,omitempty"`
}

type ErrorFrame struct {
	Type      string         `json:"type"`
	RequestID string         `json:"request_id"`
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details,omitempty"`
}

type ResultFrame struct {
	Type      string         `json:"type"`
	RequestID string         `json:"request_id"`
	Status    string         `json:"status"`
	Data      map[string]any `json:"data"`
}

type InitResponseStatus int

const (
	InitResponseWait InitResponseStatus = iota
	InitResponseReady
)
