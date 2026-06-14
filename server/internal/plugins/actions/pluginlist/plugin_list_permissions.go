package pluginlist

import (
	"slices"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
)

func pluginListCallerPermissionRank(cfg config.Config, event runtimeprotocol.Event) int {
	actorID := ""
	actorRole := ""
	if event.Actor != nil {
		actorID = strings.TrimSpace(event.Actor.ID)
		actorRole = strings.TrimSpace(event.Actor.Role)
	}
	if actorID != "" && slices.Contains(pluginListSuperAdmins(cfg), actorID) {
		return pluginListPermissionRank("super_admin")
	}
	switch actorRole {
	case "owner", "admin":
		return pluginListPermissionRank("group_admin")
	default:
		return pluginListPermissionRank("everyone")
	}
}

func pluginListSuperAdmins(cfg config.Config) []string {
	return cfg.Admin.SuperAdmins
}

func pluginListEffectiveCommandPermission(permissionLevel string, cfg config.Config) string {
	switch strings.TrimSpace(permissionLevel) {
	case "super_admin", "group_admin", "everyone":
		return strings.TrimSpace(permissionLevel)
	case "":
		return pluginListDefaultPermission(cfg)
	default:
		return "everyone"
	}
}

func pluginListEffectiveHelpPermission(permissionLevel string) string {
	switch strings.TrimSpace(permissionLevel) {
	case "super_admin", "group_admin", "everyone":
		return strings.TrimSpace(permissionLevel)
	default:
		return "everyone"
	}
}

func pluginListDefaultPermission(cfg config.Config) string {
	defaultLevel := strings.TrimSpace(cfg.Permission.DefaultLevel)
	switch defaultLevel {
	case "super_admin", "group_admin", "everyone":
		return defaultLevel
	default:
		return "everyone"
	}
}

func pluginListPermissionRank(level string) int {
	switch level {
	case "super_admin":
		return 3
	case "group_admin":
		return 2
	case "everyone":
		return 1
	default:
		return 1
	}
}
