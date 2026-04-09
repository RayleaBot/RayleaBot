package adapter

import "time"

type State string
type TransportKey string
type TransportState string

const (
	StateIdle         State = "idle"
	StateConnecting   State = "connecting"
	StateConnected    State = "connected"
	StateAuthFailed   State = "auth_failed"
	StateReconnecting State = "reconnecting"
	StateStopped      State = "stopped"
)

const (
	TransportReverseWS TransportKey = "reverse_ws"
	TransportForwardWS TransportKey = "forward_ws"
	TransportHTTPAPI   TransportKey = "http_api"
	TransportWebhook   TransportKey = "webhook"
)

const (
	TransportStateIdle         TransportState = "idle"
	TransportStateListening    TransportState = "listening"
	TransportStateConnecting   TransportState = "connecting"
	TransportStateConnected    TransportState = "connected"
	TransportStateAuthFailed   TransportState = "auth_failed"
	TransportStateReconnecting TransportState = "reconnecting"
	TransportStateStopped      TransportState = "stopped"
)

type TransportSnapshot struct {
	Enabled          bool
	Configured       bool
	Endpoint         string
	State            TransportState
	LastErrorCode    string
	LastErrorMessage string
}

type Snapshot struct {
	State                 State
	ForwardWS             TransportSnapshot
	ReverseWS             TransportSnapshot
	HTTPAPI               TransportSnapshot
	Webhook               TransportSnapshot
	ActiveTransports      []TransportKey
	BotID                 string
	LastErrorCode         string
	LastErrorMessage      string
	ReadyFrameSeen        bool
	ConnectedAt           *time.Time
	LastFrameAt           *time.Time
	LastHeartbeatAt       *time.Time
	HeartbeatInterval     time.Duration
	TotalReceivedFrames   uint64
	InvalidReceivedFrames uint64
	HeartbeatSeen         bool
	LastFrameCategory     FrameCategory
	LastFrameType         string
}

func cloneSnapshot(snapshot Snapshot) Snapshot {
	cloned := snapshot
	cloned.ConnectedAt = cloneTime(snapshot.ConnectedAt)
	cloned.LastFrameAt = cloneTime(snapshot.LastFrameAt)
	cloned.LastHeartbeatAt = cloneTime(snapshot.LastHeartbeatAt)
	if len(snapshot.ActiveTransports) > 0 {
		cloned.ActiveTransports = append([]TransportKey(nil), snapshot.ActiveTransports...)
	}
	return cloned
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}

	copied := *value
	return &copied
}
