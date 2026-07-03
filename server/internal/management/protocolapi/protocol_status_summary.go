package protocolapi

import (
	"github.com/RayleaBot/RayleaBot/server/internal/onebot11"
)

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
