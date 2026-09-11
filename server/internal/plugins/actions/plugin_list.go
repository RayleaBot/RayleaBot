package actions

import (
	"context"
	"slices"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func pluginListRegistrar() registrar {
	return registrar{
		kind: "plugin.list",
		factory: func(deps Deps) ActionHandler {
			return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
				return executePluginList(ctx, deps, req)
			}
		},
	}
}

func executePluginList(ctx context.Context, deps Deps, req ActionRequest) (map[string]any, error) {
	if deps.Permissions == nil || !deps.Permissions.PermissionDeclared(ctx, req.PluginID, "plugin.list") {
		return nil, &plugins.Error{
			Code:    errorcodes.PluginPermissionDenied,
			Message: "plugin.list permission is not declared",
		}
	}

	snapshots := deps.Permissions.ListPluginSnapshots()
	conflicts := plugins.DetectCommandConflicts(snapshots)
	items := make([]map[string]any, 0, len(snapshots))
	for _, snapshot := range snapshots {
		view := plugins.BuildSummaryView(snapshot, conflicts[snapshot.PluginID])
		commands := view.Commands
		help := view.Help
		if req.Action.PluginListVisibility == "caller" {
			cfg := currentConfig(deps)
			commands = pluginListVisibleCommandsForCaller(commands, cfg, req.ParentEvent)
			help = pluginListVisibleHelpForCaller(view.Help, commands)
		}
		commandGroups := pluginListVisibleCommandGroups(view.CommandGroups, commands)
		item := map[string]any{
			"id":                view.ID,
			"name":              view.Name,
			"description":       view.Description,
			"role":              view.Role,
			"state":             view.State,
			"commands":          pluginListBuildCommands(commands),
			"command_groups":    pluginListBuildCommandGroups(commandGroups),
			"command_conflicts": append([]string(nil), view.CommandConflicts...),
		}
		if view.StateDiagnosis != nil {
			item["state_diagnosis"] = view.StateDiagnosis
		}
		if help != nil {
			item["help"] = pluginListBuildHelp(help)
		}
		items = append(items, item)
	}

	return map[string]any{
		"items": items,
	}, nil
}

func pluginListVisibleCommandsForCaller(commands []plugins.CommandView, cfg config.Config, event chatevent.Event) []plugins.CommandView {
	if len(commands) == 0 {
		return []plugins.CommandView{}
	}

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

func pluginListVisibleHelpForCaller(help *plugins.HelpView, visibleCommands []plugins.CommandView) *plugins.HelpView {
	if help == nil || len(visibleCommands) == 0 {
		return nil
	}
	return &plugins.HelpView{Title: help.Title, Summary: help.Summary}
}

func pluginListVisibleCommandGroups(groups []plugins.CommandGroup, commands []plugins.CommandView) []plugins.CommandGroup {
	visible := make(map[string]struct{}, len(commands))
	for _, command := range commands {
		visible[command.ID] = struct{}{}
	}
	result := make([]plugins.CommandGroup, 0, len(groups))
	for _, group := range groups {
		filtered := plugins.CommandGroup{ID: group.ID, Title: group.Title}
		for _, commandID := range group.Commands {
			if _, ok := visible[commandID]; ok {
				filtered.Commands = append(filtered.Commands, commandID)
			}
		}
		if len(filtered.Commands) > 0 {
			result = append(result, filtered)
		}
	}
	return result
}

func pluginListCallerPermissionRank(cfg config.Config, event chatevent.Event) int {
	actorID := ""
	actorRole := ""
	if event.Actor != nil {
		actorID = strings.TrimSpace(event.Actor.ID)
		actorRole = strings.TrimSpace(event.Actor.Role)
	}
	if actorID != "" && slices.Contains(cfg.Admin.SuperAdmins, actorID) {
		return pluginListPermissionRank("super_admin")
	}
	switch actorRole {
	case "owner", "admin":
		return pluginListPermissionRank("group_admin")
	default:
		return pluginListPermissionRank("everyone")
	}
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

func pluginListBuildCommands(commands []plugins.CommandView) []map[string]any {
	if len(commands) == 0 {
		return []map[string]any{}
	}

	items := make([]map[string]any, 0, len(commands))
	for _, command := range commands {
		item := map[string]any{
			"id":              command.ID,
			"name":            command.Name,
			"effective_names": plugins.EffectiveCommandNames(command.TriggerType, command.EffectiveName, command.Aliases),
			"description":     command.Description,
			"usage":           command.Usage,
			"permission":      command.Permission,
			"trigger":         pluginListBuildTrigger(command),
		}
		items = append(items, item)
	}
	return items
}

func pluginListBuildTrigger(command plugins.CommandView) map[string]any {
	trigger := map[string]any{"type": command.TriggerType}
	switch command.TriggerType {
	case "exact":
		trigger["names"] = append([]string(nil), command.TriggerNames...)
	case "pattern":
		trigger["pattern"] = command.MatchPattern
	case "setting":
		trigger["settings_key"] = command.SettingsKey
	}
	return trigger
}

func pluginListBuildCommandGroups(groups []plugins.CommandGroup) []map[string]any {
	items := make([]map[string]any, 0, len(groups))
	for _, group := range groups {
		items = append(items, map[string]any{
			"id": group.ID, "title": group.Title, "commands": append([]string(nil), group.Commands...),
		})
	}
	return items
}

func pluginListBuildHelp(help *plugins.HelpView) map[string]any {
	result := map[string]any{}
	if help.Title != "" {
		result["title"] = help.Title
	}
	if help.Summary != "" {
		result["summary"] = help.Summary
	}
	return result
}
