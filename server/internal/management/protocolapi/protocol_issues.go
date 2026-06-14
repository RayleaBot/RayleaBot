package protocolapi

import (
	"strings"

	adaptershell "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/shell"
)

func protocolIssuesFromSnapshot(snapshot adaptershell.Snapshot) []protocolIssueResponse {
	issues := make([]protocolIssueResponse, 0, 4)
	appendIssue := func(transport adaptershell.TransportKey, transportSnapshot adaptershell.TransportSnapshot) {
		code := strings.TrimSpace(transportSnapshot.LastErrorCode)
		if code == "" {
			return
		}
		issues = append(issues, protocolIssueResponse{
			Code:     code,
			Severity: "warning",
			Summary:  transportIssueSummary(transport, transportSnapshot),
		})
	}

	appendIssue(adaptershell.TransportForwardWS, snapshot.ForwardWS)
	appendIssue(adaptershell.TransportReverseWS, snapshot.ReverseWS)
	appendIssue(adaptershell.TransportHTTPAPI, snapshot.HTTPAPI)
	appendIssue(adaptershell.TransportWebhook, snapshot.Webhook)
	return issues
}

func transportIssueSummary(transport adaptershell.TransportKey, snapshot adaptershell.TransportSnapshot) string {
	switch transport {
	case adaptershell.TransportForwardWS:
		switch snapshot.State {
		case adaptershell.TransportStateAuthFailed:
			return "OneBot 主动连接鉴权失败，请检查访问令牌。"
		case adaptershell.TransportStateReconnecting:
			return "OneBot 主动连接已断开，正在重试。"
		}
		switch strings.TrimSpace(snapshot.LastErrorCode) {
		case "adapter.transport_forward_ws_session_lost", "adapter.connection_lost":
			return "OneBot 主动连接已断开，正在重试。"
		case "adapter.transport_forward_ws_connection_failed":
			return "OneBot 主动连接失败，请检查地址与网络。"
		}
		return "OneBot 主动连接出现异常。"
	case adaptershell.TransportReverseWS:
		switch snapshot.State {
		case adaptershell.TransportStateAuthFailed:
			return "OneBot 回连鉴权失败，请检查访问令牌。"
		case adaptershell.TransportStateConnected:
			return "OneBot 回连链路已恢复。"
		}
		switch strings.TrimSpace(snapshot.LastErrorCode) {
		case "adapter.transport_reverse_ws_auth_failed":
			return "OneBot 回连鉴权失败，请检查访问令牌。"
		case "adapter.connection_lost":
			return "OneBot 回连会话已断开，请让 OneBot 重新回连。"
		}
		return "OneBot 回连链路出现异常。"
	case adaptershell.TransportHTTPAPI:
		switch snapshot.State {
		case adaptershell.TransportStateAuthFailed:
			return "OneBot HTTP API 鉴权失败，请检查访问令牌。"
		case adaptershell.TransportStateConnected:
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
	case adaptershell.TransportWebhook:
		switch snapshot.State {
		case adaptershell.TransportStateAuthFailed:
			return "OneBot Webhook 鉴权失败，请检查访问令牌。"
		case adaptershell.TransportStateListening, adaptershell.TransportStateConnected:
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
