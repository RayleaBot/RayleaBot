package menu

import (
	"slices"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

func builtinMenuCommandTokenSet(commands []plugins.CommandView) map[string]struct{} {
	tokens := make(map[string]struct{})
	for _, command := range commands {
		addBuiltinMenuCommandToken(tokens, command.Name)
		addBuiltinMenuCommandToken(tokens, command.DeclarationID)
		for _, alias := range command.Aliases {
			addBuiltinMenuCommandToken(tokens, alias)
		}
	}
	return tokens
}

func addBuiltinMenuCommandToken(tokens map[string]struct{}, value string) {
	value = normalizeMenuLookup(value)
	if value == "" {
		return
	}
	tokens[value] = struct{}{}
}

func builtinMenuCommandPermissionSet(commands []plugins.CommandView, cfg config.Config) map[string]string {
	permissions := make(map[string]string)
	for _, command := range commands {
		level := builtinMenuEffectiveCommandPermission(command.Permission, cfg)
		setBuiltinMenuCommandPermission(permissions, command.Name, level)
		setBuiltinMenuCommandPermission(permissions, command.DeclarationID, level)
		for _, alias := range command.Aliases {
			setBuiltinMenuCommandPermission(permissions, alias, level)
		}
	}
	return permissions
}

func setBuiltinMenuCommandPermission(permissions map[string]string, value string, level string) {
	value = normalizeMenuLookup(value)
	if value == "" {
		return
	}
	permissions[value] = level
}

func builtinCommandUsageArgs(commandName string, usage string) string {
	commandName = strings.TrimSpace(commandName)
	usage = strings.TrimSpace(usage)
	if commandName == "" || usage == "" {
		return ""
	}
	if strings.HasPrefix(usage, "/") || strings.HasPrefix(usage, "#") || strings.HasPrefix(usage, "*") {
		usage = strings.TrimSpace(usage[1:])
	}
	if usage == commandName {
		return ""
	}
	if strings.HasPrefix(usage, commandName) {
		return strings.TrimSpace(strings.TrimPrefix(usage, commandName))
	}
	return ""
}

func builtinMenuCallerPermissionRank(cfg config.Config, event runtime.Event) int {
	actorID := ""
	actorRole := ""
	if event.Actor != nil {
		actorID = strings.TrimSpace(event.Actor.ID)
		actorRole = strings.TrimSpace(event.Actor.Role)
	}
	if actorID != "" && slices.Contains(builtinMenuSuperAdmins(cfg), actorID) {
		return builtinMenuPermissionRank("super_admin")
	}
	switch actorRole {
	case "owner", "admin":
		return builtinMenuPermissionRank("group_admin")
	default:
		return builtinMenuPermissionRank("everyone")
	}
}

func builtinMenuSuperAdmins(cfg config.Config) []string {
	return cfg.Admin.SuperAdmins
}

func builtinMenuEffectiveCommandPermission(permissionLevel string, cfg config.Config) string {
	switch strings.TrimSpace(permissionLevel) {
	case "super_admin", "group_admin", "everyone":
		return strings.TrimSpace(permissionLevel)
	case "":
		return builtinMenuDefaultPermission(cfg)
	default:
		return "everyone"
	}
}

func builtinMenuEffectiveHelpPermission(permissionLevel string) string {
	switch strings.TrimSpace(permissionLevel) {
	case "super_admin", "group_admin", "everyone":
		return strings.TrimSpace(permissionLevel)
	default:
		return "everyone"
	}
}

func builtinMenuEffectiveHelpItemPermission(item plugins.HelpItemView, commandPermissions map[string]string) string {
	if strings.TrimSpace(item.Permission) != "" {
		return builtinMenuEffectiveHelpPermission(item.Permission)
	}
	if level, ok := commandPermissions[normalizeMenuLookup(item.Command)]; ok {
		return level
	}
	return builtinMenuEffectiveHelpPermission(item.Permission)
}

func builtinMenuDefaultPermission(cfg config.Config) string {
	defaultLevel := strings.TrimSpace(cfg.Permission.DefaultLevel)
	switch defaultLevel {
	case "super_admin", "group_admin", "everyone":
		return defaultLevel
	default:
		return "everyone"
	}
}

func builtinMenuPermissionRank(level string) int {
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

func builtinMenuPermissionLabel(level string) string {
	switch level {
	case "super_admin":
		return "超级管理员"
	case "group_admin":
		return "群管理员"
	case "everyone":
		return "所有人"
	default:
		return ""
	}
}
