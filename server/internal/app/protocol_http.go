package app

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/health"
)

type managementEventFrame struct {
	Channel   string `json:"channel"`
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Data      any    `json:"data"`
}

type protocolIssueResponse struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Summary  string `json:"summary"`
}

type protocolTransportStatusResponse struct {
	Transport   string `json:"transport"`
	Enabled     bool   `json:"enabled"`
	Configured  bool   `json:"configured"`
	Implemented bool   `json:"implemented"`
	Active      bool   `json:"active"`
	Endpoint    string `json:"endpoint"`
}

type oneBot11ProtocolSnapshotResponse struct {
	Protocol              string                            `json:"protocol"`
	Provider              string                            `json:"provider"`
	ConfiguredTransports  []string                          `json:"configured_transports"`
	ActiveTransport       *string                           `json:"active_transport,omitempty"`
	TransportStatus       []protocolTransportStatusResponse `json:"transport_status"`
	ConnectionState       string                            `json:"connection_state"`
	ReadinessStatus       string                            `json:"readiness_status"`
	Summary               string                            `json:"summary"`
	RecentTransportIssues []protocolIssueResponse           `json:"recent_transport_issues"`
}

type protocolCompatibilityItemResponse struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	Provider string `json:"provider,omitempty"`
	Notes    string `json:"notes,omitempty"`
}

type protocolCompatibilityGroupResponse struct {
	Group string                              `json:"group"`
	Title string                              `json:"title"`
	Items []protocolCompatibilityItemResponse `json:"items"`
}

type oneBot11ProtocolCompatibilityResponse struct {
	Protocol    string                               `json:"protocol"`
	Provider    string                               `json:"provider"`
	GeneratedAt string                               `json:"generated_at"`
	Groups      []protocolCompatibilityGroupResponse `json:"groups"`
}

func (a *App) handleProtocolOneBot11Snapshot() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeAuthJSON(w, http.StatusOK, a.currentOneBot11ProtocolSnapshot())
	}
}

func (a *App) handleProtocolOneBot11Compatibility() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeAuthJSON(w, http.StatusOK, a.currentOneBot11ProtocolCompatibility())
	}
}

func (a *App) currentOneBot11ProtocolSnapshot() oneBot11ProtocolSnapshotResponse {
	cfg := a.Config
	report := a.currentReadiness()
	adapterSnapshot := adapter.Snapshot{}
	if a.Adapter != nil {
		adapterSnapshot = a.Adapter.Snapshot()
	}

	reverseWS := cfg.OneBot.ReverseWS
	if reverseWS.URL == "" && cfg.OneBot.WSURL != "" {
		reverseWS.URL = cfg.OneBot.WSURL
	}
	if reverseWS.URL != "" {
		reverseWS.Enabled = true
	}

	transports := []struct {
		key    string
		config struct {
			Enabled bool
			URL     string
		}
		implemented bool
	}{
		{key: "reverse_ws", config: struct {
			Enabled bool
			URL     string
		}{Enabled: reverseWS.Enabled, URL: reverseWS.URL}, implemented: true},
		{key: "forward_ws", config: struct {
			Enabled bool
			URL     string
		}{Enabled: cfg.OneBot.ForwardWS.Enabled, URL: cfg.OneBot.ForwardWS.URL}, implemented: false},
		{key: "http_api", config: struct {
			Enabled bool
			URL     string
		}{Enabled: cfg.OneBot.HTTPAPI.Enabled, URL: cfg.OneBot.HTTPAPI.URL}, implemented: false},
		{key: "webhook", config: struct {
			Enabled bool
			URL     string
		}{Enabled: cfg.OneBot.Webhook.Enabled, URL: cfg.OneBot.Webhook.URL}, implemented: false},
		{key: "sse", config: struct {
			Enabled bool
			URL     string
		}{Enabled: cfg.OneBot.SSE.Enabled, URL: cfg.OneBot.SSE.URL}, implemented: false},
	}

	configuredTransports := make([]string, 0, len(transports))
	var activeTransport *string
	if reverseWS.Enabled && reverseWS.URL != "" {
		switch stateOrIdle(adapterSnapshot.State) {
		case adapter.StateConnecting, adapter.StateConnected, adapter.StateAuthFailed, adapter.StateReconnecting, adapter.StateStopped:
			key := "reverse_ws"
			activeTransport = &key
		}
	}

	transportStatus := make([]protocolTransportStatusResponse, 0, len(transports))
	for _, transport := range transports {
		configured := transport.config.Enabled && strings.TrimSpace(transport.config.URL) != ""
		if configured {
			configuredTransports = append(configuredTransports, transport.key)
		}
		transportStatus = append(transportStatus, protocolTransportStatusResponse{
			Transport:   transport.key,
			Enabled:     transport.config.Enabled,
			Configured:  configured,
			Implemented: transport.implemented,
			Active:      activeTransport != nil && *activeTransport == transport.key,
			Endpoint:    sanitizeProtocolEndpoint(transport.config.URL),
		})
	}

	return oneBot11ProtocolSnapshotResponse{
		Protocol:              "onebot11",
		Provider:              currentOneBotProvider(cfg.OneBot.Provider),
		ConfiguredTransports:  configuredTransports,
		ActiveTransport:       activeTransport,
		TransportStatus:       transportStatus,
		ConnectionState:       string(stateOrIdle(adapterSnapshot.State)),
		ReadinessStatus:       string(report.Status),
		Summary:               protocolSnapshotSummary(report, adapterSnapshot, activeTransport),
		RecentTransportIssues: protocolIssuesFromReadiness(report),
	}
}

func (a *App) currentOneBot11ProtocolCompatibility() oneBot11ProtocolCompatibilityResponse {
	provider := currentOneBotProvider(a.Config.OneBot.Provider)
	return oneBot11ProtocolCompatibilityResponse{
		Protocol:    "onebot11",
		Provider:    provider,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Groups: []protocolCompatibilityGroupResponse{
			{
				Group: "core",
				Title: "核心事件与查询",
				Items: []protocolCompatibilityItemResponse{
					{Name: "message.group", Status: "implemented"},
					{Name: "message.private", Status: "implemented"},
					{Name: "notice.group_admin", Status: "partial"},
					{Name: "notice.group_ban", Status: "partial"},
					{Name: "request.friend", Status: "frozen"},
					{Name: "request.group", Status: "frozen"},
					{Name: "meta.lifecycle", Status: "implemented"},
					{Name: "meta.heartbeat", Status: "implemented"},
					{Name: "message.history", Status: "frozen"},
					{Name: "message.forward", Status: "frozen"},
				},
			},
			{
				Group: "segment",
				Title: "消息段兼容",
				Items: []protocolCompatibilityItemResponse{
					{Name: "text", Status: "implemented"},
					{Name: "image", Status: "implemented"},
					{Name: "at", Status: "implemented"},
					{Name: "reply", Status: "implemented"},
					{Name: "face", Status: "implemented"},
					{Name: "record", Status: "frozen"},
					{Name: "video", Status: "frozen"},
					{Name: "file", Status: "frozen"},
					{Name: "json", Status: "frozen"},
					{Name: "xml", Status: "frozen"},
					{Name: "markdown", Status: "frozen"},
					{Name: "music", Status: "frozen"},
					{Name: "contact", Status: "frozen"},
					{Name: "forward", Status: "frozen"},
					{Name: "node", Status: "frozen"},
					{Name: "poke", Status: "frozen"},
					{Name: "dice", Status: "frozen"},
					{Name: "rps", Status: "frozen"},
					{Name: "mface", Status: "frozen"},
					{Name: "keyboard", Status: "frozen"},
					{Name: "shake", Status: "frozen"},
				},
			},
			{
				Group: "action",
				Title: "动作族",
				Items: []protocolCompatibilityItemResponse{
					{Name: "message.send", Status: "implemented"},
					{Name: "message.reply", Status: "implemented"},
					{Name: "user.info.get", Status: "implemented"},
					{Name: "group.info.get", Status: "implemented"},
					{Name: "group.member.get", Status: "implemented"},
					{Name: "message.get", Status: "frozen"},
					{Name: "message.history.get", Status: "frozen"},
					{Name: "message.forward.send", Status: "frozen"},
					{Name: "group.announcement.create", Status: "frozen"},
					{Name: "file.group.upload", Status: "frozen"},
					{Name: "reaction.set", Status: "frozen"},
					{Name: "poke.send", Status: "frozen"},
				},
			},
			{
				Group: "provider_extension",
				Title: "Provider 扩展",
				Items: []protocolCompatibilityItemResponse{
					{Name: "provider.napcat.message_emoji.like.set", Status: "frozen", Provider: "napcat"},
					{Name: "provider.napcat.group.sign.set", Status: "frozen", Provider: "napcat"},
					{Name: "provider.luckylillia.friend_groups.get", Status: "frozen", Provider: "luckylillia"},
					{Name: "provider.luckylillia.sse.receive", Status: "partial", Provider: "luckylillia"},
				},
			},
		},
	}
}

func (a *App) protocolSnapshotEvent() managementEventFrame {
	return managementEventFrame{
		Channel:   "events",
		Type:      "events.received",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data: map[string]any{
			"protocol":          "onebot11",
			"protocol_snapshot": a.currentOneBot11ProtocolSnapshot(),
		},
	}
}

func (a *App) protocolCompatibilityEvent() managementEventFrame {
	return managementEventFrame{
		Channel:   "events",
		Type:      "events.received",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data: map[string]any{
			"protocol":               "onebot11",
			"protocol_compatibility": a.currentOneBot11ProtocolCompatibility(),
		},
	}
}

func (a *App) publishProtocolSnapshot() {
	a.publishProtocolEvent(a.protocolSnapshotEvent())
}

func (a *App) publishProtocolCompatibility() {
	a.publishProtocolEvent(a.protocolCompatibilityEvent())
}

func (a *App) publishProtocolEvent(frame managementEventFrame) {
	if a == nil {
		return
	}

	a.protocolMu.RLock()
	subscribers := make([]chan managementEventFrame, 0, len(a.protocolSubscribers))
	for _, subscriber := range a.protocolSubscribers {
		subscribers = append(subscribers, subscriber)
	}
	a.protocolMu.RUnlock()

	for _, subscriber := range subscribers {
		select {
		case subscriber <- frame:
		default:
		}
	}
}

func (a *App) subscribeProtocolEvents(buffer int) (<-chan managementEventFrame, func()) {
	if buffer <= 0 {
		buffer = 1
	}

	ch := make(chan managementEventFrame, buffer)
	a.protocolMu.Lock()
	id := a.nextProtocolSubID
	a.nextProtocolSubID++
	a.protocolSubscribers[id] = ch
	a.protocolMu.Unlock()

	return ch, func() {
		a.protocolMu.Lock()
		defer a.protocolMu.Unlock()
		subscriber, ok := a.protocolSubscribers[id]
		if !ok {
			return
		}
		delete(a.protocolSubscribers, id)
		close(subscriber)
	}
}

func protocolIssuesFromReadiness(report health.ReadinessReport) []protocolIssueResponse {
	issues := make([]protocolIssueResponse, 0, len(report.Issues))
	for _, issue := range report.Issues {
		if !strings.HasPrefix(issue.Code, "adapter.") {
			continue
		}
		issues = append(issues, protocolIssueResponse{
			Code:     issue.Code,
			Severity: issue.Severity,
			Summary:  issue.Summary,
		})
	}
	return issues
}

func protocolSnapshotSummary(report health.ReadinessReport, snapshot adapter.Snapshot, activeTransport *string) string {
	switch stateOrIdle(snapshot.State) {
	case adapter.StateConnected:
		return "OneBot11 reverse WebSocket 已连接"
	case adapter.StateConnecting:
		return "OneBot11 reverse WebSocket 正在连接"
	case adapter.StateAuthFailed:
		return "OneBot11 鉴权失败，请检查访问令牌"
	case adapter.StateReconnecting:
		return "OneBot11 正在重连"
	case adapter.StateStopped:
		return "OneBot11 连接已停止"
	case adapter.StateIdle:
		if activeTransport != nil {
			return "OneBot11 已配置，等待建立连接"
		}
	}
	if strings.TrimSpace(report.Reason) != "" {
		return report.Reason
	}
	return "OneBot11 尚未配置连接"
}

func currentOneBotProvider(raw string) string {
	switch strings.TrimSpace(raw) {
	case "napcat", "luckylillia":
		return strings.TrimSpace(raw)
	default:
		return "standard"
	}
}

func sanitizeProtocolEndpoint(raw string) string {
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}
