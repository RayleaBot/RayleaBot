package events

import "time"

const (
	channelEvents = "events"
	eventReceived = "events.received"
)

type Frame struct {
	Channel   string `json:"channel"`
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Data      any    `json:"data"`
}

type ServiceStatusPayload struct {
	ServiceStatus string   `json:"service_status"`
	Summary       string   `json:"summary"`
	Reason        string   `json:"reason,omitempty"`
	ReasonCodes   []string `json:"reason_codes,omitempty"`
}

type PluginStatePayload struct {
	PluginID          string              `json:"plugin_id"`
	RegistrationState string              `json:"registration_state"`
	DesiredState      string              `json:"desired_state"`
	RuntimeState      string              `json:"runtime_state"`
	DisplayState      string              `json:"display_state"`
	Commands          []PluginCommandItem `json:"commands"`
	CommandConflicts  []string            `json:"command_conflicts"`
}

type PluginCommandItem struct {
	Name          string   `json:"name"`
	Aliases       []string `json:"aliases,omitempty"`
	Description   string   `json:"description,omitempty"`
	Usage         string   `json:"usage,omitempty"`
	Permission    string   `json:"permission,omitempty"`
	CommandSource string   `json:"command_source"`
	DeclarationID string   `json:"declaration_id,omitempty"`
}

type GenericPayload struct {
	EventType string `json:"event_type"`
	Summary   string `json:"summary"`
}

type ProtocolSnapshotPayload struct {
	Protocol         string `json:"protocol"`
	ProtocolSnapshot any    `json:"protocol_snapshot"`
}

func NewReceivedFrame(data any) Frame {
	return Frame{
		Channel:   channelEvents,
		Type:      eventReceived,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data:      data,
	}
}
