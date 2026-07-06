package management

import (
	"context"
	"net/http"
	"strings"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/wsevents"
)

type protocolAcceptedResponse struct {
	Accepted bool `json:"accepted"`
}

const protocolCodeInvalidRequest = "platform.invalid_request"

type oneBot11IdentityResolveRequest struct {
	Items []wsevents.OneBot11IdentityResolveItem `json:"items"`
}

type ProtocolHandlers struct {
	protocol protocolHTTPService
}

type protocolHTTPService interface {
	CurrentOneBot11ProtocolSnapshot() wsevents.OneBot11ProtocolSnapshot
	CurrentOneBot11ProtocolTargets(context.Context) wsevents.OneBot11ProtocolTargets
	ResolveOneBot11Identities(context.Context, []wsevents.OneBot11IdentityResolveItem) wsevents.OneBot11IdentityResolveResult
	CurrentOneBot11ProtocolCompatibility() (wsevents.OneBot11ProtocolCompatibility, error)
	ReverseWSIngressAvailable() bool
	ReverseWSIngressEnabled() bool
	ReverseWSAccessToken() string
	ReverseWSAccessTokenQueryCompat() bool
	MarkReverseWSAuthFailed()
	AttachReverseWS(*websocket.Conn)
	WebhookIngressAvailable() bool
	WebhookIngressEnabled() bool
	WebhookAccessToken() string
	WebhookAccessTokenQueryCompat() bool
	MarkWebhookAuthFailed()
	AcceptWebhookPayload(context.Context, []byte) error
}

func NewProtocolHandlers(protocol protocolHTTPService) *ProtocolHandlers {
	return &ProtocolHandlers{protocol: protocol}
}

func (h *ProtocolHandlers) RegisterPublicRoutes(router chi.Router) {
	router.Get("/api/protocols/onebot11/reverse-ws", h.HandleProtocolOneBot11ReverseWS())
	router.Post("/api/protocols/onebot11/webhook", h.HandleProtocolOneBot11Webhook())
}

func (h *ProtocolHandlers) RegisterProtectedRoutes(router chi.Router) {
	router.Get("/api/protocols/onebot11", h.HandleProtocolOneBot11Snapshot())
	router.Get("/api/protocols/onebot11/targets", h.HandleProtocolOneBot11Targets())
	router.Post("/api/protocols/onebot11/identities/resolve", h.HandleProtocolOneBot11IdentitiesResolve())
	router.Get("/api/protocols/onebot11/compatibility", h.HandleProtocolOneBot11Compatibility())
}

func (h *ProtocolHandlers) HandleProtocolOneBot11Snapshot() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		httpapi.WriteJSON(w, http.StatusOK, h.protocol.CurrentOneBot11ProtocolSnapshot())
	}
}

func (h *ProtocolHandlers) HandleProtocolOneBot11Targets() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		httpapi.WriteJSON(w, http.StatusOK, h.protocol.CurrentOneBot11ProtocolTargets(r.Context()))
	}
}

func (h *ProtocolHandlers) HandleProtocolOneBot11IdentitiesResolve() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body oneBot11IdentityResolveRequest
		if err := httpapi.DecodeStrictJSON(w, r, &body, httpapi.MaxManagementJSONBodyBytes); err != nil || len(body.Items) == 0 || len(body.Items) > 100 {
			httpapi.WriteError(w, r, http.StatusBadRequest, protocolCodeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, h.protocol.ResolveOneBot11Identities(r.Context(), body.Items))
	}
}

func (h *ProtocolHandlers) HandleProtocolOneBot11Compatibility() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response, err := h.protocol.CurrentOneBot11ProtocolCompatibility()
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "adapter.matrix_projection_failed", "协议兼容矩阵生成失败", "errors.adapter.matrix_projection_failed", nil)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, response)
	}
}

func (h *ProtocolHandlers) HandleProtocolOneBot11ReverseWS() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.protocol.ReverseWSIngressAvailable() {
			httpapi.WriteError(w, r, http.StatusServiceUnavailable, "adapter.transport_reverse_ws_upgrade_failed", "OneBot 回连入口不可用", "errors.adapter.transport_reverse_ws_upgrade_failed", nil)
			return
		}
		if !h.protocol.ReverseWSIngressEnabled() {
			httpapi.WriteError(w, r, http.StatusServiceUnavailable, "adapter.transport_reverse_ws_upgrade_failed", "OneBot 回连入口未启用", "errors.adapter.transport_reverse_ws_upgrade_failed", nil)
			return
		}
		if !allowOneBotIngress(r, h.protocol.ReverseWSAccessToken(), h.protocol.ReverseWSAccessTokenQueryCompat()) {
			h.protocol.MarkReverseWSAuthFailed()
			httpapi.WriteError(w, r, http.StatusUnauthorized, "adapter.transport_reverse_ws_auth_failed", "协议鉴权失败", "errors.adapter.transport_reverse_ws_auth_failed", nil)
			return
		}

		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		h.protocol.AttachReverseWS(conn)
	}
}

func (h *ProtocolHandlers) HandleProtocolOneBot11Webhook() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.protocol.WebhookIngressAvailable() {
			httpapi.WriteError(w, r, http.StatusServiceUnavailable, "adapter.transport_webhook_invalid_payload", "OneBot Webhook 不可用", "errors.adapter.transport_webhook_invalid_payload", nil)
			return
		}
		if !h.protocol.WebhookIngressEnabled() {
			httpapi.WriteError(w, r, http.StatusServiceUnavailable, "adapter.transport_webhook_invalid_payload", "OneBot Webhook 入口未启用", "errors.adapter.transport_webhook_invalid_payload", nil)
			return
		}
		if !allowOneBotIngress(r, h.protocol.WebhookAccessToken(), h.protocol.WebhookAccessTokenQueryCompat()) {
			h.protocol.MarkWebhookAuthFailed()
			httpapi.WriteError(w, r, http.StatusUnauthorized, "adapter.transport_webhook_auth_failed", "协议鉴权失败", "errors.adapter.transport_webhook_auth_failed", nil)
			return
		}

		payload, err := httpapi.ReadRequestBody(w, r, httpapi.MaxWebhookBodyBytes)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, protocolCodeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}
		if err := h.protocol.AcceptWebhookPayload(r.Context(), payload); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "adapter.transport_webhook_invalid_payload", "OneBot Webhook 负载不合法", "errors.adapter.transport_webhook_invalid_payload", nil)
			return
		}
		httpapi.WriteJSON(w, http.StatusAccepted, protocolAcceptedResponse{Accepted: true})
	}
}

func allowOneBotIngress(r *http.Request, accessToken string, allowQueryToken bool) bool {
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
	if allowQueryToken && strings.TrimSpace(r.URL.Query().Get("access_token")) == trimmedToken {
		return true
	}
	return false
}
