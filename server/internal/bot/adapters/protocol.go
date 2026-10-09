package adapters

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/adapters/onebot11"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/adapters/qqofficial"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/pubsub"
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

type ConfigSource interface {
	CurrentConfig() config.Config
}

type Service struct {
	observe                   func(AdaptersView)
	published                 func()
	reloaded                  func(string)
	config                    ConfigSource
	registry                  *Registry
	newOneBot11               func(string, config.OneBotConfig, config.AdapterConfig) *onebot11.Shell
	newQQOfficial             func(string, config.QQOfficialConfig, config.AdapterConfig) QQOfficialAdapter
	runCtx                    context.Context
	oneBot11TargetReadTimeout time.Duration
	hub                       pubsub.Hub[AdaptersView]
	snapshotMu                sync.Mutex
	lifecycleMu               sync.Mutex
	stopMu                    sync.Mutex
	stopped                   bool
}

// Instances are the configured adapters, keyed by instance id.
type Instances struct {
	Observe       func(AdaptersView)
	Published     func()
	Reloaded      func(string)
	Registry      *Registry
	NewOneBot11   func(string, config.OneBotConfig, config.AdapterConfig) *onebot11.Shell
	NewQQOfficial func(string, config.QQOfficialConfig, config.AdapterConfig) QQOfficialAdapter
	// OneBot11 contains every configured instance, including disabled ones.
	OneBot11   map[string]*onebot11.Shell
	QQOfficial map[string]QQOfficialAdapter
}

var ErrStopped = errors.New("adapter service stopped")

func NewService(configSource ConfigSource, instances Instances) (*Service, error) {
	if configSource == nil {
		return nil, errors.New("adapter config source is required")
	}
	if instances.Observe == nil {
		instances.Observe = func(AdaptersView) {}
	}
	if instances.Published == nil {
		instances.Published = func() {}
	}
	if instances.Reloaded == nil {
		instances.Reloaded = func(string) {}
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
	if instances.Registry == nil {
		instances.Registry = NewRegistry(configSource.CurrentConfig(), oneBotShells, qqClients)
	}
	if instances.NewOneBot11 == nil {
		instances.NewOneBot11 = func(id string, cfg config.OneBotConfig, adapter config.AdapterConfig) *onebot11.Shell {
			return onebot11.New(id, cfg, adapter, nil)
		}
	}
	if instances.NewQQOfficial == nil {
		instances.NewQQOfficial = func(id string, cfg config.QQOfficialConfig, adapter config.AdapterConfig) QQOfficialAdapter {
			return qqofficial.New(id, cfg, adapter, nil)
		}
	}
	return &Service{
		observe: instances.Observe, reloaded: instances.Reloaded, published: instances.Published,
		config:   configSource,
		registry: instances.Registry, newOneBot11: instances.NewOneBot11, newQQOfficial: instances.NewQQOfficial,
		oneBot11TargetReadTimeout: 3 * time.Second,
	}, nil
}

// ApplyConfigReload serializes membership changes with start and stop. Runtime
// shutdown does not hold any reader lock, and unchanged instances are reused.
func (s *Service) ApplyConfigReload(cfg config.Config) error {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if s.stopped {
		return ErrStopped
	}
	previous := s.registry.Snapshot()
	oneBots := make(map[string]*onebot11.Shell)
	qqClients := make(map[string]QQOfficialAdapter)
	retired := make([]stopEntry, 0)
	for id, shell := range previous.oneBot11 {
		if instance, ok := cfg.AdapterByID(id); ok && instance.Type == config.AdapterTypeOneBot11 {
			oneBots[id] = shell
		} else {
			retired = append(retired, stopEntry{id: id, stop: shell.Stop})
		}
	}
	for id, client := range previous.qqOfficial {
		if instance, ok := cfg.AdapterByID(id); ok && instance.Type == config.AdapterTypeQQOfficial {
			qqClients[id] = client
		} else {
			retired = append(retired, stopEntry{id: id, stop: client.Stop})
		}
	}
	for _, instance := range cfg.Adapters {
		switch instance.Type {
		case config.AdapterTypeOneBot11:
			settings, _ := cfg.OneBot11RuntimeSettings(instance.ID)
			if shell := oneBots[instance.ID]; shell != nil {
				if shell.Snapshot().State == onebot11.StateStopped {
					return ErrStopped
				}
				if err := shell.Reload(settings, cfg.Adapter); err != nil {
					return fmt.Errorf("adapter %s: %w", instance.ID, err)
				}
				if old, ok := previous.cfg.OneBot11RuntimeSettings(instance.ID); !ok || old != settings || previous.cfg.Adapter != cfg.Adapter {
					s.reloaded(instance.ID)
				}
			}
		case config.AdapterTypeQQOfficial:
			if client := qqClients[instance.ID]; client != nil {
				settings, _ := cfg.QQOfficialSettings(instance.ID)
				if client.Reload(settings) {
					s.reloaded(instance.ID)
				}
				client.SetEnabled(instance.Enabled)
			}
		}
	}
	// Wait for removed runtimes before publishing replacements under the same id.
	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := stopAdapters(stopCtx, retired); err != nil {
		return err
	}
	for _, instance := range cfg.Adapters {
		switch instance.Type {
		case config.AdapterTypeOneBot11:
			if oneBots[instance.ID] == nil {
				settings, _ := cfg.OneBot11RuntimeSettings(instance.ID)
				oneBots[instance.ID] = s.newOneBot11(instance.ID, settings, cfg.Adapter)
			}
		case config.AdapterTypeQQOfficial:
			if qqClients[instance.ID] == nil {
				settings, _ := cfg.QQOfficialSettings(instance.ID)
				client := s.newQQOfficial(instance.ID, settings, cfg.Adapter)
				client.SetEnabled(instance.Enabled)
				qqClients[instance.ID] = client
			}
		}
	}
	s.registry.store(cfg, oneBots, qqClients)
	s.PublishSnapshot()
	// Membership must be visible before a new connection can emit events.
	if s.runCtx != nil {
		for id, shell := range oneBots {
			if shell != previous.oneBot11[id] {
				shell.Start(s.runCtx)
			}
		}
		for id, client := range qqClients {
			if client != previous.qqOfficial[id] {
				client.Start(s.runCtx)
			}
		}
	}
	return nil
}

func (s *Service) PublishSnapshot() {
	s.snapshotMu.Lock()
	snapshot := s.Adapters()
	s.observe(snapshot)
	s.hub.PublishReplaceEach(func() AdaptersView { return cloneView(snapshot) })
	s.snapshotMu.Unlock()
	// Status and plugin notifications have their own serialization and may do IO.
	s.published()
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
