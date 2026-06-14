package shell

import (
	"strings"
	"time"

	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/intake"
)

type State string
type TransportKey string
type TransportState string

const (
	ProviderUnknown     = "unknown"
	ProviderStandard    = "standard"
	ProviderNapCat      = "napcat"
	ProviderLuckyLillia = "luckylillia"
)

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
	RuntimeInfo      TransportRuntimeInfo
}

type TransportRuntimeInfo struct {
	Provider        string
	AppName         string
	ProtocolVersion string
	AppVersion      string
	UserID          string
	Nickname        string
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
	LastFrameCategory     adapterintake.FrameCategory
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

func (snapshot Snapshot) DetectedProvider() string {
	for _, transport := range snapshot.ActiveTransports {
		info := snapshot.transportRuntimeInfo(transport)
		if info.Provider != "" && info.Provider != ProviderUnknown {
			return info.Provider
		}
	}
	return ProviderUnknown
}

func (snapshot Snapshot) transportRuntimeInfo(transport TransportKey) TransportRuntimeInfo {
	switch transport {
	case TransportForwardWS:
		return snapshot.ForwardWS.RuntimeInfo
	case TransportReverseWS:
		return snapshot.ReverseWS.RuntimeInfo
	case TransportHTTPAPI:
		return snapshot.HTTPAPI.RuntimeInfo
	case TransportWebhook:
		return snapshot.Webhook.RuntimeInfo
	default:
		return TransportRuntimeInfo{}
	}
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}

	copied := *value
	return &copied
}

func DetectProvider(appName string) string {
	normalized := strings.ToLower(strings.TrimSpace(appName))
	switch {
	case normalized == "":
		return ProviderUnknown
	case strings.Contains(normalized, "napcat"):
		return ProviderNapCat
	case strings.Contains(normalized, "llonebot"), strings.Contains(normalized, "luckylillia"):
		return ProviderLuckyLillia
	default:
		return ProviderStandard
	}
}

func (s *Shell) CurrentBotID() string {
	if s == nil {
		return ""
	}
	snapshot := s.Snapshot()
	if snapshot.State != StateConnected {
		return ""
	}
	return strings.TrimSpace(snapshot.BotID)
}

func (s *Shell) CurrentState() string {
	if s == nil {
		return string(StateIdle)
	}
	state := s.Snapshot().State
	if state == "" {
		return string(StateIdle)
	}
	return string(state)
}

func (s *Shell) DetectedProvider() string {
	if s == nil {
		return ProviderUnknown
	}
	return s.Snapshot().DetectedProvider()
}
