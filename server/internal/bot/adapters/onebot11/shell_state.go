package onebot11

import (
	"strings"
	"time"
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
	AccountID        string
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

// shellTransports lists every transport in the order their login info is
// consulted when the account behind an event needs a name.
var shellTransports = []TransportKey{TransportForwardWS, TransportReverseWS, TransportHTTPAPI, TransportWebhook}

// loginNickname reports the nickname get_login_info returned for botID. Login
// info is recorded per transport, so a transport signed in as another account
// is not consulted; one that reported no account id cannot contradict botID
// and is trusted.
func (snapshot Snapshot) loginNickname(botID string) string {
	botID = strings.TrimSpace(botID)
	for _, transport := range shellTransports {
		info := snapshot.transportRuntimeInfo(transport)
		nickname := strings.TrimSpace(info.Nickname)
		if nickname == "" {
			continue
		}
		if userID := strings.TrimSpace(info.UserID); userID == "" || userID == botID {
			return nickname
		}
	}
	return ""
}

// loginUserID reports the first account id any transport's login info named.
func (snapshot Snapshot) loginUserID() string {
	for _, transport := range shellTransports {
		if userID := strings.TrimSpace(snapshot.transportRuntimeInfo(transport).UserID); userID != "" {
			return userID
		}
	}
	return ""
}

// ResolveBotDisplay names the account this instance is signed in as, for the
// outbound log line. Another instance's question is not answered: its login
// is a different account.
func (s *Shell) ResolveBotDisplay(adapterID string) (string, string) {
	if adapterID != "" && s.adapterID != "" && adapterID != s.adapterID {
		return "", ""
	}
	snapshot := s.Snapshot()
	botID := strings.TrimSpace(snapshot.BotID)
	if botID == "" {
		botID = snapshot.loginUserID()
	}
	return botID, snapshot.loginNickname(botID)
}

func (s *Shell) CurrentBotID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.snapshot.State != StateConnected {
		return ""
	}
	return strings.TrimSpace(s.snapshot.BotID)
}

// ConfirmedAccount is the current transport identity used by the management
// directory. HTTP login information and valid Webhook self_id are sufficient;
// the plugin identity list and outbound routing keep CurrentBotID semantics.
func (s *Shell) ConfirmedAccount() (string, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for index, transport := range []TransportSnapshot{s.snapshot.ReverseWS, s.snapshot.ForwardWS, s.snapshot.HTTPAPI, s.snapshot.Webhook} {
		// A webhook never connects; while it listens, a valid self_id confirms the account.
		active := transport.State == TransportStateConnected || index == 3 && transport.State == TransportStateListening
		if !transport.Enabled || !active {
			continue
		}
		id := transport.AccountID
		if id == "" {
			id = transport.RuntimeInfo.UserID
		}
		if id == "" {
			continue
		}
		name := ""
		for _, profile := range []TransportSnapshot{s.snapshot.ReverseWS, s.snapshot.ForwardWS, s.snapshot.HTTPAPI} {
			if profile.Enabled && profile.State == TransportStateConnected && profile.RuntimeInfo.UserID == id && profile.RuntimeInfo.Nickname != "" {
				name = profile.RuntimeInfo.Nickname
				break
			}
		}
		return id, name
	}
	return "", ""
}

func (s *Shell) DetectedProvider() string {
	return s.Snapshot().DetectedProvider()
}
