package governance

import (
	"context"
	"sort"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

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

func cooldownSnapshot(cfg config.Config) CommandCooldownResponse {
	userRateLimit := strings.TrimSpace(cfg.User.CommandRateLimit)
	groupRateLimit := strings.TrimSpace(cfg.Group.CommandRateLimit)
	cooldownReply := cfg.User.CooldownReply

	if userRateLimit == "" {
		userRateLimit = config.DefaultUserCommandRateLimit
	}
	if groupRateLimit == "" {
		groupRateLimit = config.DefaultGroupCommandRateLimit
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
	switch strings.TrimSpace(source) {
	case plugins.CommandSourceDynamic:
		return plugins.CommandSourceDynamic
	case plugins.CommandSourcePattern:
		return plugins.CommandSourcePattern
	default:
		return plugins.CommandSourceManifest
	}
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
