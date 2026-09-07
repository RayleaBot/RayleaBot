// Package wsevents holds the management WebSocket event domain: the frozen
// frame envelope, event payload types, and the services that project and
// broadcast protocol, governance, and service-status events.
package wsevents

import (
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

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
	PluginID         string                  `json:"plugin_id"`
	State            string                  `json:"state"`
	StateDiagnosis   *plugins.StateDiagnosis `json:"state_diagnosis,omitempty"`
	Commands         []PluginCommandItem     `json:"commands"`
	CommandConflicts []string                `json:"command_conflicts"`
}

type PluginCommandItem struct {
	ID             string               `json:"id"`
	Name           string               `json:"name"`
	EffectiveNames []string             `json:"effective_names"`
	Description    string               `json:"description"`
	Usage          string               `json:"usage"`
	Permission     string               `json:"permission"`
	Trigger        PluginCommandTrigger `json:"trigger"`
}

type PluginCommandTrigger struct {
	Type        string   `json:"type"`
	Names       []string `json:"names,omitempty"`
	Pattern     string   `json:"pattern,omitempty"`
	SettingsKey string   `json:"settings_key,omitempty"`
}

type GenericPayload struct {
	EventType string `json:"event_type"`
	Summary   string `json:"summary"`
}

type ProtocolSnapshotPayload struct {
	Protocol         string `json:"protocol"`
	ProtocolSnapshot any    `json:"protocol_snapshot"`
}

// AdaptersSnapshotPayload carries the state of every configured adapter
// instance. The OneBot snapshot describes one protocol in transport-level
// detail; this one says what every adapter is doing, which is what the adapter
// list needs and what an adapter with no transports of its own can report.
type AdaptersSnapshotPayload struct {
	Adapters []AdapterDescriptor `json:"adapters"`
}

func NewReceivedFrame(data any) Frame {
	return Frame{
		Channel:   channelEvents,
		Type:      eventReceived,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data:      data,
	}
}
