package protocolapi

import (
	adaptershell "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/shell"
)

func protocolReadinessStatus(snapshot adaptershell.Snapshot) string {
	outboundReady := snapshot.ForwardWS.State == adaptershell.TransportStateConnected ||
		snapshot.ReverseWS.State == adaptershell.TransportStateConnected ||
		snapshot.HTTPAPI.State == adaptershell.TransportStateConnected
	inboundReady := snapshot.ReverseWS.State == adaptershell.TransportStateConnected ||
		snapshot.Webhook.State == adaptershell.TransportStateListening ||
		snapshot.Webhook.State == adaptershell.TransportStateConnected

	configuredAny := snapshot.ForwardWS.Configured || snapshot.ReverseWS.Configured || snapshot.HTTPAPI.Configured || snapshot.Webhook.Configured
	if !configuredAny {
		return "setup_required"
	}
	if snapshot.ForwardWS.State == adaptershell.TransportStateConnected || snapshot.ReverseWS.State == adaptershell.TransportStateConnected {
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

func protocolSnapshotSummary(snapshot adaptershell.Snapshot, readiness string) string {
	switch readiness {
	case "ready":
		if snapshot.ForwardWS.State == adaptershell.TransportStateConnected {
			return "OneBot11 主动连接已就绪"
		}
		if snapshot.ReverseWS.State == adaptershell.TransportStateConnected {
			return "OneBot11 回连链路已就绪"
		}
		return "OneBot11 HTTP API 与 Webhook 已就绪"
	case "degraded":
		if snapshot.HTTPAPI.State == adaptershell.TransportStateConnected && (snapshot.Webhook.State == adaptershell.TransportStateListening || snapshot.Webhook.State == adaptershell.TransportStateConnected) {
			return "OneBot11 HTTP API 与 Webhook 可用，但尚未建立 WebSocket 会话"
		}
		if snapshot.HTTPAPI.State == adaptershell.TransportStateConnected {
			return "OneBot11 仅 HTTP API 可用"
		}
		if snapshot.Webhook.State == adaptershell.TransportStateListening || snapshot.Webhook.State == adaptershell.TransportStateConnected {
			return "OneBot11 仅 Webhook 上报可用"
		}
		if snapshot.ReverseWS.State == adaptershell.TransportStateListening {
			return "OneBot11 等待回连"
		}
		if snapshot.ForwardWS.State == adaptershell.TransportStateConnecting || snapshot.ForwardWS.State == adaptershell.TransportStateReconnecting {
			return "OneBot11 正在建立主动连接"
		}
		return "OneBot11 传输链路部分可用"
	case "failed":
		return "OneBot11 传输链路不可用"
	default:
		return "OneBot11 尚未配置连接"
	}
}

func protocolTransportSummary(key adaptershell.TransportKey, snapshot adaptershell.TransportSnapshot) string {
	if !snapshot.Enabled || !snapshot.Configured {
		return "未启用"
	}

	switch key {
	case adaptershell.TransportReverseWS:
		switch snapshot.State {
		case adaptershell.TransportStateConnected:
			return "OneBot 已回连"
		case adaptershell.TransportStateAuthFailed:
			return "最近一次回连鉴权失败"
		case adaptershell.TransportStateStopped:
			return "回连入口已停止"
		default:
			return "等待 OneBot 回连"
		}
	case adaptershell.TransportForwardWS:
		switch snapshot.State {
		case adaptershell.TransportStateConnected:
			return "主动连接已建立"
		case adaptershell.TransportStateConnecting:
			return "正在主动连接"
		case adaptershell.TransportStateReconnecting:
			return "连接已断开，正在重试"
		case adaptershell.TransportStateAuthFailed:
			return "主动连接鉴权失败"
		case adaptershell.TransportStateStopped:
			return "主动连接已停止"
		default:
			return "等待主动连接"
		}
	case adaptershell.TransportHTTPAPI:
		switch snapshot.State {
		case adaptershell.TransportStateConnected:
			return "HTTP API 可用"
		case adaptershell.TransportStateAuthFailed:
			return "HTTP API 鉴权失败"
		case adaptershell.TransportStateReconnecting:
			return "HTTP API 请求失败，等待重试"
		case adaptershell.TransportStateStopped:
			return "HTTP API 已停止"
		default:
			return "HTTP API 未验证"
		}
	case adaptershell.TransportWebhook:
		switch snapshot.State {
		case adaptershell.TransportStateConnected:
			return "Webhook 正在接收上报"
		case adaptershell.TransportStateAuthFailed:
			return "Webhook 鉴权失败"
		case adaptershell.TransportStateStopped:
			return "Webhook 入口已停止"
		default:
			return "Webhook 入口可接收上报"
		}
	default:
		return "未启用"
	}
}
