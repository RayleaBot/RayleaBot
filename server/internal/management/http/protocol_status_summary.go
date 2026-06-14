package managementhttp

import "github.com/RayleaBot/RayleaBot/server/internal/adapter"

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
