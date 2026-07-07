package wsevents

import (
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/onebot11"
)

func (s *ProtocolService) CurrentOneBot11ProtocolSnapshot() OneBot11ProtocolSnapshot {
	adapterSnapshot := onebot11.Snapshot{}
	if s.adapter != nil {
		adapterSnapshot = s.adapter.Snapshot()
	}

	transports := []struct {
		key      onebot11.TransportKey
		snapshot onebot11.TransportSnapshot
	}{
		{key: onebot11.TransportReverseWS, snapshot: adapterSnapshot.ReverseWS},
		{key: onebot11.TransportForwardWS, snapshot: adapterSnapshot.ForwardWS},
		{key: onebot11.TransportHTTPAPI, snapshot: adapterSnapshot.HTTPAPI},
		{key: onebot11.TransportWebhook, snapshot: adapterSnapshot.Webhook},
	}

	configured := make([]string, 0, len(transports))
	status := make([]TransportStatus, 0, len(transports))
	for _, transport := range transports {
		if transport.snapshot.Configured {
			configured = append(configured, string(transport.key))
		}
		runtimeInfo := transport.snapshot.RuntimeInfo
		status = append(status, TransportStatus{
			Transport:       string(transport.key),
			Enabled:         transport.snapshot.Enabled,
			Configured:      transport.snapshot.Configured,
			Endpoint:        transport.snapshot.Endpoint,
			State:           string(transport.snapshot.State),
			Summary:         protocolTransportSummary(transport.key, transport.snapshot),
			Provider:        currentOneBotProvider(runtimeInfo.Provider),
			AppName:         runtimeInfo.AppName,
			ProtocolVersion: runtimeInfo.ProtocolVersion,
			AppVersion:      runtimeInfo.AppVersion,
			UserID:          runtimeInfo.UserID,
			Nickname:        runtimeInfo.Nickname,
		})
	}

	active := make([]string, 0, len(adapterSnapshot.ActiveTransports))
	for _, key := range adapterSnapshot.ActiveTransports {
		active = append(active, string(key))
	}

	readiness := protocolReadinessStatus(adapterSnapshot)
	return OneBot11ProtocolSnapshot{
		Protocol:              "onebot11",
		Provider:              adapterSnapshot.DetectedProvider(),
		ConfiguredTransports:  configured,
		ActiveTransports:      active,
		TransportStatus:       status,
		ReadinessStatus:       readiness,
		Summary:               protocolSnapshotSummary(adapterSnapshot, readiness),
		RecentTransportIssues: protocolIssuesFromSnapshot(adapterSnapshot),
	}
}

func (s *ProtocolService) transportIngressEnabled(transport onebot11.TransportKey) bool {
	if s.adapter == nil {
		return false
	}

	snapshot := s.adapter.Snapshot()
	switch transport {
	case onebot11.TransportReverseWS:
		return snapshot.ReverseWS.Enabled && snapshot.ReverseWS.Configured
	case onebot11.TransportWebhook:
		return snapshot.Webhook.Enabled && snapshot.Webhook.Configured
	default:
		return false
	}
}

func protocolIssuesFromSnapshot(snapshot onebot11.Snapshot) []ProtocolIssue {
	issues := make([]ProtocolIssue, 0, 4)
	appendIssue := func(transport onebot11.TransportKey, transportSnapshot onebot11.TransportSnapshot) {
		code := strings.TrimSpace(transportSnapshot.LastErrorCode)
		if code == "" {
			return
		}
		issues = append(issues, ProtocolIssue{
			Code:     code,
			Severity: "warning",
			Summary:  transportIssueSummary(transport, transportSnapshot),
		})
	}

	appendIssue(onebot11.TransportForwardWS, snapshot.ForwardWS)
	appendIssue(onebot11.TransportReverseWS, snapshot.ReverseWS)
	appendIssue(onebot11.TransportHTTPAPI, snapshot.HTTPAPI)
	appendIssue(onebot11.TransportWebhook, snapshot.Webhook)
	return issues
}

func transportIssueSummary(transport onebot11.TransportKey, snapshot onebot11.TransportSnapshot) string {
	switch transport {
	case onebot11.TransportForwardWS:
		switch snapshot.State {
		case onebot11.TransportStateAuthFailed:
			return "OneBot 主动连接鉴权失败，请检查访问令牌。"
		case onebot11.TransportStateReconnecting:
			return "OneBot 主动连接已断开，正在重试。"
		}
		switch strings.TrimSpace(snapshot.LastErrorCode) {
		case "adapter.transport_forward_ws_session_lost", "adapter.connection_lost":
			return "OneBot 主动连接已断开，正在重试。"
		case "adapter.transport_forward_ws_connection_failed":
			return "OneBot 主动连接失败，请检查地址与网络。"
		}
		return "OneBot 主动连接出现异常。"
	case onebot11.TransportReverseWS:
		switch snapshot.State {
		case onebot11.TransportStateAuthFailed:
			return "OneBot 回连鉴权失败，请检查访问令牌。"
		case onebot11.TransportStateConnected:
			return "OneBot 回连链路已恢复。"
		}
		switch strings.TrimSpace(snapshot.LastErrorCode) {
		case "adapter.transport_reverse_ws_auth_failed":
			return "OneBot 回连鉴权失败，请检查访问令牌。"
		case "adapter.connection_lost":
			return "OneBot 回连会话已断开，请让 OneBot 重新回连。"
		}
		return "OneBot 回连链路出现异常。"
	case onebot11.TransportHTTPAPI:
		switch snapshot.State {
		case onebot11.TransportStateAuthFailed:
			return "OneBot HTTP API 鉴权失败，请检查访问令牌。"
		case onebot11.TransportStateConnected:
			return "OneBot HTTP API 已恢复可用。"
		}
		switch strings.TrimSpace(snapshot.LastErrorCode) {
		case "adapter.transport_http_api_auth_failed":
			return "OneBot HTTP API 鉴权失败，请检查访问令牌。"
		case "adapter.transport_http_api_invalid_response":
			return "OneBot HTTP API 返回无效响应。"
		case "adapter.transport_http_api_request_failed", "adapter.connection_lost":
			return "OneBot HTTP API 请求失败，请检查地址与网络。"
		}
		return "OneBot HTTP API 出现异常。"
	case onebot11.TransportWebhook:
		switch snapshot.State {
		case onebot11.TransportStateAuthFailed:
			return "OneBot Webhook 鉴权失败，请检查访问令牌。"
		case onebot11.TransportStateListening, onebot11.TransportStateConnected:
			if strings.TrimSpace(snapshot.LastErrorCode) == "" {
				return "OneBot Webhook 入口运行正常。"
			}
		}
		switch strings.TrimSpace(snapshot.LastErrorCode) {
		case "adapter.transport_webhook_auth_failed":
			return "OneBot Webhook 鉴权失败，请检查访问令牌。"
		case "adapter.transport_webhook_invalid_payload":
			return "OneBot Webhook 上报格式无效。"
		case "adapter.transport_webhook_duplicate_event":
			return "OneBot Webhook 收到重复事件，已自动忽略。"
		}
		return "OneBot Webhook 入口出现异常。"
	default:
		return "OneBot 传输链路出现异常。"
	}
}

func protocolReadinessStatus(snapshot onebot11.Snapshot) string {
	outboundReady := snapshot.ForwardWS.State == onebot11.TransportStateConnected ||
		snapshot.ReverseWS.State == onebot11.TransportStateConnected ||
		snapshot.HTTPAPI.State == onebot11.TransportStateConnected
	inboundReady := snapshot.ReverseWS.State == onebot11.TransportStateConnected ||
		snapshot.Webhook.State == onebot11.TransportStateListening ||
		snapshot.Webhook.State == onebot11.TransportStateConnected

	configuredAny := snapshot.ForwardWS.Configured || snapshot.ReverseWS.Configured || snapshot.HTTPAPI.Configured || snapshot.Webhook.Configured
	if !configuredAny {
		return "setup_required"
	}
	if snapshot.ForwardWS.State == onebot11.TransportStateConnected || snapshot.ReverseWS.State == onebot11.TransportStateConnected {
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

func protocolSnapshotSummary(snapshot onebot11.Snapshot, readiness string) string {
	switch readiness {
	case "ready":
		if snapshot.ForwardWS.State == onebot11.TransportStateConnected {
			return "OneBot11 主动连接已就绪"
		}
		if snapshot.ReverseWS.State == onebot11.TransportStateConnected {
			return "OneBot11 回连链路已就绪"
		}
		return "OneBot11 HTTP API 与 Webhook 已就绪"
	case "degraded":
		if snapshot.HTTPAPI.State == onebot11.TransportStateConnected && (snapshot.Webhook.State == onebot11.TransportStateListening || snapshot.Webhook.State == onebot11.TransportStateConnected) {
			return "OneBot11 HTTP API 与 Webhook 可用，但尚未建立 WebSocket 会话"
		}
		if snapshot.HTTPAPI.State == onebot11.TransportStateConnected {
			return "OneBot11 仅 HTTP API 可用"
		}
		if snapshot.Webhook.State == onebot11.TransportStateListening || snapshot.Webhook.State == onebot11.TransportStateConnected {
			return "OneBot11 仅 Webhook 上报可用"
		}
		if snapshot.ReverseWS.State == onebot11.TransportStateListening {
			return "OneBot11 等待回连"
		}
		if snapshot.ForwardWS.State == onebot11.TransportStateConnecting || snapshot.ForwardWS.State == onebot11.TransportStateReconnecting {
			return "OneBot11 正在建立主动连接"
		}
		return "OneBot11 传输链路部分可用"
	case "failed":
		return "OneBot11 传输链路不可用"
	default:
		return "OneBot11 尚未配置连接"
	}
}

func protocolTransportSummary(key onebot11.TransportKey, snapshot onebot11.TransportSnapshot) string {
	if !snapshot.Enabled || !snapshot.Configured {
		return "未启用"
	}

	switch key {
	case onebot11.TransportReverseWS:
		switch snapshot.State {
		case onebot11.TransportStateConnected:
			return "OneBot 已回连"
		case onebot11.TransportStateAuthFailed:
			return "最近一次回连鉴权失败"
		case onebot11.TransportStateStopped:
			return "回连入口已停止"
		default:
			return "等待 OneBot 回连"
		}
	case onebot11.TransportForwardWS:
		switch snapshot.State {
		case onebot11.TransportStateConnected:
			return "主动连接已建立"
		case onebot11.TransportStateConnecting:
			return "正在主动连接"
		case onebot11.TransportStateReconnecting:
			return "连接已断开，正在重试"
		case onebot11.TransportStateAuthFailed:
			return "主动连接鉴权失败"
		case onebot11.TransportStateStopped:
			return "主动连接已停止"
		default:
			return "等待主动连接"
		}
	case onebot11.TransportHTTPAPI:
		switch snapshot.State {
		case onebot11.TransportStateConnected:
			return "HTTP API 可用"
		case onebot11.TransportStateAuthFailed:
			return "HTTP API 鉴权失败"
		case onebot11.TransportStateReconnecting:
			return "HTTP API 请求失败，等待重试"
		case onebot11.TransportStateStopped:
			return "HTTP API 已停止"
		default:
			return "HTTP API 未验证"
		}
	case onebot11.TransportWebhook:
		switch snapshot.State {
		case onebot11.TransportStateConnected:
			return "Webhook 正在接收上报"
		case onebot11.TransportStateAuthFailed:
			return "Webhook 鉴权失败"
		case onebot11.TransportStateStopped:
			return "Webhook 入口已停止"
		default:
			return "Webhook 入口可接收上报"
		}
	default:
		return "未启用"
	}
}
