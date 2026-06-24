package governance

import (
	"errors"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

const (
	defaultGovernanceSummary = "治理设置已更新"
)

var (
	ErrServiceUnavailable = errors.New("governance service unavailable")
	ErrInvalidRequest     = errors.New("governance invalid request")
)

type Deps struct {
	CurrentConfig  func() config.Config
	Plugins        plugins.CatalogView
	BlacklistRepo  permission.BlacklistRepository
	WhitelistRepo  permission.WhitelistRepository
	WhitelistState permission.WhitelistStateRepository
	NotifyChanged  func(string)
}

type EntryResponse struct {
	EntryType string `json:"entry_type"`
	TargetID  string `json:"target_id"`
	Reason    string `json:"reason"`
	CreatedAt string `json:"created_at"`
}

type BlacklistSnapshot struct {
	UserEntries  []EntryResponse `json:"user_entries"`
	GroupEntries []EntryResponse `json:"group_entries"`
}

type WhitelistSnapshot struct {
	Enabled      bool            `json:"enabled"`
	UserEntries  []EntryResponse `json:"user_entries"`
	GroupEntries []EntryResponse `json:"group_entries"`
}

type WhitelistStateResponse struct {
	Enabled bool `json:"enabled"`
}

type CommandCooldownResponse struct {
	UserCommandRateLimit  string `json:"user_command_rate_limit"`
	GroupCommandRateLimit string `json:"group_command_rate_limit"`
	CooldownReply         bool   `json:"cooldown_reply"`
}

type CommandPolicyEntryResponse struct {
	PluginID            string   `json:"plugin_id"`
	PluginName          string   `json:"plugin_name"`
	Command             string   `json:"command"`
	Aliases             []string `json:"aliases"`
	CommandSource       string   `json:"command_source"`
	DeclarationID       string   `json:"declaration_id,omitempty"`
	DeclaredPermission  *string  `json:"declared_permission"`
	EffectivePermission string   `json:"effective_permission"`
	PermissionSource    string   `json:"permission_source"`
}

type CommandPolicyResponse struct {
	DefaultLevel string                       `json:"default_level"`
	Cooldown     CommandCooldownResponse      `json:"cooldown"`
	Commands     []CommandPolicyEntryResponse `json:"commands"`
}

type Service struct {
	currentConfig  func() config.Config
	plugins        plugins.CatalogView
	blacklistRepo  permission.BlacklistRepository
	whitelistRepo  permission.WhitelistRepository
	whitelistState permission.WhitelistStateRepository
	notifyChanged  func(string)
}

func NewService(deps Deps) *Service {
	return &Service{
		currentConfig:  deps.CurrentConfig,
		plugins:        deps.Plugins,
		blacklistRepo:  deps.BlacklistRepo,
		whitelistRepo:  deps.WhitelistRepo,
		whitelistState: deps.WhitelistState,
		notifyChanged:  deps.NotifyChanged,
	}
}

func IsEntryType(value string) bool {
	switch strings.TrimSpace(value) {
	case "user", "group":
		return true
	default:
		return false
	}
}

func (s *Service) currentCfg() config.Config {
	if s == nil || s.currentConfig == nil {
		return config.Config{}
	}
	return s.currentConfig()
}

func (s *Service) notify(summary string) {
	if s == nil || s.notifyChanged == nil {
		return
	}
	s.notifyChanged(strings.TrimSpace(summary))
}

func buildEntryResponse(entryType, targetID, reason, createdAt string) EntryResponse {
	return EntryResponse{
		EntryType: strings.TrimSpace(entryType),
		TargetID:  strings.TrimSpace(targetID),
		Reason:    strings.TrimSpace(reason),
		CreatedAt: strings.TrimSpace(createdAt),
	}
}

func validEntryInput(entryType, targetID, reason string) bool {
	return IsEntryType(entryType) && strings.TrimSpace(targetID) != "" && strings.TrimSpace(reason) != ""
}

func validEntryDeleteInput(entryType, targetID string) bool {
	return IsEntryType(entryType) && strings.TrimSpace(targetID) != ""
}
