package management

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/configruntime"
	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/onebot11"
	"github.com/RayleaBot/RayleaBot/server/internal/pubsub"
)

type protocolAcceptedResponse struct {
	Accepted bool `json:"accepted"`
}

const protocolCodeInvalidRequest = "platform.invalid_request"

type protocolIssueResponse struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Summary  string `json:"summary"`
}

type protocolTransportStatusResponse struct {
	Transport       string `json:"transport"`
	Enabled         bool   `json:"enabled"`
	Configured      bool   `json:"configured"`
	Endpoint        string `json:"endpoint"`
	State           string `json:"state"`
	Summary         string `json:"summary"`
	Provider        string `json:"provider,omitempty"`
	AppName         string `json:"app_name,omitempty"`
	ProtocolVersion string `json:"protocol_version,omitempty"`
	AppVersion      string `json:"app_version,omitempty"`
	UserID          string `json:"user_id,omitempty"`
	Nickname        string `json:"nickname,omitempty"`
}

type oneBot11ProtocolSnapshotView struct {
	Protocol              string                            `json:"protocol"`
	Provider              string                            `json:"provider"`
	ConfiguredTransports  []string                          `json:"configured_transports"`
	ActiveTransports      []string                          `json:"active_transports"`
	TransportStatus       []protocolTransportStatusResponse `json:"transport_status"`
	ReadinessStatus       string                            `json:"readiness_status"`
	Summary               string                            `json:"summary"`
	RecentTransportIssues []protocolIssueResponse           `json:"recent_transport_issues"`
}

type oneBot11TargetIssueResponse struct {
	Scope   string `json:"scope"`
	Message string `json:"message"`
}

type oneBot11GroupTargetResponse struct {
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	TargetName string `json:"target_name"`
	AvatarURL  string `json:"avatar_url,omitempty"`
}

type oneBot11PrivateTargetResponse struct {
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	Nickname   string `json:"nickname"`
	AvatarURL  string `json:"avatar_url,omitempty"`
}

type oneBot11ProtocolTargetsResponse struct {
	Protocol     string                          `json:"protocol"`
	Available    bool                            `json:"available"`
	Groups       []oneBot11GroupTargetResponse   `json:"groups"`
	PrivateUsers []oneBot11PrivateTargetResponse `json:"private_users"`
	Issues       []oneBot11TargetIssueResponse   `json:"issues"`
}

type oneBot11IdentityResolveRequest struct {
	Items []oneBot11IdentityResolveItem `json:"items"`
}

type oneBot11IdentityResolveItem struct {
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	UserID     string `json:"user_id"`
}

type oneBot11IdentityResponse struct {
	TargetType    string `json:"target_type"`
	TargetID      string `json:"target_id"`
	UserID        string `json:"user_id"`
	Nickname      string `json:"nickname"`
	GroupNickname string `json:"group_nickname,omitempty"`
	Title         string `json:"title,omitempty"`
	Role          string `json:"role,omitempty"`
	RoleLabel     string `json:"role_label,omitempty"`
	AvatarURL     string `json:"avatar_url"`
}

type oneBot11IdentityResolveResponse struct {
	Items  []oneBot11IdentityResponse    `json:"items"`
	Issues []oneBot11TargetIssueResponse `json:"issues"`
}

type protocolCompatibilitySupportResponse struct {
	Standard    string `json:"standard"`
	NapCat      string `json:"napcat"`
	LuckyLillia string `json:"luckylillia"`
}

type protocolCompatibilityItemResponse struct {
	Key     string                               `json:"key"`
	Label   string                               `json:"label"`
	Support protocolCompatibilitySupportResponse `json:"support"`
	Summary string                               `json:"summary"`
}

type protocolCompatibilityCategoryResponse struct {
	Key   string                              `json:"key"`
	Title string                              `json:"title"`
	Items []protocolCompatibilityItemResponse `json:"items"`
}

type oneBot11ProtocolCompatibilityResponse struct {
	Protocol   string                                  `json:"protocol"`
	Categories []protocolCompatibilityCategoryResponse `json:"categories"`
}

type ProtocolConfigSource interface {
	CurrentConfig() config.Config
}

type ProtocolService struct {
	config                    ProtocolConfigSource
	adapter                   *onebot11.Shell
	oneBot11TargetReadTimeout time.Duration
	hub                       pubsub.Hub[Frame]
}

func NewProtocolService(configSource ProtocolConfigSource, adapterShell *onebot11.Shell) *ProtocolService {
	return &ProtocolService{
		config:                    configSource,
		adapter:                   adapterShell,
		oneBot11TargetReadTimeout: 3 * time.Second,
	}
}

type ProtocolHandlers struct {
	protocol protocolHTTPService
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

func NewProtocolHandlers(protocol protocolHTTPService) *ProtocolHandlers {
	return &ProtocolHandlers{protocol: protocol}
}

func (s *ProtocolService) ApplyConfigReload(cfg config.Config) error {
	if s == nil || s.adapter == nil {
		return nil
	}
	if s.adapter.Snapshot().State == onebot11.StateStopped {
		return configruntime.ErrProtocolStopped
	}
	return s.adapter.Reload(cfg.OneBot, cfg.Adapter)
}

func (s *ProtocolService) ProtocolSnapshotEvent() Frame {
	return NewReceivedFrame(ProtocolSnapshotPayload{
		Protocol:         "onebot11",
		ProtocolSnapshot: s.currentOneBot11ProtocolSnapshot(),
	})
}

func (s *ProtocolService) PublishSnapshot() {
	if s == nil {
		return
	}
	s.hub.Publish(s.ProtocolSnapshotEvent())
}

func (s *ProtocolService) SubscribeProtocolEvents(buffer int) (<-chan Frame, func()) {
	return s.hub.Subscribe(buffer)
}

func (s *ProtocolService) reverseWSIngressAvailable() bool {
	return s != nil && s.adapter != nil
}

func (s *ProtocolService) reverseWSIngressEnabled() bool {
	return s.transportIngressEnabled(onebot11.TransportReverseWS)
}

func (s *ProtocolService) reverseWSAccessToken() string {
	if s == nil || s.config == nil {
		return ""
	}
	return s.config.CurrentConfig().OneBot.ReverseWS.AccessToken
}

func (s *ProtocolService) reverseWSAccessTokenQueryCompat() bool {
	if s == nil || s.config == nil {
		return false
	}
	return s.config.CurrentConfig().OneBot.ReverseWS.AccessTokenQueryCompat
}

func (s *ProtocolService) markReverseWSAuthFailed() {
	if s == nil || s.adapter == nil {
		return
	}
	s.adapter.MarkReverseWSAuthFailed()
}

func (s *ProtocolService) attachReverseWS(conn *websocket.Conn) {
	if s == nil || s.adapter == nil {
		return
	}
	s.adapter.AttachReverseWS(conn)
}

func (s *ProtocolService) webhookIngressAvailable() bool {
	return s != nil && s.adapter != nil
}

func (s *ProtocolService) webhookIngressEnabled() bool {
	return s.transportIngressEnabled(onebot11.TransportWebhook)
}

func (s *ProtocolService) webhookAccessToken() string {
	if s == nil || s.config == nil {
		return ""
	}
	return s.config.CurrentConfig().OneBot.Webhook.AccessToken
}

func (s *ProtocolService) webhookAccessTokenQueryCompat() bool {
	if s == nil || s.config == nil {
		return false
	}
	return s.config.CurrentConfig().OneBot.Webhook.AccessTokenQueryCompat
}

func (s *ProtocolService) markWebhookAuthFailed() {
	if s == nil || s.adapter == nil {
		return
	}
	s.adapter.MarkWebhookAuthFailed()
}

func (s *ProtocolService) acceptWebhookPayload(ctx context.Context, payload []byte) error {
	if s == nil || s.adapter == nil {
		return configruntime.ErrProtocolStopped
	}
	return s.adapter.AcceptWebhookPayload(ctx, payload)
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
		httpapi.WriteJSON(w, http.StatusOK, h.protocol.currentOneBot11ProtocolSnapshot())
	}
}

func (h *ProtocolHandlers) HandleProtocolOneBot11Targets() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		httpapi.WriteJSON(w, http.StatusOK, h.protocol.currentOneBot11ProtocolTargets(r.Context()))
	}
}

func (h *ProtocolHandlers) HandleProtocolOneBot11IdentitiesResolve() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body oneBot11IdentityResolveRequest
		if err := httpapi.DecodeStrictJSON(w, r, &body, httpapi.MaxManagementJSONBodyBytes); err != nil || len(body.Items) == 0 || len(body.Items) > 100 {
			httpapi.WriteError(w, r, http.StatusBadRequest, protocolCodeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, h.protocol.resolveOneBot11Identities(r.Context(), body.Items))
	}
}

func (h *ProtocolHandlers) HandleProtocolOneBot11Compatibility() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response, err := h.protocol.currentOneBot11ProtocolCompatibility()
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "adapter.matrix_projection_failed", "协议兼容矩阵生成失败", "errors.adapter.matrix_projection_failed", nil)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, response)
	}
}

func (h *ProtocolHandlers) HandleProtocolOneBot11ReverseWS() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.protocol.reverseWSIngressAvailable() {
			httpapi.WriteError(w, r, http.StatusServiceUnavailable, "adapter.transport_reverse_ws_upgrade_failed", "OneBot 回连入口不可用", "errors.adapter.transport_reverse_ws_upgrade_failed", nil)
			return
		}
		if !h.protocol.reverseWSIngressEnabled() {
			httpapi.WriteError(w, r, http.StatusServiceUnavailable, "adapter.transport_reverse_ws_upgrade_failed", "OneBot 回连入口未启用", "errors.adapter.transport_reverse_ws_upgrade_failed", nil)
			return
		}
		if !allowOneBotIngress(r, h.protocol.reverseWSAccessToken(), h.protocol.reverseWSAccessTokenQueryCompat()) {
			h.protocol.markReverseWSAuthFailed()
			httpapi.WriteError(w, r, http.StatusUnauthorized, "adapter.transport_reverse_ws_auth_failed", "协议鉴权失败", "errors.adapter.transport_reverse_ws_auth_failed", nil)
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
			httpapi.WriteError(w, r, http.StatusServiceUnavailable, "adapter.transport_webhook_invalid_payload", "OneBot Webhook 不可用", "errors.adapter.transport_webhook_invalid_payload", nil)
			return
		}
		if !h.protocol.webhookIngressEnabled() {
			httpapi.WriteError(w, r, http.StatusServiceUnavailable, "adapter.transport_webhook_invalid_payload", "OneBot Webhook 入口未启用", "errors.adapter.transport_webhook_invalid_payload", nil)
			return
		}
		if !allowOneBotIngress(r, h.protocol.webhookAccessToken(), h.protocol.webhookAccessTokenQueryCompat()) {
			h.protocol.markWebhookAuthFailed()
			httpapi.WriteError(w, r, http.StatusUnauthorized, "adapter.transport_webhook_auth_failed", "协议鉴权失败", "errors.adapter.transport_webhook_auth_failed", nil)
			return
		}

		payload, err := httpapi.ReadRequestBody(w, r, httpapi.MaxWebhookBodyBytes)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, protocolCodeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}
		if err := h.protocol.acceptWebhookPayload(r.Context(), payload); err != nil {
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

func currentOneBotProvider(raw string) string {
	switch strings.TrimSpace(raw) {
	case "standard", "napcat", "luckylillia":
		return strings.TrimSpace(raw)
	default:
		return "unknown"
	}
}

func oneBot11AvatarURL(userID string) string {
	return "https://q1.qlogo.cn/g?b=qq&nk=" + strings.TrimSpace(userID) + "&s=640"
}

func oneBot11GroupAvatarURL(groupID string) string {
	id := strings.TrimSpace(groupID)
	if id == "" {
		return ""
	}
	return "https://p.qlogo.cn/gh/" + id + "/" + id + "/100"
}

func oneBot11RoleLabel(role string) string {
	switch strings.TrimSpace(role) {
	case "owner":
		return "群主"
	case "admin":
		return "管理员"
	case "member":
		return "成员"
	default:
		return ""
	}
}

func isDigits(raw string) bool {
	if raw == "" {
		return false
	}
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}
