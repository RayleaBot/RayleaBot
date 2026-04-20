package governance

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

const (
	defaultUserCommandRateLimit  = "10/60s"
	defaultGroupCommandRateLimit = "30/60s"
)

type Deps struct {
	CurrentConfig  func() config.Config
	Plugins        *plugins.Catalog
	BlacklistRepo  permission.BlacklistRepository
	WhitelistRepo  permission.WhitelistRepository
	WhitelistState permission.WhitelistStateRepository
}

type Handlers struct {
	currentConfig  func() config.Config
	plugins        *plugins.Catalog
	blacklistRepo  permission.BlacklistRepository
	whitelistRepo  permission.WhitelistRepository
	whitelistState permission.WhitelistStateRepository
}

func NewHandlers(deps Deps) *Handlers {
	return &Handlers{
		currentConfig:  deps.CurrentConfig,
		plugins:        deps.Plugins,
		blacklistRepo:  deps.BlacklistRepo,
		whitelistRepo:  deps.WhitelistRepo,
		whitelistState: deps.WhitelistState,
	}
}

func (h *Handlers) RegisterProtectedRoutes(router chi.Router) {
	if router == nil {
		return
	}
	router.Get("/api/governance/blacklist", h.handleGovernanceBlacklist())
	router.Post("/api/governance/blacklist/entries", h.handleGovernanceBlacklistEntryUpsert())
	router.Delete("/api/governance/blacklist/entries/{entry_type}/{target_id}", h.handleGovernanceBlacklistEntryDelete())
	router.Get("/api/governance/whitelist", h.handleGovernanceWhitelist())
	router.Put("/api/governance/whitelist/state", h.handleGovernanceWhitelistStatePut())
	router.Post("/api/governance/whitelist/entries", h.handleGovernanceWhitelistEntryUpsert())
	router.Delete("/api/governance/whitelist/entries/{entry_type}/{target_id}", h.handleGovernanceWhitelistEntryDelete())
	router.Get("/api/governance/command-policy", h.handleGovernanceCommandPolicy())
}

type governanceEntryResponse struct {
	EntryType string `json:"entry_type"`
	TargetID  string `json:"target_id"`
	Reason    string `json:"reason"`
	CreatedAt string `json:"created_at"`
}

type governanceBlacklistResponse struct {
	UserEntries  []governanceEntryResponse `json:"user_entries"`
	GroupEntries []governanceEntryResponse `json:"group_entries"`
}

type governanceWhitelistResponse struct {
	Enabled      bool                      `json:"enabled"`
	UserEntries  []governanceEntryResponse `json:"user_entries"`
	GroupEntries []governanceEntryResponse `json:"group_entries"`
}

type governanceWhitelistStateResponse struct {
	Enabled bool `json:"enabled"`
}

type governanceEntryUpsertRequest struct {
	EntryType string `json:"entry_type"`
	TargetID  string `json:"target_id"`
	Reason    string `json:"reason"`
}

type governanceWhitelistStateUpdateRequest struct {
	Enabled *bool `json:"enabled"`
}

type governanceCommandCooldownResponse struct {
	UserCommandRateLimit  string `json:"user_command_rate_limit"`
	GroupCommandRateLimit string `json:"group_command_rate_limit"`
	CooldownReply         bool   `json:"cooldown_reply"`
}

type governanceCommandPolicyEntryResponse struct {
	PluginID            string   `json:"plugin_id"`
	PluginName          string   `json:"plugin_name"`
	Command             string   `json:"command"`
	Aliases             []string `json:"aliases"`
	DeclaredPermission  *string  `json:"declared_permission"`
	EffectivePermission string   `json:"effective_permission"`
	PermissionSource    string   `json:"permission_source"`
}

type governanceCommandPolicyResponse struct {
	DefaultLevel string                                 `json:"default_level"`
	Cooldown     governanceCommandCooldownResponse      `json:"cooldown"`
	Commands     []governanceCommandPolicyEntryResponse `json:"commands"`
}

func (h *Handlers) currentCfg() config.Config {
	if h == nil || h.currentConfig == nil {
		return config.Config{}
	}
	return h.currentConfig()
}

func (h *Handlers) handleGovernanceBlacklist() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h == nil || h.blacklistRepo == nil {
			httpapi.WriteJSON(w, http.StatusOK, governanceBlacklistResponse{
				UserEntries:  []governanceEntryResponse{},
				GroupEntries: []governanceEntryResponse{},
			})
			return
		}

		userEntries, err := h.blacklistRepo.List(r.Context(), "user")
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}
		groupEntries, err := h.blacklistRepo.List(r.Context(), "group")
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, governanceBlacklistResponse{
			UserEntries:  buildGovernanceBlacklistEntries(userEntries),
			GroupEntries: buildGovernanceBlacklistEntries(groupEntries),
		})
	}
}

func (h *Handlers) handleGovernanceBlacklistEntryUpsert() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h == nil || h.blacklistRepo == nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}

		request, ok := decodeGovernanceEntryUpsertRequest(w, r)
		if !ok {
			return
		}

		if err := h.blacklistRepo.Add(r.Context(), request.EntryType, request.TargetID, request.Reason); err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}

		entry, err := h.blacklistRepo.Get(r.Context(), request.EntryType, request.TargetID)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, buildGovernanceEntryResponse(entry.EntryType, entry.TargetID, entry.Reason, entry.CreatedAt))
	}
}

func (h *Handlers) handleGovernanceBlacklistEntryDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h == nil || h.blacklistRepo == nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}

		entryType, targetID, ok := readGovernanceEntryPath(w, r)
		if !ok {
			return
		}

		if err := h.blacklistRepo.Remove(r.Context(), entryType, targetID); err != nil {
			if errors.Is(err, permission.ErrGovernanceEntryNotFound) {
				httpapi.WriteError(w, r, http.StatusNotFound, "platform.resource_missing", "缺少必要资源", "errors.platform.resource_missing", map[string]any{
					"entry_type": entryType,
					"target_id":  targetID,
				})
				return
			}
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *Handlers) handleGovernanceWhitelist() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h == nil {
			httpapi.WriteJSON(w, http.StatusOK, governanceWhitelistResponse{
				Enabled:      false,
				UserEntries:  []governanceEntryResponse{},
				GroupEntries: []governanceEntryResponse{},
			})
			return
		}

		enabled, err := governanceWhitelistEnabled(r.Context(), h.whitelistState)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}

		userEntries, groupEntries, err := governanceWhitelistEntries(r.Context(), h.whitelistRepo)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, governanceWhitelistResponse{
			Enabled:      enabled,
			UserEntries:  userEntries,
			GroupEntries: groupEntries,
		})
	}
}

func (h *Handlers) handleGovernanceWhitelistStatePut() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h == nil || h.whitelistState == nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}

		var request governanceWhitelistStateUpdateRequest
		if err := httpapi.DecodeStrictJSON(w, r, &request, httpapi.MaxManagementJSONBodyBytes); err != nil || request.Enabled == nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "platform.invalid_request", "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}

		if err := h.whitelistState.SetEnabled(r.Context(), *request.Enabled); err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, governanceWhitelistStateResponse{Enabled: *request.Enabled})
	}
}

func (h *Handlers) handleGovernanceWhitelistEntryUpsert() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h == nil || h.whitelistRepo == nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}

		request, ok := decodeGovernanceEntryUpsertRequest(w, r)
		if !ok {
			return
		}

		if err := h.whitelistRepo.Add(r.Context(), request.EntryType, request.TargetID, request.Reason); err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}

		entry, err := h.whitelistRepo.Get(r.Context(), request.EntryType, request.TargetID)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, buildGovernanceEntryResponse(entry.EntryType, entry.TargetID, entry.Reason, entry.CreatedAt))
	}
}

func (h *Handlers) handleGovernanceWhitelistEntryDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h == nil || h.whitelistRepo == nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}

		entryType, targetID, ok := readGovernanceEntryPath(w, r)
		if !ok {
			return
		}

		if err := h.whitelistRepo.Remove(r.Context(), entryType, targetID); err != nil {
			if errors.Is(err, permission.ErrGovernanceEntryNotFound) {
				httpapi.WriteError(w, r, http.StatusNotFound, "platform.resource_missing", "缺少必要资源", "errors.platform.resource_missing", map[string]any{
					"entry_type": entryType,
					"target_id":  targetID,
				})
				return
			}
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *Handlers) handleGovernanceCommandPolicy() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg := h.currentCfg()

		var snapshots []plugins.Snapshot
		if h != nil && h.plugins != nil {
			snapshots = h.plugins.List()
		}

		httpapi.WriteJSON(w, http.StatusOK, governanceCommandPolicyResponse{
			DefaultLevel: commandPermissionDefaultLevel(cfg),
			Cooldown:     governanceCooldownSnapshot(cfg),
			Commands:     buildGovernanceCommandPolicyEntries(snapshots, cfg),
		})
	}
}

func decodeGovernanceEntryUpsertRequest(w http.ResponseWriter, r *http.Request) (governanceEntryUpsertRequest, bool) {
	var request governanceEntryUpsertRequest
	if err := httpapi.DecodeStrictJSON(w, r, &request, httpapi.MaxManagementJSONBodyBytes); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "platform.invalid_request", "请求参数不合法", "errors.platform.invalid_request", nil)
		return governanceEntryUpsertRequest{}, false
	}

	request.EntryType = strings.TrimSpace(request.EntryType)
	request.TargetID = strings.TrimSpace(request.TargetID)
	request.Reason = strings.TrimSpace(request.Reason)
	if !isGovernanceEntryType(request.EntryType) || request.TargetID == "" || request.Reason == "" {
		httpapi.WriteError(w, r, http.StatusBadRequest, "platform.invalid_request", "请求参数不合法", "errors.platform.invalid_request", nil)
		return governanceEntryUpsertRequest{}, false
	}

	return request, true
}

func readGovernanceEntryPath(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	entryType := strings.TrimSpace(chi.URLParam(r, "entry_type"))
	targetID := strings.TrimSpace(chi.URLParam(r, "target_id"))
	if !isGovernanceEntryType(entryType) || targetID == "" {
		httpapi.WriteError(w, r, http.StatusBadRequest, "platform.invalid_request", "请求参数不合法", "errors.platform.invalid_request", nil)
		return "", "", false
	}
	return entryType, targetID, true
}

func isGovernanceEntryType(value string) bool {
	switch strings.TrimSpace(value) {
	case "user", "group":
		return true
	default:
		return false
	}
}

func governanceWhitelistEnabled(ctx context.Context, repo permission.WhitelistStateRepository) (bool, error) {
	if repo == nil {
		return false, nil
	}
	return repo.Enabled(ctx)
}

func governanceWhitelistEntries(ctx context.Context, repo permission.WhitelistRepository) ([]governanceEntryResponse, []governanceEntryResponse, error) {
	if repo == nil {
		return []governanceEntryResponse{}, []governanceEntryResponse{}, nil
	}

	userEntries, err := repo.List(ctx, "user")
	if err != nil {
		return nil, nil, err
	}
	groupEntries, err := repo.List(ctx, "group")
	if err != nil {
		return nil, nil, err
	}
	return buildGovernanceWhitelistEntries(userEntries), buildGovernanceWhitelistEntries(groupEntries), nil
}

func buildGovernanceEntryResponse(entryType, targetID, reason, createdAt string) governanceEntryResponse {
	return governanceEntryResponse{
		EntryType: strings.TrimSpace(entryType),
		TargetID:  strings.TrimSpace(targetID),
		Reason:    strings.TrimSpace(reason),
		CreatedAt: strings.TrimSpace(createdAt),
	}
}

func buildGovernanceBlacklistEntries(entries []permission.BlacklistEntry) []governanceEntryResponse {
	if len(entries) == 0 {
		return []governanceEntryResponse{}
	}

	items := make([]governanceEntryResponse, 0, len(entries))
	for _, entry := range entries {
		items = append(items, buildGovernanceEntryResponse(entry.EntryType, entry.TargetID, entry.Reason, entry.CreatedAt))
	}
	return items
}

func buildGovernanceWhitelistEntries(entries []permission.WhitelistEntry) []governanceEntryResponse {
	if len(entries) == 0 {
		return []governanceEntryResponse{}
	}

	items := make([]governanceEntryResponse, 0, len(entries))
	for _, entry := range entries {
		items = append(items, buildGovernanceEntryResponse(entry.EntryType, entry.TargetID, entry.Reason, entry.CreatedAt))
	}
	return items
}

func governanceCooldownSnapshot(cfg config.Config) governanceCommandCooldownResponse {
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

	return governanceCommandCooldownResponse{
		UserCommandRateLimit:  userRateLimit,
		GroupCommandRateLimit: groupRateLimit,
		CooldownReply:         cooldownReply,
	}
}

func buildGovernanceCommandPolicyEntries(snapshots []plugins.Snapshot, cfg config.Config) []governanceCommandPolicyEntryResponse {
	items := make([]governanceCommandPolicyEntryResponse, 0)
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
			items = append(items, governanceCommandPolicyEntryResponse{
				PluginID:            snapshot.PluginID,
				PluginName:          governancePluginDisplayName(snapshot),
				Command:             name,
				Aliases:             governanceNormalizedStrings(command.Aliases),
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
		return []governanceCommandPolicyEntryResponse{}
	}
	return items
}

func governancePluginDisplayName(snapshot plugins.Snapshot) string {
	if trimmed := strings.TrimSpace(snapshot.Name); trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(snapshot.PluginID)
}

func governanceNormalizedStrings(values []string) []string {
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
