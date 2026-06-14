package managementhttp

import (
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
)

func protocolIssuesFromSnapshot(snapshot adapter.Snapshot) []protocolIssueResponse {
	issues := make([]protocolIssueResponse, 0, 4)
	appendIssue := func(transport adapter.TransportKey, transportSnapshot adapter.TransportSnapshot) {
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

	appendIssue(adapter.TransportForwardWS, snapshot.ForwardWS)
	appendIssue(adapter.TransportReverseWS, snapshot.ReverseWS)
	appendIssue(adapter.TransportHTTPAPI, snapshot.HTTPAPI)
	appendIssue(adapter.TransportWebhook, snapshot.Webhook)
	return issues
}

func transportIssueSummary(transport adapter.TransportKey, snapshot adapter.TransportSnapshot) string {
	switch transport {
	case adapter.TransportForwardWS:
		switch snapshot.State {
		case adapter.TransportStateAuthFailed:
			return "OneBot 主动连接鉴权失败，请检查访问令牌。"
		case adapter.TransportStateReconnecting:
			return "OneBot 主动连接已断开，正在重试。"
		}
		switch strings.TrimSpace(snapshot.LastErrorCode) {
		case "adapter.transport_forward_ws_session_lost", "adapter.connection_lost":
			return "OneBot 主动连接已断开，正在重试。"
		case "adapter.transport_forward_ws_connection_failed":
			return "OneBot 主动连接失败，请检查地址与网络。"
		}
		return "OneBot 主动连接出现异常。"
	case adapter.TransportReverseWS:
		switch snapshot.State {
		case adapter.TransportStateAuthFailed:
			return "OneBot 回连鉴权失败，请检查访问令牌。"
		case adapter.TransportStateConnected:
			return "OneBot 回连链路已恢复。"
		}
		switch strings.TrimSpace(snapshot.LastErrorCode) {
		case "adapter.transport_reverse_ws_auth_failed":
			return "OneBot 回连鉴权失败，请检查访问令牌。"
		case "adapter.connection_lost":
			return "OneBot 回连会话已断开，请让 OneBot 重新回连。"
		}
		return "OneBot 回连链路出现异常。"
	case adapter.TransportHTTPAPI:
		switch snapshot.State {
		case adapter.TransportStateAuthFailed:
			return "OneBot HTTP API 鉴权失败，请检查访问令牌。"
		case adapter.TransportStateConnected:
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
	case adapter.TransportWebhook:
		switch snapshot.State {
		case adapter.TransportStateAuthFailed:
			return "OneBot Webhook 鉴权失败，请检查访问令牌。"
		case adapter.TransportStateListening, adapter.TransportStateConnected:
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
