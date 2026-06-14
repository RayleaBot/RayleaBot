package adapter

import adaptershell "github.com/RayleaBot/RayleaBot/server/internal/adapter/shell"

type State = adaptershell.State
type TransportKey = adaptershell.TransportKey
type TransportState = adaptershell.TransportState

const (
	StateIdle         = adaptershell.StateIdle
	StateConnecting   = adaptershell.StateConnecting
	StateConnected    = adaptershell.StateConnected
	StateAuthFailed   = adaptershell.StateAuthFailed
	StateReconnecting = adaptershell.StateReconnecting
	StateStopped      = adaptershell.StateStopped
)

const (
	TransportReverseWS = adaptershell.TransportReverseWS
	TransportForwardWS = adaptershell.TransportForwardWS
	TransportHTTPAPI   = adaptershell.TransportHTTPAPI
	TransportWebhook   = adaptershell.TransportWebhook
)

const (
	TransportStateIdle         = adaptershell.TransportStateIdle
	TransportStateListening    = adaptershell.TransportStateListening
	TransportStateConnecting   = adaptershell.TransportStateConnecting
	TransportStateConnected    = adaptershell.TransportStateConnected
	TransportStateAuthFailed   = adaptershell.TransportStateAuthFailed
	TransportStateReconnecting = adaptershell.TransportStateReconnecting
	TransportStateStopped      = adaptershell.TransportStateStopped
)

type TransportSnapshot = adaptershell.TransportSnapshot
type TransportRuntimeInfo = adaptershell.TransportRuntimeInfo
type Snapshot = adaptershell.Snapshot
