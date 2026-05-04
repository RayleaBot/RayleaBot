package governance

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

const (
	defaultUserCommandRateLimit  = "10/60s"
	defaultGroupCommandRateLimit = "30/60s"
	defaultGovernanceSummary     = "治理设置已更新"
)

var (
	ErrServiceUnavailable = errors.New("governance service unavailable")
	ErrInvalidRequest     = errors.New("governance invalid request")
)

type Deps struct {
	CurrentConfig  func() config.Config
	Plugins        *plugins.Catalog
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
	plugins        *plugins.Catalog
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

func (s *Service) ReadBlacklist(ctx context.Context) (BlacklistSnapshot, error) {
	if s == nil || s.blacklistRepo == nil {
		return BlacklistSnapshot{
			UserEntries:  []EntryResponse{},
			GroupEntries: []EntryResponse{},
		}, nil
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
		UserEntries:  buildBlacklistEntries(userEntries),
		GroupEntries: buildBlacklistEntries(groupEntries),
	}, nil
}

func (s *Service) UpsertBlacklistEntry(ctx context.Context, entryType, targetID, reason string) (EntryResponse, error) {
	entryType = strings.TrimSpace(entryType)
	targetID = strings.TrimSpace(targetID)
	reason = strings.TrimSpace(reason)
	if !IsEntryType(entryType) || targetID == "" || reason == "" {
		return EntryResponse{}, ErrInvalidRequest
	}
	if s == nil || s.blacklistRepo == nil {
		return EntryResponse{}, ErrServiceUnavailable
	}

	if err := s.blacklistRepo.Add(ctx, entryType, targetID, reason); err != nil {
		return EntryResponse{}, err
	}
	entry, err := s.blacklistRepo.Get(ctx, entryType, targetID)
	if err != nil {
		return EntryResponse{}, err
	}
	s.notify(defaultGovernanceSummary)
	return buildEntryResponse(entry.EntryType, entry.TargetID, entry.Reason, entry.CreatedAt), nil
}

func (s *Service) DeleteBlacklistEntry(ctx context.Context, entryType, targetID string) error {
	entryType = strings.TrimSpace(entryType)
	targetID = strings.TrimSpace(targetID)
	if !IsEntryType(entryType) || targetID == "" {
		return ErrInvalidRequest
	}
	if s == nil || s.blacklistRepo == nil {
		return ErrServiceUnavailable
	}

	if err := s.blacklistRepo.Remove(ctx, entryType, targetID); err != nil {
		return err
	}
	s.notify(defaultGovernanceSummary)
	return nil
}

func (s *Service) ReadWhitelist(ctx context.Context) (WhitelistSnapshot, error) {
	if s == nil {
		return WhitelistSnapshot{
			Enabled:      false,
			UserEntries:  []EntryResponse{},
			GroupEntries: []EntryResponse{},
		}, nil
	}

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
	if s == nil || s.whitelistState == nil {
		return WhitelistStateResponse{}, ErrServiceUnavailable
	}
	if err := s.whitelistState.SetEnabled(ctx, enabled); err != nil {
		return WhitelistStateResponse{}, err
	}
	s.notify(defaultGovernanceSummary)
	return WhitelistStateResponse{Enabled: enabled}, nil
}

func (s *Service) UpsertWhitelistEntry(ctx context.Context, entryType, targetID, reason string) (EntryResponse, error) {
	entryType = strings.TrimSpace(entryType)
	targetID = strings.TrimSpace(targetID)
	reason = strings.TrimSpace(reason)
	if !IsEntryType(entryType) || targetID == "" || reason == "" {
		return EntryResponse{}, ErrInvalidRequest
	}
	if s == nil || s.whitelistRepo == nil {
		return EntryResponse{}, ErrServiceUnavailable
	}

	if err := s.whitelistRepo.Add(ctx, entryType, targetID, reason); err != nil {
		return EntryResponse{}, err
	}
	entry, err := s.whitelistRepo.Get(ctx, entryType, targetID)
	if err != nil {
		return EntryResponse{}, err
	}
	s.notify(defaultGovernanceSummary)
	return buildEntryResponse(entry.EntryType, entry.TargetID, entry.Reason, entry.CreatedAt), nil
}

func (s *Service) DeleteWhitelistEntry(ctx context.Context, entryType, targetID string) error {
	entryType = strings.TrimSpace(entryType)
	targetID = strings.TrimSpace(targetID)
	if !IsEntryType(entryType) || targetID == "" {
		return ErrInvalidRequest
	}
	if s == nil || s.whitelistRepo == nil {
		return ErrServiceUnavailable
	}

	if err := s.whitelistRepo.Remove(ctx, entryType, targetID); err != nil {
		return err
	}
	s.notify(defaultGovernanceSummary)
	return nil
}

func (s *Service) ReadCommandPolicy(context.Context) (CommandPolicyResponse, error) {
	cfg := s.currentCfg()

	var snapshots []plugins.Snapshot
	if s != nil && s.plugins != nil {
		snapshots = s.plugins.List()
	}

	return CommandPolicyResponse{
		DefaultLevel: commandPermissionDefaultLevel(cfg),
		Cooldown:     cooldownSnapshot(cfg),
		Commands:     buildCommandPolicyEntries(snapshots, cfg),
	}, nil
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

func whitelistEnabled(ctx context.Context, repo permission.WhitelistStateRepository) (bool, error) {
	if repo == nil {
		return false, nil
	}
	return repo.Enabled(ctx)
}

func whitelistEntries(ctx context.Context, repo permission.WhitelistRepository) ([]EntryResponse, []EntryResponse, error) {
	if repo == nil {
		return []EntryResponse{}, []EntryResponse{}, nil
	}

	userEntries, err := repo.List(ctx, "user")
	if err != nil {
		return nil, nil, err
	}
	groupEntries, err := repo.List(ctx, "group")
	if err != nil {
		return nil, nil, err
	}
	return buildWhitelistEntries(userEntries), buildWhitelistEntries(groupEntries), nil
}

func buildEntryResponse(entryType, targetID, reason, createdAt string) EntryResponse {
	return EntryResponse{
		EntryType: strings.TrimSpace(entryType),
		TargetID:  strings.TrimSpace(targetID),
		Reason:    strings.TrimSpace(reason),
		CreatedAt: strings.TrimSpace(createdAt),
	}
}

func buildBlacklistEntries(entries []permission.BlacklistEntry) []EntryResponse {
	if len(entries) == 0 {
		return []EntryResponse{}
	}

	items := make([]EntryResponse, 0, len(entries))
	for _, entry := range entries {
		items = append(items, buildEntryResponse(entry.EntryType, entry.TargetID, entry.Reason, entry.CreatedAt))
	}
	return items
}

func buildWhitelistEntries(entries []permission.WhitelistEntry) []EntryResponse {
	if len(entries) == 0 {
		return []EntryResponse{}
	}

	items := make([]EntryResponse, 0, len(entries))
	for _, entry := range entries {
		items = append(items, buildEntryResponse(entry.EntryType, entry.TargetID, entry.Reason, entry.CreatedAt))
	}
	return items
}

func cooldownSnapshot(cfg config.Config) CommandCooldownResponse {
	userRateLimit := strings.TrimSpace(cfg.User.CommandRateLimit)
	groupRateLimit := strings.TrimSpace(cfg.Group.CommandRateLimit)
	cooldownReply := cfg.User.CooldownReply

	if cfg.Cooldown != nil {
		if trimmed := strings.TrimSpace(cfg.Cooldown.UserCommandRateLimit); trimmed != "" {
			userRateLimit = trimmed
		}
		if trimmed := strings.TrimSpace(cfg.Cooldown.GroupCommandRateLimit); trimmed != "" {
			groupRateLimit = trimmed
		}
		cooldownReply = cfg.Cooldown.CooldownReply
	}

	if userRateLimit == "" {
		userRateLimit = defaultUserCommandRateLimit
	}
	if groupRateLimit == "" {
		groupRateLimit = defaultGroupCommandRateLimit
	}
	if userRateLimit == defaultUserCommandRateLimit && groupRateLimit == defaultGroupCommandRateLimit && !cfg.User.CooldownReply && cfg.Cooldown == nil {
		cooldownReply = true
	}

	return CommandCooldownResponse{
		UserCommandRateLimit:  userRateLimit,
		GroupCommandRateLimit: groupRateLimit,
		CooldownReply:         cooldownReply,
	}
}

func buildCommandPolicyEntries(snapshots []plugins.Snapshot, cfg config.Config) []CommandPolicyEntryResponse {
	items := make([]CommandPolicyEntryResponse, 0)
	for _, snapshot := range snapshots {
		if !pluginParticipatesInCommandPolicy(snapshot) {
			continue
		}
		for _, command := range snapshot.Commands {
			name := strings.TrimSpace(command.Name)
			if name == "" {
				continue
			}
			declaredPermission := normalizedDeclaredCommandPermission(command.Permission)
			effectivePermission := effectiveCommandPermissionLevel(command.Permission, cfg)
			permissionSource := "default_level"
			if declaredPermission != nil {
				permissionSource = "declared"
			}
			items = append(items, CommandPolicyEntryResponse{
				PluginID:            snapshot.PluginID,
				PluginName:          pluginDisplayName(snapshot),
				Command:             name,
				Aliases:             normalizedStrings(command.Aliases),
				CommandSource:       commandSourceOrDefault(command.CommandSource),
				DeclarationID:       strings.TrimSpace(command.DeclarationID),
				DeclaredPermission:  declaredPermission,
				EffectivePermission: effectivePermission,
				PermissionSource:    permissionSource,
			})
		}
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].PluginName != items[j].PluginName {
			return items[i].PluginName < items[j].PluginName
		}
		if items[i].PluginID != items[j].PluginID {
			return items[i].PluginID < items[j].PluginID
		}
		return items[i].Command < items[j].Command
	})

	if len(items) == 0 {
		return []CommandPolicyEntryResponse{}
	}
	return items
}

func commandSourceOrDefault(source string) string {
	if strings.TrimSpace(source) == plugins.CommandSourceDynamic {
		return plugins.CommandSourceDynamic
	}
	return plugins.CommandSourceManifest
}

func pluginDisplayName(snapshot plugins.Snapshot) string {
	if trimmed := strings.TrimSpace(snapshot.Name); trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(snapshot.PluginID)
}

func normalizedStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}

	items := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			items = append(items, trimmed)
		}
	}
	if len(items) == 0 {
		return []string{}
	}
	return items
}

func normalizedDeclaredCommandPermission(raw string) *string {
	switch strings.TrimSpace(raw) {
	case "super_admin", "group_admin", "everyone":
		value := strings.TrimSpace(raw)
		return &value
	default:
		return nil
	}
}

func commandPermissionDefaultLevel(cfg config.Config) string {
	defaultLevel := strings.TrimSpace(cfg.Permission.DefaultLevel)
	if defaultLevel == "" {
		defaultLevel = strings.TrimSpace(cfg.Auth.DefaultLevel)
	}
	switch defaultLevel {
	case "super_admin", "group_admin", "everyone":
		return defaultLevel
	default:
		return "everyone"
	}
}

func effectiveCommandPermissionLevel(permissionLevel string, cfg config.Config) string {
	switch strings.TrimSpace(permissionLevel) {
	case "super_admin", "group_admin", "everyone":
		return strings.TrimSpace(permissionLevel)
	case "":
		return commandPermissionDefaultLevel(cfg)
	default:
		return "everyone"
	}
}

func pluginParticipatesInCommandPolicy(snapshot plugins.Snapshot) bool {
	return snapshot.Valid &&
		snapshot.RegistrationState == "installed" &&
		snapshot.DesiredState == "enabled"
}
