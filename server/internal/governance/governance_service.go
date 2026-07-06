package governance

import (
	"context"
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
	if s.currentConfig == nil {
		return config.Config{}
	}
	return s.currentConfig()
}

func (s *Service) notify(summary string) {
	if s.notifyChanged == nil {
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

func (s *Service) ReadBlacklist(ctx context.Context) (BlacklistSnapshot, error) {
	if s.blacklistRepo == nil {
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
	if !validEntryInput(entryType, targetID, reason) {
		return EntryResponse{}, ErrInvalidRequest
	}
	if s.blacklistRepo == nil {
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
	if !validEntryDeleteInput(entryType, targetID) {
		return ErrInvalidRequest
	}
	if s.blacklistRepo == nil {
		return ErrServiceUnavailable
	}

	if err := s.blacklistRepo.Remove(ctx, entryType, targetID); err != nil {
		return err
	}
	s.notify(defaultGovernanceSummary)
	return nil
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
	s.notify(defaultGovernanceSummary)
	return WhitelistStateResponse{Enabled: enabled}, nil
}

func (s *Service) UpsertWhitelistEntry(ctx context.Context, entryType, targetID, reason string) (EntryResponse, error) {
	entryType = strings.TrimSpace(entryType)
	targetID = strings.TrimSpace(targetID)
	reason = strings.TrimSpace(reason)
	if !validEntryInput(entryType, targetID, reason) {
		return EntryResponse{}, ErrInvalidRequest
	}
	if s.whitelistRepo == nil {
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
	if !validEntryDeleteInput(entryType, targetID) {
		return ErrInvalidRequest
	}
	if s.whitelistRepo == nil {
		return ErrServiceUnavailable
	}

	if err := s.whitelistRepo.Remove(ctx, entryType, targetID); err != nil {
		return err
	}
	s.notify(defaultGovernanceSummary)
	return nil
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
