package adapters

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
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

type ConfigSource interface {
	CurrentConfig() config.Config
}

type Service struct {
	config                    ConfigSource
	oneBotShells              map[string]*onebot11.Shell
	qqClients                 map[string]QQOfficialAdapter
	oneBot11TargetReadTimeout time.Duration
	hub                       pubsub.Hub[AdaptersView]
	snapshotMu                sync.Mutex
	lifecycleMu               sync.Mutex
	stopMu                    sync.Mutex
	stopped                   bool
}

// Instances are the configured adapters, keyed by instance id.
type Instances struct {
	// OneBot11 contains every configured instance, including disabled ones.
	OneBot11   map[string]*onebot11.Shell
	QQOfficial map[string]QQOfficialAdapter
}

var ErrStopped = errors.New("adapter service stopped")

func NewService(configSource ConfigSource, instances Instances) (*Service, error) {
	if configSource == nil {
		return nil, errors.New("adapter config source is required")
	}
	oneBotShells := make(map[string]*onebot11.Shell, len(instances.OneBot11))
	for id, shell := range instances.OneBot11 {
		if shell == nil {
			return nil, fmt.Errorf("adapter %s has no OneBot runtime", id)
		}
		oneBotShells[id] = shell
	}
	qqClients := make(map[string]QQOfficialAdapter, len(instances.QQOfficial))
	for id, client := range instances.QQOfficial {
		if client == nil {
			return nil, fmt.Errorf("adapter %s has no QQ runtime", id)
		}
		qqClients[id] = client
	}
	return &Service{
		config:                    configSource,
		oneBotShells:              oneBotShells,
		qqClients:                 qqClients,
		oneBot11TargetReadTimeout: 3 * time.Second,
	}, nil
}

// ApplyConfigReload applies the new configuration to every running adapter.
// An instance whose settings did not change is left connected; one that is no
// longer configured is left to a restart, because removing an adapter is a
// change to the set of adapters rather than to one adapter's settings.
func (s *Service) ApplyConfigReload(cfg config.Config) error {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if s.stopped {
		return ErrStopped
	}
	failures := make([]error, 0, len(s.oneBotShells)+len(s.qqClients))
	for _, instance := range cfg.Adapters {
		if instance.Type == config.AdapterTypeOneBot11 && s.oneBotShell(instance.ID) == nil ||
			instance.Type == config.AdapterTypeQQOfficial && s.qqClient(instance.ID) == nil {
			// Adding an instance changes the collection and requires a restart.
			// A subsequent settings save must retain that requirement.
			failures = append(failures, ErrStopped)
		}
	}

	for id, shell := range s.oneBotShells {
		settings, ok := cfg.OneBot11RuntimeSettings(id)
		if !ok {
			continue
		}
		if shell.Snapshot().State == onebot11.StateStopped {
			failures = append(failures, ErrStopped)
			continue
		}
		if err := shell.Reload(settings, cfg.Adapter); err != nil {
			failures = append(failures, fmt.Errorf("adapter %s: %w", id, err))
		}
	}

	for id, client := range s.qqClients {
		settings, ok := cfg.QQOfficialSettings(id)
		if !ok {
			continue
		}
		// The client logs the reconnect itself, where the adapter id and the
		// new settings are both in hand.
		client.Reload(settings)
		instance, _ := cfg.AdapterByID(id)
		client.SetEnabled(instance.Enabled)
	}

	// Preserve each adapter failure so the configuration coordinator can retain
	// the effective settings and report fields that require a restart.
	if len(failures) == 1 {
		return failures[0]
	}
	return errors.Join(failures...)
}

func (s *Service) PublishSnapshot() {
	s.snapshotMu.Lock()
	defer s.snapshotMu.Unlock()
	snapshot := s.Adapters()
	s.hub.PublishReplaceEach(func() AdaptersView { return cloneView(snapshot) })
}

// SnapshotAndSubscribe excludes already-published events from a new stream,
// so an initial snapshot cannot be followed by an older queued snapshot.
func (s *Service) SnapshotAndSubscribe(buffer int) (AdaptersView, <-chan AdaptersView, func()) {
	s.snapshotMu.Lock()
	defer s.snapshotMu.Unlock()
	snapshot := s.Adapters()
	channel, unsubscribe := s.hub.Subscribe(buffer)
	return snapshot, channel, unsubscribe
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
