package runtime

type pingFrame struct {
	ProtocolVersion string `json:"protocol_version"`
	Type            string `json:"type"`
	Timestamp       int64  `json:"timestamp"`
	PluginID        string `json:"plugin_id"`
	RequestID       string `json:"request_id"`
}

type initFrame struct {
	ProtocolVersion string            `json:"protocol_version"`
	Type            string            `json:"type"`
	Timestamp       int64             `json:"timestamp"`
	PluginID        string            `json:"plugin_id"`
	RequestID       string            `json:"request_id"`
	Bot             *botFrame         `json:"bot,omitempty"`
	Capabilities    []string          `json:"capabilities,omitempty"`
	Permissions     *permissionsFrame `json:"permissions,omitempty"`
	CommandPrefixes []string          `json:"command_prefixes"`
}

type botFrame struct {
	ID       string `json:"id"`
	Nickname string `json:"nickname,omitempty"`
}

type permissionsFrame struct {
	SuperAdmins []string `json:"super_admins,omitempty"`
}

type shutdownFrame struct {
	ProtocolVersion string `json:"protocol_version"`
	Type            string `json:"type"`
	Timestamp       int64  `json:"timestamp"`
	PluginID        string `json:"plugin_id"`
	RequestID       string `json:"request_id"`
	Reason          string `json:"reason"`
}

type frameEnvelope struct {
	ProtocolVersion string `json:"protocol_version"`
	Type            string `json:"type"`
	Timestamp       int64  `json:"timestamp"`
	PluginID        string `json:"plugin_id"`
	RequestID       string `json:"request_id"`
}

type initProgressFrame struct {
	ProtocolVersion string `json:"protocol_version"`
	Type            string `json:"type"`
	Timestamp       int64  `json:"timestamp"`
	PluginID        string `json:"plugin_id"`
	RequestID       string `json:"request_id"`
	Summary         string `json:"summary"`
}

type initAckFrame struct {
	Type          string   `json:"type"`
	RequestID     string   `json:"request_id"`
	Status        string   `json:"status"`
	Subscriptions []string `json:"subscriptions,omitempty"`
	ErrorMessage  string   `json:"error_message,omitempty"`
}

type errorFrame struct {
	Type      string         `json:"type"`
	RequestID string         `json:"request_id"`
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details,omitempty"`
}

type resultFrame struct {
	Type      string         `json:"type"`
	RequestID string         `json:"request_id"`
	Status    string         `json:"status"`
	Data      map[string]any `json:"data"`
}

type initResponseStatus int

const (
	initResponseWait initResponseStatus = iota
	initResponseReady
)
