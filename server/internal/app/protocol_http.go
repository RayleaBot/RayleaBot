package app

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coder/websocket"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
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
	Transport  string `json:"transport"`
	Enabled    bool   `json:"enabled"`
	Configured bool   `json:"configured"`
	Endpoint   string `json:"endpoint"`
	State      string `json:"state"`
	Summary    string `json:"summary"`
}

type oneBot11ProtocolSnapshotResponse struct {
	Protocol              string                            `json:"protocol"`
	Provider              string                            `json:"provider"`
	ConfiguredTransports  []string                          `json:"configured_transports"`
	ActiveTransports      []string                          `json:"active_transports"`
	TransportStatus       []protocolTransportStatusResponse `json:"transport_status"`
	ReadinessStatus       string                            `json:"readiness_status"`
	Summary               string                            `json:"summary"`
	RecentTransportIssues []protocolIssueResponse           `json:"recent_transport_issues"`
}

func (a *App) handleProtocolOneBot11Snapshot() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeAuthJSON(w, http.StatusOK, a.currentOneBot11ProtocolSnapshot())
	}
}

func (a *App) handleProtocolOneBot11ReverseWS() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if a.Adapter == nil {
			writeAppError(w, r, http.StatusServiceUnavailable, "adapter.transport_reverse_ws_upgrade_failed", "OneBot 回连入口不可用", "errors.adapter.transport_reverse_ws_upgrade_failed", nil)
			return
		}
		if strings.TrimSpace(a.Config.OneBot.AccessToken) == "" {
			a.Logger.Warn("onebot reverse websocket ingress accepted without access token", "component", "adapter", "transport", "reverse_ws")
		}
		if !allowOneBotIngress(r, a.Config.OneBot.AccessToken) {
			a.Adapter.MarkReverseWSAuthFailed()
			writeAppError(w, r, http.StatusUnauthorized, "adapter.transport_reverse_ws_auth_failed", "协议鉴权失败", "errors.adapter.transport_reverse_ws_auth_failed", nil)
			return
		}

		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		a.Adapter.AttachReverseWS(conn)
	}
}

func (a *App) handleProtocolOneBot11Webhook() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if a.Adapter == nil {
			writeAppError(w, r, http.StatusServiceUnavailable, "adapter.transport_webhook_invalid_payload", "OneBot Webhook 不可用", "errors.adapter.transport_webhook_invalid_payload", nil)
			return
		}
		if strings.TrimSpace(a.Config.OneBot.AccessToken) == "" {
			a.Logger.Warn("onebot webhook ingress accepted without access token", "component", "adapter", "transport", "webhook")
		}
		if !allowOneBotIngress(r, a.Config.OneBot.AccessToken) {
			a.Adapter.MarkWebhookAuthFailed()
			writeAppError(w, r, http.StatusUnauthorized, "adapter.transport_webhook_auth_failed", "协议鉴权失败", "errors.adapter.transport_webhook_auth_failed", nil)
			return
		}

		payload, err := readRequestBody(w, r, maxWebhookBodyBytes)
		if err != nil {
			writeAppError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}
		if err := a.Adapter.AcceptWebhookPayload(r.Context(), payload); err != nil {
			writeAppError(w, r, http.StatusBadRequest, "adapter.transport_webhook_invalid_payload", "OneBot Webhook 负载不合法", "errors.adapter.transport_webhook_invalid_payload", nil)
			return
		}
		writeAuthJSON(w, http.StatusAccepted, systemShutdownResponse{Accepted: true})
	}
}

func (a *App) currentOneBot11ProtocolSnapshot() oneBot11ProtocolSnapshotResponse {
	adapterSnapshot := adapter.Snapshot{}
	if a.Adapter != nil {
		adapterSnapshot = a.Adapter.Snapshot()
	}

	transports := []struct {
		key      adapter.TransportKey
		snapshot adapter.TransportSnapshot
	}{
		{key: adapter.TransportReverseWS, snapshot: adapterSnapshot.ReverseWS},
		{key: adapter.TransportForwardWS, snapshot: adapterSnapshot.ForwardWS},
		{key: adapter.TransportHTTPAPI, snapshot: adapterSnapshot.HTTPAPI},
		{key: adapter.TransportWebhook, snapshot: adapterSnapshot.Webhook},
	}

	configured := make([]string, 0, len(transports))
	status := make([]protocolTransportStatusResponse, 0, len(transports))
	for _, transport := range transports {
		if transport.snapshot.Configured {
			configured = append(configured, string(transport.key))
		}
		status = append(status, protocolTransportStatusResponse{
			Transport:  string(transport.key),
			Enabled:    transport.snapshot.Enabled,
			Configured: transport.snapshot.Configured,
			Endpoint:   transport.snapshot.Endpoint,
			State:      string(transport.snapshot.State),
			Summary:    protocolTransportSummary(transport.key, transport.snapshot),
		})
	}

	active := make([]string, 0, len(adapterSnapshot.ActiveTransports))
	for _, key := range adapterSnapshot.ActiveTransports {
		active = append(active, string(key))
	}

	readiness := protocolReadinessStatus(adapterSnapshot)
	return oneBot11ProtocolSnapshotResponse{
		Protocol:              "onebot11",
		Provider:              currentOneBotProvider(a.Config.OneBot.Provider),
		ConfiguredTransports:  configured,
		ActiveTransports:      active,
		TransportStatus:       status,
		ReadinessStatus:       readiness,
		Summary:               protocolSnapshotSummary(adapterSnapshot, readiness),
		RecentTransportIssues: protocolIssuesFromSnapshot(adapterSnapshot),
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

func (a *App) publishProtocolSnapshot() {
	a.publishProtocolEvent(a.protocolSnapshotEvent())
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

func protocolReadinessStatus(snapshot adapter.Snapshot) string {
	outboundReady := snapshot.ForwardWS.State == adapter.TransportStateConnected ||
		snapshot.ReverseWS.State == adapter.TransportStateConnected ||
		snapshot.HTTPAPI.State == adapter.TransportStateConnected
	inboundReady := snapshot.ReverseWS.State == adapter.TransportStateConnected ||
		snapshot.Webhook.State == adapter.TransportStateListening ||
		snapshot.Webhook.State == adapter.TransportStateConnected

	configuredAny := snapshot.ForwardWS.Configured || snapshot.ReverseWS.Configured || snapshot.HTTPAPI.Configured || snapshot.Webhook.Configured
	if !configuredAny {
		return "setup_required"
	}
	if snapshot.ForwardWS.State == adapter.TransportStateConnected || snapshot.ReverseWS.State == adapter.TransportStateConnected {
		return "ready"
	}
	if outboundReady && inboundReady {
		return "ready"
	}
	if outboundReady || inboundReady || len(snapshot.ActiveTransports) > 0 {
		return "degraded"
	}
	return "failed"
}

func protocolSnapshotSummary(snapshot adapter.Snapshot, readiness string) string {
	switch readiness {
	case "ready":
		if snapshot.ForwardWS.State == adapter.TransportStateConnected {
			return "OneBot11 主动连接已就绪"
		}
		if snapshot.ReverseWS.State == adapter.TransportStateConnected {
			return "OneBot11 回连链路已就绪"
		}
		return "OneBot11 HTTP API 与 Webhook 已就绪"
	case "degraded":
		if snapshot.HTTPAPI.State == adapter.TransportStateConnected && (snapshot.Webhook.State == adapter.TransportStateListening || snapshot.Webhook.State == adapter.TransportStateConnected) {
			return "OneBot11 HTTP API 与 Webhook 可用，但尚未建立 WebSocket 会话"
		}
		if snapshot.HTTPAPI.State == adapter.TransportStateConnected {
			return "OneBot11 仅 HTTP API 可用"
		}
		if snapshot.Webhook.State == adapter.TransportStateListening || snapshot.Webhook.State == adapter.TransportStateConnected {
			return "OneBot11 仅 Webhook 上报可用"
		}
		if snapshot.ReverseWS.State == adapter.TransportStateListening {
			return "OneBot11 等待回连"
		}
		if snapshot.ForwardWS.State == adapter.TransportStateConnecting || snapshot.ForwardWS.State == adapter.TransportStateReconnecting {
			return "OneBot11 正在建立主动连接"
		}
		return "OneBot11 传输链路部分可用"
	case "failed":
		return "OneBot11 传输链路不可用"
	default:
		return "OneBot11 尚未配置连接"
	}
}

func protocolTransportSummary(key adapter.TransportKey, snapshot adapter.TransportSnapshot) string {
	if !snapshot.Enabled || !snapshot.Configured {
		return "未启用"
	}

	switch key {
	case adapter.TransportReverseWS:
		switch snapshot.State {
		case adapter.TransportStateConnected:
			return "OneBot 已回连"
		case adapter.TransportStateAuthFailed:
			return "最近一次回连鉴权失败"
		case adapter.TransportStateStopped:
			return "回连入口已停止"
		default:
			return "等待 OneBot 回连"
		}
	case adapter.TransportForwardWS:
		switch snapshot.State {
		case adapter.TransportStateConnected:
			return "主动连接已建立"
		case adapter.TransportStateConnecting:
			return "正在主动连接"
		case adapter.TransportStateReconnecting:
			return "连接已断开，正在重试"
		case adapter.TransportStateAuthFailed:
			return "主动连接鉴权失败"
		case adapter.TransportStateStopped:
			return "主动连接已停止"
		default:
			return "等待主动连接"
		}
	case adapter.TransportHTTPAPI:
		switch snapshot.State {
		case adapter.TransportStateConnected:
			return "HTTP API 可用"
		case adapter.TransportStateAuthFailed:
			return "HTTP API 鉴权失败"
		case adapter.TransportStateReconnecting:
			return "HTTP API 请求失败，等待重试"
		case adapter.TransportStateStopped:
			return "HTTP API 已停止"
		default:
			return "HTTP API 未验证"
		}
	case adapter.TransportWebhook:
		switch snapshot.State {
		case adapter.TransportStateConnected:
			return "Webhook 正在接收上报"
		case adapter.TransportStateAuthFailed:
			return "Webhook 鉴权失败"
		case adapter.TransportStateStopped:
			return "Webhook 入口已停止"
		default:
			return "Webhook 入口可接收上报"
		}
	default:
		return "未启用"
	}
}

func protocolIssuesFromSnapshot(snapshot adapter.Snapshot) []protocolIssueResponse {
	issues := make([]protocolIssueResponse, 0, 4)
	appendIssue := func(code, summary string) {
		if strings.TrimSpace(code) == "" {
			return
		}
		issues = append(issues, protocolIssueResponse{
			Code:     code,
			Severity: "warning",
			Summary:  summary,
		})
	}

	appendIssue(snapshot.ForwardWS.LastErrorCode, transportIssueSummary(snapshot.ForwardWS))
	appendIssue(snapshot.ReverseWS.LastErrorCode, transportIssueSummary(snapshot.ReverseWS))
	appendIssue(snapshot.HTTPAPI.LastErrorCode, transportIssueSummary(snapshot.HTTPAPI))
	appendIssue(snapshot.Webhook.LastErrorCode, transportIssueSummary(snapshot.Webhook))
	return issues
}

func transportIssueSummary(snapshot adapter.TransportSnapshot) string {
	if strings.TrimSpace(snapshot.LastErrorMessage) != "" {
		return snapshot.LastErrorMessage
	}
	return "OneBot 传输链路出现异常"
}

func allowOneBotIngress(r *http.Request, accessToken string) bool {
	trimmedToken := strings.TrimSpace(accessToken)
	if trimmedToken == "" {
		return true
	}

	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		if strings.TrimSpace(authHeader[7:]) == trimmedToken {
			return true
		}
	}
	if strings.TrimSpace(r.URL.Query().Get("access_token")) == trimmedToken {
		return true
	}
	return false
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
