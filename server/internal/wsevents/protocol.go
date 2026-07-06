package wsevents

import (
	"context"
	"strings"
	"time"

	"github.com/coder/websocket"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/configruntime"
	"github.com/RayleaBot/RayleaBot/server/internal/onebot11"
	"github.com/RayleaBot/RayleaBot/server/internal/pubsub"
)

type ProtocolIssue struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Summary  string `json:"summary"`
}

type TransportStatus struct {
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

type OneBot11ProtocolSnapshot struct {
	Protocol              string            `json:"protocol"`
	Provider              string            `json:"provider"`
	ConfiguredTransports  []string          `json:"configured_transports"`
	ActiveTransports      []string          `json:"active_transports"`
	TransportStatus       []TransportStatus `json:"transport_status"`
	ReadinessStatus       string            `json:"readiness_status"`
	Summary               string            `json:"summary"`
	RecentTransportIssues []ProtocolIssue   `json:"recent_transport_issues"`
}

type OneBot11TargetIssue struct {
	Scope   string `json:"scope"`
	Message string `json:"message"`
}

type OneBot11GroupTarget struct {
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	TargetName string `json:"target_name"`
	AvatarURL  string `json:"avatar_url,omitempty"`
}

type OneBot11PrivateTarget struct {
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	Nickname   string `json:"nickname"`
	AvatarURL  string `json:"avatar_url,omitempty"`
}

type OneBot11ProtocolTargets struct {
	Protocol     string                  `json:"protocol"`
	Available    bool                    `json:"available"`
	Groups       []OneBot11GroupTarget   `json:"groups"`
	PrivateUsers []OneBot11PrivateTarget `json:"private_users"`
	Issues       []OneBot11TargetIssue   `json:"issues"`
}

type OneBot11IdentityResolveItem struct {
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	UserID     string `json:"user_id"`
}

type OneBot11Identity struct {
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

type OneBot11IdentityResolveResult struct {
	Items  []OneBot11Identity    `json:"items"`
	Issues []OneBot11TargetIssue `json:"issues"`
}

type CompatibilitySupport struct {
	Standard    string `json:"standard"`
	NapCat      string `json:"napcat"`
	LuckyLillia string `json:"luckylillia"`
}

type CompatibilityItem struct {
	Key     string               `json:"key"`
	Label   string               `json:"label"`
	Support CompatibilitySupport `json:"support"`
	Summary string               `json:"summary"`
}

type CompatibilityCategory struct {
	Key   string              `json:"key"`
	Title string              `json:"title"`
	Items []CompatibilityItem `json:"items"`
}

type OneBot11ProtocolCompatibility struct {
	Protocol   string                  `json:"protocol"`
	Categories []CompatibilityCategory `json:"categories"`
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
		ProtocolSnapshot: s.CurrentOneBot11ProtocolSnapshot(),
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

func (s *ProtocolService) ReverseWSIngressAvailable() bool {
	return s != nil && s.adapter != nil
}

func (s *ProtocolService) ReverseWSIngressEnabled() bool {
	return s.transportIngressEnabled(onebot11.TransportReverseWS)
}

func (s *ProtocolService) ReverseWSAccessToken() string {
	if s == nil || s.config == nil {
		return ""
	}
	return s.config.CurrentConfig().OneBot.ReverseWS.AccessToken
}

func (s *ProtocolService) ReverseWSAccessTokenQueryCompat() bool {
	if s == nil || s.config == nil {
		return false
	}
	return s.config.CurrentConfig().OneBot.ReverseWS.AccessTokenQueryCompat
}

func (s *ProtocolService) MarkReverseWSAuthFailed() {
	if s == nil || s.adapter == nil {
		return
	}
	s.adapter.MarkReverseWSAuthFailed()
}

func (s *ProtocolService) AttachReverseWS(conn *websocket.Conn) {
	if s == nil || s.adapter == nil {
		return
	}
	s.adapter.AttachReverseWS(conn)
}

func (s *ProtocolService) WebhookIngressAvailable() bool {
	return s != nil && s.adapter != nil
}

func (s *ProtocolService) WebhookIngressEnabled() bool {
	return s.transportIngressEnabled(onebot11.TransportWebhook)
}

func (s *ProtocolService) WebhookAccessToken() string {
	if s == nil || s.config == nil {
		return ""
	}
	return s.config.CurrentConfig().OneBot.Webhook.AccessToken
}

func (s *ProtocolService) WebhookAccessTokenQueryCompat() bool {
	if s == nil || s.config == nil {
		return false
	}
	return s.config.CurrentConfig().OneBot.Webhook.AccessTokenQueryCompat
}

func (s *ProtocolService) MarkWebhookAuthFailed() {
	if s == nil || s.adapter == nil {
		return
	}
	s.adapter.MarkWebhookAuthFailed()
}

func (s *ProtocolService) AcceptWebhookPayload(ctx context.Context, payload []byte) error {
	if s == nil || s.adapter == nil {
		return configruntime.ErrProtocolStopped
	}
	return s.adapter.AcceptWebhookPayload(ctx, payload)
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
