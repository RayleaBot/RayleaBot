package wsevents

import (
	"strings"
	"time"

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
	config ProtocolConfigSource
	// adapter is the primary OneBot instance, which the OneBot-specific
	// management endpoints speak about; oneBotShells holds every instance,
	// which is what the per-instance ingress needs.
	adapter                   *onebot11.Shell
	oneBotShells              map[string]*onebot11.Shell
	qqClients                 map[string]QQOfficialStatusSource
	oneBot11TargetReadTimeout time.Duration
	hub                       pubsub.Hub[Frame]
}

// ProtocolServiceAdapters are the running adapters, keyed by instance id.
type ProtocolServiceAdapters struct {
	OneBot11   map[string]*onebot11.Shell
	QQOfficial map[string]QQOfficialStatusSource
	// PrimaryOneBot11 is the instance the OneBot management endpoints report on.
	PrimaryOneBot11 *onebot11.Shell
}

func NewProtocolService(configSource ProtocolConfigSource, adapters ProtocolServiceAdapters) *ProtocolService {
	return &ProtocolService{
		config:                    configSource,
		adapter:                   adapters.PrimaryOneBot11,
		oneBotShells:              adapters.OneBot11,
		qqClients:                 adapters.QQOfficial,
		oneBot11TargetReadTimeout: 3 * time.Second,
	}
}

func (s *ProtocolService) ApplyConfigReload(cfg config.Config) error {
	if s.adapter == nil {
		return nil
	}
	if s.adapter.Snapshot().State == onebot11.StateStopped {
		return configruntime.ErrProtocolStopped
	}
	return s.adapter.Reload(primaryOneBotSettingsOf(cfg), cfg.Adapter)
}

func (s *ProtocolService) ProtocolSnapshotEvent() Frame {
	return NewReceivedFrame(ProtocolSnapshotPayload{
		Protocol:         "onebot11",
		ProtocolSnapshot: s.CurrentOneBot11ProtocolSnapshot(),
	})
}

func (s *ProtocolService) PublishSnapshot() {
	s.hub.Publish(s.ProtocolSnapshotEvent())
}

func (s *ProtocolService) SubscribeProtocolEvents(buffer int) (<-chan Frame, func()) {
	return s.hub.Subscribe(buffer)
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

// primaryOneBotSettings returns the first configured OneBot adapter's settings.
// The OneBot protocol surface predates multiple adapters and still speaks about
// a single connection; this makes "which one" explicit rather than implied.
func (s *ProtocolService) primaryOneBotSettings() config.OneBotConfig {
	return primaryOneBotSettingsOf(s.config.CurrentConfig())
}

func primaryOneBotSettingsOf(cfg config.Config) config.OneBotConfig {
	if _, settings, ok := cfg.PrimaryOneBot11(); ok {
		return settings
	}
	return config.OneBotConfig{}
}
