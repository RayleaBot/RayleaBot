package governance

import (
	"context"
	"errors"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
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
	BlacklistRepo  ManagementEntryRepository
	WhitelistRepo  ManagementEntryRepository
	WhitelistState permission.WhitelistStateRepository
	NotifyChanged  func(string)
}

type EntryResponse struct {
	Scope     chatevent.IdentityScope `json:"scope"`
	EntryType string                  `json:"entry_type"`
	TargetID  string                  `json:"target_id"`
	Reason    string                  `json:"reason"`
	CreatedAt string                  `json:"created_at"`
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
	PluginID            string                 `json:"plugin_id"`
	PluginName          string                 `json:"plugin_name"`
	CommandID           string                 `json:"command_id"`
	Command             string                 `json:"command"`
	Aliases             []string               `json:"aliases"`
	Trigger             CommandTriggerResponse `json:"trigger"`
	DeclaredPermission  *string                `json:"declared_permission"`
	EffectivePermission string                 `json:"effective_permission"`
	PermissionSource    string                 `json:"permission_source"`
}

type CommandTriggerResponse struct {
	Type        string   `json:"type"`
	Names       []string `json:"names,omitempty"`
	Pattern     string   `json:"pattern,omitempty"`
	SettingsKey string   `json:"settings_key,omitempty"`
}

type CommandPolicyResponse struct {
	DefaultLevel string                       `json:"default_level"`
	Cooldown     CommandCooldownResponse      `json:"cooldown"`
	Commands     []CommandPolicyEntryResponse `json:"commands"`
}

type Service struct {
	currentConfig  func() config.Config
	plugins        plugins.CatalogView
	blacklistRepo  ManagementEntryRepository
	whitelistRepo  ManagementEntryRepository
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
	if s.currentConfig == nil {
		return config.Config{}
	}
	return s.currentConfig()
}

func (s *Service) notify() {
	if s.notifyChanged == nil {
		return
	}
	s.notifyChanged(defaultGovernanceSummary)
}

func buildEntryResponse(entry permission.Entry) EntryResponse {
	return EntryResponse{
		Scope:     entry.Scope,
		EntryType: strings.TrimSpace(entry.EntryType),
		TargetID:  strings.TrimSpace(entry.TargetID),
		Reason:    strings.TrimSpace(entry.Reason),
		CreatedAt: strings.TrimSpace(entry.CreatedAt),
	}
}

func validEntryInput(entryType, targetID, reason string) bool {
	return IsEntryType(entryType) && strings.TrimSpace(targetID) != "" && strings.TrimSpace(reason) != ""
}

func validEntryDeleteInput(entryType, targetID string) bool {
	return IsEntryType(entryType) && strings.TrimSpace(targetID) != ""
}

func (s *Service) ReadBlacklist(ctx context.Context) (BlacklistSnapshot, error) {
	if s.blacklistRepo == nil {
		return BlacklistSnapshot{}, ErrServiceUnavailable
	}

	userEntries, err := s.blacklistRepo.List(ctx, "user")
	if err != nil {
		return BlacklistSnapshot{}, err
	}
	groupEntries, err := s.blacklistRepo.List(ctx, "group")
	if err != nil {
		return BlacklistSnapshot{}, err
	}

	return BlacklistSnapshot{
		UserEntries:  buildEntries(userEntries),
		GroupEntries: buildEntries(groupEntries),
	}, nil
}

func (s *Service) UpsertBlacklistEntry(ctx context.Context, scope chatevent.IdentityScope, entryType, targetID, reason string) (EntryResponse, error) {
	entryType = strings.TrimSpace(entryType)
	targetID = strings.TrimSpace(targetID)
	reason = strings.TrimSpace(reason)
	if !scope.Valid() || !validEntryInput(entryType, targetID, reason) {
		return EntryResponse{}, ErrInvalidRequest
	}
	if s.blacklistRepo == nil {
		return EntryResponse{}, ErrServiceUnavailable
	}

	if err := s.blacklistRepo.Add(ctx, scope, entryType, targetID, reason); err != nil {
		return EntryResponse{}, err
	}
	entry, err := s.blacklistRepo.Get(ctx, scope, entryType, targetID)
	if err != nil {
		return EntryResponse{}, err
	}
	s.notify()
	return buildEntryResponse(entry), nil
}

func (s *Service) DeleteBlacklistEntry(ctx context.Context, scope chatevent.IdentityScope, entryType, targetID string) error {
	entryType = strings.TrimSpace(entryType)
	targetID = strings.TrimSpace(targetID)
	if !scope.Valid() || !validEntryDeleteInput(entryType, targetID) {
		return ErrInvalidRequest
	}
	if s.blacklistRepo == nil {
		return ErrServiceUnavailable
	}

	if err := s.blacklistRepo.Remove(ctx, scope, entryType, targetID); err != nil {
		return err
	}
	s.notify()
	return nil
}

func buildEntries(entries []permission.Entry) []EntryResponse {
	if len(entries) == 0 {
		return []EntryResponse{}
	}

	items := make([]EntryResponse, 0, len(entries))
	for _, entry := range entries {
		items = append(items, buildEntryResponse(entry))
	}
	return items
}

func (s *Service) ReadWhitelist(ctx context.Context) (WhitelistSnapshot, error) {

	enabled, err := whitelistEnabled(ctx, s.whitelistState)
	if err != nil {
		return WhitelistSnapshot{}, err
	}
	userEntries, groupEntries, err := whitelistEntries(ctx, s.whitelistRepo)
	if err != nil {
		return WhitelistSnapshot{}, err
	}

	return WhitelistSnapshot{
		Enabled:      enabled,
		UserEntries:  userEntries,
		GroupEntries: groupEntries,
	}, nil
}

func (s *Service) SetWhitelistEnabled(ctx context.Context, enabled bool) (WhitelistStateResponse, error) {
	if s.whitelistState == nil {
		return WhitelistStateResponse{}, ErrServiceUnavailable
	}
	if err := s.whitelistState.SetEnabled(ctx, enabled); err != nil {
		return WhitelistStateResponse{}, err
	}
	s.notify()
	return WhitelistStateResponse{Enabled: enabled}, nil
}

func (s *Service) UpsertWhitelistEntry(ctx context.Context, scope chatevent.IdentityScope, entryType, targetID, reason string) (EntryResponse, error) {
	entryType = strings.TrimSpace(entryType)
	targetID = strings.TrimSpace(targetID)
	reason = strings.TrimSpace(reason)
	if !scope.Valid() || !validEntryInput(entryType, targetID, reason) {
		return EntryResponse{}, ErrInvalidRequest
	}
	if s.whitelistRepo == nil {
		return EntryResponse{}, ErrServiceUnavailable
	}

	if err := s.whitelistRepo.Add(ctx, scope, entryType, targetID, reason); err != nil {
		return EntryResponse{}, err
	}
	entry, err := s.whitelistRepo.Get(ctx, scope, entryType, targetID)
	if err != nil {
		return EntryResponse{}, err
	}
	s.notify()
	return buildEntryResponse(entry), nil
}

func (s *Service) DeleteWhitelistEntry(ctx context.Context, scope chatevent.IdentityScope, entryType, targetID string) error {
	entryType = strings.TrimSpace(entryType)
	targetID = strings.TrimSpace(targetID)
	if !scope.Valid() || !validEntryDeleteInput(entryType, targetID) {
		return ErrInvalidRequest
	}
	if s.whitelistRepo == nil {
		return ErrServiceUnavailable
	}

	if err := s.whitelistRepo.Remove(ctx, scope, entryType, targetID); err != nil {
		return err
	}
	s.notify()
	return nil
}

func whitelistEnabled(ctx context.Context, repo permission.WhitelistStateRepository) (bool, error) {
	if repo == nil {
		return false, ErrServiceUnavailable
	}
	return repo.Enabled(ctx)
}

func whitelistEntries(ctx context.Context, repo permission.EntryRepository) ([]EntryResponse, []EntryResponse, error) {
	if repo == nil {
		return nil, nil, ErrServiceUnavailable
	}

	userEntries, err := repo.List(ctx, "user")
	if err != nil {
		return nil, nil, err
	}
	groupEntries, err := repo.List(ctx, "group")
	if err != nil {
		return nil, nil, err
	}
	return buildEntries(userEntries), buildEntries(groupEntries), nil
}
