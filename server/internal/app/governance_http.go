package app

import (
	"net/http"
	"sort"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

type governanceBlacklistEntryResponse struct {
	EntryType string `json:"entry_type"`
	TargetID  string `json:"target_id"`
	Reason    string `json:"reason"`
	CreatedAt string `json:"created_at"`
}

type governanceBlacklistResponse struct {
	UserEntries  []governanceBlacklistEntryResponse `json:"user_entries"`
	GroupEntries []governanceBlacklistEntryResponse `json:"group_entries"`
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

func (h *governanceHTTPHandlers) handleGovernanceBlacklist() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h == nil || h.blacklistRepo == nil {
			writeAuthJSON(w, http.StatusOK, governanceBlacklistResponse{
				UserEntries:  []governanceBlacklistEntryResponse{},
				GroupEntries: []governanceBlacklistEntryResponse{},
			})
			return
		}

		userEntries, err := h.blacklistRepo.List(r.Context(), "user")
		if err != nil {
			writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}
		groupEntries, err := h.blacklistRepo.List(r.Context(), "group")
		if err != nil {
			writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}

		writeAuthJSON(w, http.StatusOK, governanceBlacklistResponse{
			UserEntries:  buildGovernanceBlacklistEntries(userEntries),
			GroupEntries: buildGovernanceBlacklistEntries(groupEntries),
		})
	}
}

func (h *governanceHTTPHandlers) handleGovernanceCommandPolicy() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg := config.Config{}
		if h != nil && h.state != nil {
			cfg = h.state.Config
		}

		var snapshots []plugins.Snapshot
		if h != nil && h.plugins != nil {
			snapshots = h.plugins.List()
		}

		writeAuthJSON(w, http.StatusOK, governanceCommandPolicyResponse{
			DefaultLevel: commandPermissionDefaultLevel(cfg),
			Cooldown:     governanceCooldownSnapshot(cfg),
			Commands:     buildGovernanceCommandPolicyEntries(snapshots, cfg),
		})
	}
}

func buildGovernanceBlacklistEntries(entries []permission.BlacklistEntry) []governanceBlacklistEntryResponse {
	if len(entries) == 0 {
		return []governanceBlacklistEntryResponse{}
	}

	items := make([]governanceBlacklistEntryResponse, 0, len(entries))
	for _, entry := range entries {
		items = append(items, governanceBlacklistEntryResponse{
			EntryType: strings.TrimSpace(entry.EntryType),
			TargetID:  strings.TrimSpace(entry.TargetID),
			Reason:    strings.TrimSpace(entry.Reason),
			CreatedAt: strings.TrimSpace(entry.CreatedAt),
		})
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
