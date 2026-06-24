package protocolapi

import (
	"context"
	"net/http"

	"github.com/coder/websocket"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
)

type protocolAcceptedResponse struct {
	Accepted bool `json:"accepted"`
}

type protocolHTTPService interface {
	currentOneBot11ProtocolSnapshot() oneBot11ProtocolSnapshotView
	currentOneBot11ProtocolTargets(context.Context) oneBot11ProtocolTargetsResponse
	resolveOneBot11Identities(context.Context, []oneBot11IdentityResolveItem) oneBot11IdentityResolveResponse
	currentOneBot11ProtocolCompatibility() (oneBot11ProtocolCompatibilityResponse, error)
	reverseWSIngressAvailable() bool
	reverseWSIngressEnabled() bool
	reverseWSAccessToken() string
	reverseWSAccessTokenQueryCompat() bool
	markReverseWSAuthFailed()
	attachReverseWS(*websocket.Conn)
	webhookIngressAvailable() bool
	webhookIngressEnabled() bool
	webhookAccessToken() string
	webhookAccessTokenQueryCompat() bool
	markWebhookAuthFailed()
	acceptWebhookPayload(context.Context, []byte) error
}

func (h *ProtocolHandlers) HandleProtocolOneBot11Snapshot() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeAuthJSON(w, http.StatusOK, h.protocol.currentOneBot11ProtocolSnapshot())
	}
}

func (h *ProtocolHandlers) HandleProtocolOneBot11Targets() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeAuthJSON(w, http.StatusOK, h.protocol.currentOneBot11ProtocolTargets(r.Context()))
	}
}

func (h *ProtocolHandlers) HandleProtocolOneBot11IdentitiesResolve() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body oneBot11IdentityResolveRequest
		if err := httpapi.DecodeStrictJSON(w, r, &body, httpapi.MaxManagementJSONBodyBytes); err != nil || len(body.Items) == 0 || len(body.Items) > 100 {
			writeAppError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}
		writeAuthJSON(w, http.StatusOK, h.protocol.resolveOneBot11Identities(r.Context(), body.Items))
	}
}

func (h *ProtocolHandlers) HandleProtocolOneBot11Compatibility() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response, err := h.protocol.currentOneBot11ProtocolCompatibility()
		if err != nil {
			writeAppError(w, r, http.StatusInternalServerError, "adapter.matrix_projection_failed", "协议兼容矩阵生成失败", "errors.adapter.matrix_projection_failed", nil)
			return
		}
		writeAuthJSON(w, http.StatusOK, response)
	}
}

func (h *ProtocolHandlers) HandleProtocolOneBot11ReverseWS() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.protocol.reverseWSIngressAvailable() {
			writeAppError(w, r, http.StatusServiceUnavailable, "adapter.transport_reverse_ws_upgrade_failed", "OneBot 回连入口不可用", "errors.adapter.transport_reverse_ws_upgrade_failed", nil)
			return
		}
		if !h.protocol.reverseWSIngressEnabled() {
			writeAppError(w, r, http.StatusServiceUnavailable, "adapter.transport_reverse_ws_upgrade_failed", "OneBot 回连入口未启用", "errors.adapter.transport_reverse_ws_upgrade_failed", nil)
			return
		}
		if !allowOneBotIngress(r, h.protocol.reverseWSAccessToken(), h.protocol.reverseWSAccessTokenQueryCompat()) {
			h.protocol.markReverseWSAuthFailed()
			writeAppError(w, r, http.StatusUnauthorized, "adapter.transport_reverse_ws_auth_failed", "协议鉴权失败", "errors.adapter.transport_reverse_ws_auth_failed", nil)
			return
		}

		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		h.protocol.attachReverseWS(conn)
	}
}

func (h *ProtocolHandlers) HandleProtocolOneBot11Webhook() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.protocol.webhookIngressAvailable() {
			writeAppError(w, r, http.StatusServiceUnavailable, "adapter.transport_webhook_invalid_payload", "OneBot Webhook 不可用", "errors.adapter.transport_webhook_invalid_payload", nil)
			return
		}
		if !h.protocol.webhookIngressEnabled() {
			writeAppError(w, r, http.StatusServiceUnavailable, "adapter.transport_webhook_invalid_payload", "OneBot Webhook 入口未启用", "errors.adapter.transport_webhook_invalid_payload", nil)
			return
		}
		if !allowOneBotIngress(r, h.protocol.webhookAccessToken(), h.protocol.webhookAccessTokenQueryCompat()) {
			h.protocol.markWebhookAuthFailed()
			writeAppError(w, r, http.StatusUnauthorized, "adapter.transport_webhook_auth_failed", "协议鉴权失败", "errors.adapter.transport_webhook_auth_failed", nil)
			return
		}

		payload, err := httpapi.ReadRequestBody(w, r, httpapi.MaxWebhookBodyBytes)
		if err != nil {
			writeAppError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}
		if err := h.protocol.acceptWebhookPayload(r.Context(), payload); err != nil {
			writeAppError(w, r, http.StatusBadRequest, "adapter.transport_webhook_invalid_payload", "OneBot Webhook 负载不合法", "errors.adapter.transport_webhook_invalid_payload", nil)
			return
		}
		writeAuthJSON(w, http.StatusAccepted, protocolAcceptedResponse{Accepted: true})
	}
}
