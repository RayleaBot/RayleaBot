package app

import "time"

const (
	managementChannelEvents = "events"
	managementEventReceived = "events.received"
)

type managementEventFrame struct {
	Channel   string `json:"channel"`
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Data      any    `json:"data"`
}

type serviceStatusEventPayload struct {
	ServiceStatus string   `json:"service_status"`
	Summary       string   `json:"summary"`
	Reason        string   `json:"reason,omitempty"`
	ReasonCodes   []string `json:"reason_codes,omitempty"`
}

type pluginStateEventPayload struct {
	PluginID          string                   `json:"plugin_id"`
	RegistrationState string                   `json:"registration_state"`
	DesiredState      string                   `json:"desired_state"`
	RuntimeState      string                   `json:"runtime_state"`
	DisplayState      string                   `json:"display_state"`
	Commands          []pluginCommandEventItem `json:"commands"`
	CommandConflicts  []string                 `json:"command_conflicts"`
}

type pluginCommandEventItem struct {
	Name          string   `json:"name"`
	Aliases       []string `json:"aliases,omitempty"`
	Description   string   `json:"description,omitempty"`
	Usage         string   `json:"usage,omitempty"`
	Permission    string   `json:"permission,omitempty"`
	CommandSource string   `json:"command_source"`
	DeclarationID string   `json:"declaration_id,omitempty"`
}

type genericManagementEventPayload struct {
	EventType string `json:"event_type"`
	Summary   string `json:"summary"`
}

type protocolSnapshotEventPayload struct {
	Protocol         string                       `json:"protocol"`
	ProtocolSnapshot oneBot11ProtocolSnapshotView `json:"protocol_snapshot"`
}

func newEventsReceivedFrame(data any) managementEventFrame {
	return managementEventFrame{
		Channel:   managementChannelEvents,
		Type:      managementEventReceived,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data:      data,
	}
}
