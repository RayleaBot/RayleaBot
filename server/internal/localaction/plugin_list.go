package localaction

import (
	"context"
	"slices"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

func (s *Service) executePluginList(ctx context.Context, pluginID string, action runtime.Action, parentEvent runtime.Event) (map[string]any, error) {
	if s == nil || s.grants == nil || !s.grants.CapabilityGranted(ctx, pluginID, "plugin.list") {
		return nil, &runtime.Error{
			Code:    "permission.scope_violation",
			Message: "plugin.list capability is not granted",
		}
	}

	snapshots := s.grants.ListPluginSnapshots()
	conflicts := plugins.DetectCommandConflicts(snapshots)
	items := make([]map[string]any, 0, len(snapshots))
	for _, snapshot := range snapshots {
		view := plugins.BuildSummaryView(snapshot, conflicts[snapshot.PluginID])
		commands := view.Commands
		if action.PluginListVisibility == "caller" {
			commands = s.visiblePluginListCommandsForCaller(commands, parentEvent)
		}
		items = append(items, map[string]any{
			"id":                 view.ID,
			"name":               view.Name,
			"description":        view.Description,
			"role":               view.Role,
			"registration_state": view.RegistrationState,
			"desired_state":      view.DesiredState,
			"runtime_state":      view.RuntimeState,
			"display_state":      view.DisplayState,
			"commands":           buildPluginListCommands(commands),
			"command_conflicts":  append([]string(nil), view.CommandConflicts...),
		})
	}

	return map[string]any{
		"items": items,
	}, nil
}

func (s *Service) visiblePluginListCommandsForCaller(commands []plugins.CommandView, event runtime.Event) []plugins.CommandView {
	if len(commands) == 0 {
		return []plugins.CommandView{}
	}

	cfg := s.config()
	callerRank := pluginListCallerPermissionRank(cfg, event)
	visible := make([]plugins.CommandView, 0, len(commands))
	for _, command := range commands {
		level := pluginListEffectiveCommandPermission(command.Permission, cfg)
		if callerRank >= pluginListPermissionRank(level) {
			visible = append(visible, command)
		}
	}
	return visible
}

func buildPluginListCommands(commands []plugins.CommandView) []map[string]any {
	if len(commands) == 0 {
		return []map[string]any{}
	}

	items := make([]map[string]any, 0, len(commands))
	for _, command := range commands {
		item := map[string]any{
			"name":           command.Name,
			"command_source": commandSourceOrDefault(command.CommandSource),
		}
		if len(command.Aliases) > 0 {
			item["aliases"] = append([]string(nil), command.Aliases...)
		}
		if command.Description != "" {
			item["description"] = command.Description
		}
		if command.Usage != "" {
			item["usage"] = command.Usage
		}
		if command.Permission != "" {
			item["permission"] = command.Permission
		}
		if command.DeclarationID != "" {
			item["declaration_id"] = command.DeclarationID
		}
		items = append(items, item)
	}
	return items
}

func commandSourceOrDefault(source string) string {
	if source == plugins.CommandSourceDynamic {
		return plugins.CommandSourceDynamic
	}
	return plugins.CommandSourceManifest
}

func pluginListCallerPermissionRank(cfg config.Config, event runtime.Event) int {
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
	if len(cfg.Admin.SuperAdmins) > 0 {
		return cfg.Admin.SuperAdmins
	}
	return cfg.Auth.SuperAdmins
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

func pluginListDefaultPermission(cfg config.Config) string {
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
