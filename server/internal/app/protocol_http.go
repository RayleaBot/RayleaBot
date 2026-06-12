package app

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/coder/websocket"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/protocolcap"
)

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

type oneBot11ProtocolSnapshotResponse = oneBot11ProtocolSnapshotView

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
}

type oneBot11PrivateTargetResponse struct {
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	Nickname   string `json:"nickname"`
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

type protocolService struct {
	state       *appRuntimeState
	adapter     *adapter.Shell
	mu          sync.RWMutex
	nextSubID   uint64
	subscribers map[uint64]chan managementEventFrame
}

func newProtocolService(state *appRuntimeState, adapterShell *adapter.Shell) *protocolService {
	return &protocolService{
		state:       state,
		adapter:     adapterShell,
		subscribers: make(map[uint64]chan managementEventFrame),
	}
}

func (h *protocolHTTPHandlers) handleProtocolOneBot11Snapshot() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeAuthJSON(w, http.StatusOK, h.protocol.currentOneBot11ProtocolSnapshot())
	}
}

func (h *protocolHTTPHandlers) handleProtocolOneBot11Targets() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeAuthJSON(w, http.StatusOK, h.protocol.currentOneBot11ProtocolTargets(r.Context()))
	}
}

func (h *protocolHTTPHandlers) handleProtocolOneBot11IdentitiesResolve() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body oneBot11IdentityResolveRequest
		if err := httpapi.DecodeStrictJSON(w, r, &body, httpapi.MaxManagementJSONBodyBytes); err != nil || len(body.Items) == 0 || len(body.Items) > 100 {
			writeAppError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}
		writeAuthJSON(w, http.StatusOK, h.protocol.resolveOneBot11Identities(r.Context(), body.Items))
	}
}

func (h *protocolHTTPHandlers) handleProtocolOneBot11Compatibility() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response, err := h.protocol.currentOneBot11ProtocolCompatibility()
		if err != nil {
			writeAppError(w, r, http.StatusInternalServerError, "adapter.matrix_projection_failed", "协议兼容矩阵生成失败", "errors.adapter.matrix_projection_failed", nil)
			return
		}
		writeAuthJSON(w, http.StatusOK, response)
	}
}

func (h *protocolHTTPHandlers) handleProtocolOneBot11ReverseWS() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.protocol.adapter == nil {
			writeAppError(w, r, http.StatusServiceUnavailable, "adapter.transport_reverse_ws_upgrade_failed", "OneBot 回连入口不可用", "errors.adapter.transport_reverse_ws_upgrade_failed", nil)
			return
		}
		if !h.protocol.transportIngressEnabled(adapter.TransportReverseWS) {
			writeAppError(w, r, http.StatusServiceUnavailable, "adapter.transport_reverse_ws_upgrade_failed", "OneBot 回连入口未启用", "errors.adapter.transport_reverse_ws_upgrade_failed", nil)
			return
		}
		if !allowOneBotIngress(r, h.protocol.state.Config.OneBot.ReverseWS.AccessToken) {
			h.protocol.adapter.MarkReverseWSAuthFailed()
			writeAppError(w, r, http.StatusUnauthorized, "adapter.transport_reverse_ws_auth_failed", "协议鉴权失败", "errors.adapter.transport_reverse_ws_auth_failed", nil)
			return
		}

		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		h.protocol.adapter.AttachReverseWS(conn)
	}
}

func (h *protocolHTTPHandlers) handleProtocolOneBot11Webhook() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.protocol.adapter == nil {
			writeAppError(w, r, http.StatusServiceUnavailable, "adapter.transport_webhook_invalid_payload", "OneBot Webhook 不可用", "errors.adapter.transport_webhook_invalid_payload", nil)
			return
		}
		if !h.protocol.transportIngressEnabled(adapter.TransportWebhook) {
			writeAppError(w, r, http.StatusServiceUnavailable, "adapter.transport_webhook_invalid_payload", "OneBot Webhook 入口未启用", "errors.adapter.transport_webhook_invalid_payload", nil)
			return
		}
		if !allowOneBotIngress(r, h.protocol.state.Config.OneBot.Webhook.AccessToken) {
			h.protocol.adapter.MarkWebhookAuthFailed()
			writeAppError(w, r, http.StatusUnauthorized, "adapter.transport_webhook_auth_failed", "协议鉴权失败", "errors.adapter.transport_webhook_auth_failed", nil)
			return
		}

		payload, err := httpapi.ReadRequestBody(w, r, httpapi.MaxWebhookBodyBytes)
		if err != nil {
			writeAppError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}
		if err := h.protocol.adapter.AcceptWebhookPayload(r.Context(), payload); err != nil {
			writeAppError(w, r, http.StatusBadRequest, "adapter.transport_webhook_invalid_payload", "OneBot Webhook 负载不合法", "errors.adapter.transport_webhook_invalid_payload", nil)
			return
		}
		writeAuthJSON(w, http.StatusAccepted, systemShutdownResponse{Accepted: true})
	}
}

func (s *protocolService) currentOneBot11ProtocolSnapshot() oneBot11ProtocolSnapshotResponse {
	adapterSnapshot := adapter.Snapshot{}
	if s.adapter != nil {
		adapterSnapshot = s.adapter.Snapshot()
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
		runtimeInfo := transport.snapshot.RuntimeInfo
		status = append(status, protocolTransportStatusResponse{
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
	return oneBot11ProtocolSnapshotResponse{
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

func (s *protocolService) currentOneBot11ProtocolTargets(ctx context.Context) oneBot11ProtocolTargetsResponse {
	response := oneBot11ProtocolTargetsResponse{
		Protocol:     "onebot11",
		Groups:       []oneBot11GroupTargetResponse{},
		PrivateUsers: []oneBot11PrivateTargetResponse{},
		Issues:       []oneBot11TargetIssueResponse{},
	}
	if s == nil || s.adapter == nil {
		response.Issues = append(response.Issues, oneBot11TargetIssueResponse{Scope: "protocol", Message: "OneBot11 协议不可用"})
		return response
	}

	groups, groupErr := s.adapter.ListGroups(ctx)
	if groupErr != nil {
		response.Issues = append(response.Issues, oneBot11TargetIssueResponse{Scope: "groups", Message: "群聊列表读取失败"})
	} else {
		for _, group := range groups {
			response.Groups = append(response.Groups, oneBot11GroupTargetResponse{
				TargetType: "group",
				TargetID:   group.ID,
				TargetName: group.Name,
			})
		}
	}

	friends, friendErr := s.adapter.ListFriends(ctx)
	if friendErr != nil {
		response.Issues = append(response.Issues, oneBot11TargetIssueResponse{Scope: "private_users", Message: "私聊对象列表读取失败"})
	} else {
		for _, friend := range friends {
			response.PrivateUsers = append(response.PrivateUsers, oneBot11PrivateTargetResponse{
				TargetType: "private",
				TargetID:   friend.ID,
				Nickname:   friend.Nickname,
			})
		}
	}

	response.Available = groupErr == nil && friendErr == nil
	return response
}

func (s *protocolService) resolveOneBot11Identities(ctx context.Context, items []oneBot11IdentityResolveItem) oneBot11IdentityResolveResponse {
	response := oneBot11IdentityResolveResponse{
		Items:  []oneBot11IdentityResponse{},
		Issues: []oneBot11TargetIssueResponse{},
	}
	if s == nil || s.adapter == nil {
		response.Issues = append(response.Issues, oneBot11TargetIssueResponse{Scope: "protocol", Message: "OneBot11 协议不可用"})
		return response
	}

	seen := map[string]struct{}{}
	for _, item := range items {
		targetType := strings.TrimSpace(item.TargetType)
		targetID := strings.TrimSpace(item.TargetID)
		userID := strings.TrimSpace(item.UserID)
		if (targetType != "group" && targetType != "private") || !isDigits(targetID) || !isDigits(userID) {
			response.Issues = append(response.Issues, oneBot11TargetIssueResponse{Scope: "identity", Message: "身份解析参数不合法"})
			continue
		}
		key := targetType + ":" + targetID + ":" + userID
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		switch targetType {
		case "group":
			member, err := s.adapter.GetGroupMemberInfo(ctx, targetID, userID)
			if err != nil {
				response.Issues = append(response.Issues, oneBot11TargetIssueResponse{Scope: "identity", Message: "群成员身份读取失败"})
				continue
			}
			nickname := member.Nickname
			if nickname == "" {
				nickname = userID
			}
			response.Items = append(response.Items, oneBot11IdentityResponse{
				TargetType:    "group",
				TargetID:      targetID,
				UserID:        userID,
				Nickname:      nickname,
				GroupNickname: member.Card,
				Title:         member.Title,
				Role:          member.Role,
				RoleLabel:     oneBot11RoleLabel(member.Role),
				AvatarURL:     oneBot11AvatarURL(userID),
			})
		case "private":
			stranger, err := s.adapter.GetStrangerInfo(ctx, userID)
			if err != nil {
				response.Issues = append(response.Issues, oneBot11TargetIssueResponse{Scope: "identity", Message: "私聊身份读取失败"})
				continue
			}
			nickname := stranger.Nickname
			if nickname == "" {
				nickname = userID
			}
			response.Items = append(response.Items, oneBot11IdentityResponse{
				TargetType: "private",
				TargetID:   targetID,
				UserID:     userID,
				Nickname:   nickname,
				AvatarURL:  oneBot11AvatarURL(userID),
			})
		}
	}
	return response
}

func (s *protocolService) currentOneBot11ProtocolCompatibility() (oneBot11ProtocolCompatibilityResponse, error) {
	matrix := protocolcap.OneBot11CompatibilityMatrix()
	categories := make([]protocolCompatibilityCategoryResponse, 0, len(matrix.Categories))
	for _, category := range matrix.Categories {
		items := make([]protocolCompatibilityItemResponse, 0, len(category.Items))
		for _, item := range category.Items {
			items = append(items, protocolCompatibilityItemResponse{
				Key:   item.Key,
				Label: item.Label,
				Support: protocolCompatibilitySupportResponse{
					Standard:    item.Support.Standard,
					NapCat:      item.Support.NapCat,
					LuckyLillia: item.Support.LuckyLillia,
				},
				Summary: item.Summary,
			})
		}
		categories = append(categories, protocolCompatibilityCategoryResponse{
			Key:   category.Key,
			Title: category.Title,
			Items: items,
		})
	}

	return oneBot11ProtocolCompatibilityResponse{
		Protocol:   matrix.Protocol,
		Categories: categories,
	}, nil
}

func (s *protocolService) protocolSnapshotEvent() managementEventFrame {
	return newEventsReceivedFrame(protocolSnapshotEventPayload{
		Protocol:         "onebot11",
		ProtocolSnapshot: s.currentOneBot11ProtocolSnapshot(),
	})
}

func (s *protocolService) PublishSnapshot() {
	s.publishProtocolEvent(s.protocolSnapshotEvent())
}

func (s *protocolService) transportIngressEnabled(transport adapter.TransportKey) bool {
	if s == nil || s.adapter == nil {
		return false
	}

	snapshot := s.adapter.Snapshot()
	switch transport {
	case adapter.TransportReverseWS:
		return snapshot.ReverseWS.Enabled && snapshot.ReverseWS.Configured
	case adapter.TransportWebhook:
		return snapshot.Webhook.Enabled && snapshot.Webhook.Configured
	default:
		return false
	}
}

func (s *protocolService) publishProtocolEvent(frame managementEventFrame) {
	if s == nil {
		return
	}

	s.mu.RLock()
	subscribers := make([]chan managementEventFrame, 0, len(s.subscribers))
	for _, subscriber := range s.subscribers {
		subscribers = append(subscribers, subscriber)
	}
	s.mu.RUnlock()

	for _, subscriber := range subscribers {
		select {
		case subscriber <- frame:
		default:
		}
	}
}

func (s *protocolService) subscribeProtocolEvents(buffer int) (<-chan managementEventFrame, func()) {
	if buffer <= 0 {
		buffer = 1
	}

	ch := make(chan managementEventFrame, buffer)
	s.mu.Lock()
	id := s.nextSubID
	s.nextSubID++
	s.subscribers[id] = ch
	s.mu.Unlock()

	return ch, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		subscriber, ok := s.subscribers[id]
		if !ok {
			return
		}
		delete(s.subscribers, id)
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
	case "standard", "napcat", "luckylillia":
		return strings.TrimSpace(raw)
	default:
		return "unknown"
	}
}

func oneBot11AvatarURL(userID string) string {
	return "https://q1.qlogo.cn/g?b=qq&nk=" + strings.TrimSpace(userID) + "&s=640"
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
