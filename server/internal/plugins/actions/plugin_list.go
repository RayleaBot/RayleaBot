package actions

import (
	"context"
	"slices"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
)

func init() {
	register(Metadata{
		Action:             "plugin.list",
		Capability:         "plugin.list",
		RequestSchema:      "plugin-protocol.action_plugin_list",
		ResponseSchema:     "plugin-protocol.local_action_result",
		RequiredPermission: "declared capability",
		AuditFields:        []string{"plugin_id", "visibility"},
		ErrorCodes:         commonErrorCodes(),
	}, func(deps Deps) ActionHandler {
		return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
			return executePluginList(ctx, deps, req)
		}
	})
}

func executePluginList(ctx context.Context, deps Deps, req ActionRequest) (map[string]any, error) {
	if deps.Capabilities == nil || !deps.Capabilities.CapabilityDeclared(ctx, req.PluginID, "plugin.list") {
		return nil, &pluginruntime.Error{
			Code:    "plugin.capability_violation",
			Message: "plugin.list capability is not declared",
		}
	}

	snapshots := deps.Capabilities.ListPluginSnapshots()
	conflicts := plugins.DetectCommandConflicts(snapshots)
	items := make([]map[string]any, 0, len(snapshots))
	for _, snapshot := range snapshots {
		view := plugins.BuildSummaryView(snapshot, conflicts[snapshot.PluginID])
		commands := view.Commands
		help := view.Help
		if req.Action.PluginListVisibility == "caller" {
			cfg := currentConfig(deps)
			commands = pluginListVisibleCommandsForCaller(commands, cfg, req.ParentEvent)
			help = pluginListVisibleHelpForCaller(view.Help, view.Commands, commands, cfg, req.ParentEvent)
		}
		item := map[string]any{
			"id":                view.ID,
			"name":              view.Name,
			"description":       view.Description,
			"role":              view.Role,
			"state":             view.State,
			"commands":          pluginListBuildCommands(commands),
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

func pluginListVisibleCommandsForCaller(commands []plugins.CommandView, cfg config.Config, event pluginruntime.Event) []plugins.CommandView {
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

func pluginListVisibleHelpForCaller(help *plugins.HelpView, allCommands []plugins.CommandView, visibleCommands []plugins.CommandView, cfg config.Config, event pluginruntime.Event) *plugins.HelpView {
	if help == nil {
		return nil
	}

	visibleTokens := pluginListCommandTokenSet(visibleCommands)
	allTokens := pluginListCommandTokenSet(allCommands)
	callerRank := pluginListCallerPermissionRank(cfg, event)
	filtered := &plugins.HelpView{
		Title:   help.Title,
		Summary: help.Summary,
	}
	for _, group := range help.Groups {
		filteredGroup := plugins.HelpGroupView{Title: group.Title}
		for _, item := range group.Items {
			commandToken := strings.ToLower(strings.TrimSpace(item.Command))
			if commandToken != "" {
				if _, commandExists := allTokens[commandToken]; !commandExists {
					continue
				}
				if _, commandVisible := visibleTokens[commandToken]; !commandVisible {
					continue
				}
				filteredGroup.Items = append(filteredGroup.Items, item)
				continue
			}

			level := pluginListEffectiveHelpPermission(item.Permission)
			if callerRank >= pluginListPermissionRank(level) {
				filteredGroup.Items = append(filteredGroup.Items, item)
			}
		}
		if len(filteredGroup.Items) > 0 {
			filtered.Groups = append(filtered.Groups, filteredGroup)
		}
	}
	if filtered.Title == "" && filtered.Summary == "" && len(filtered.Groups) == 0 {
		return nil
	}
	if len(filtered.Groups) == 0 {
		return nil
	}
	return filtered
}

func pluginListCommandTokenSet(commands []plugins.CommandView) map[string]struct{} {
	tokens := make(map[string]struct{})
	for _, command := range commands {
		addPluginListCommandToken(tokens, command.Name)
		addPluginListCommandToken(tokens, command.DeclarationID)
		for _, alias := range command.Aliases {
			addPluginListCommandToken(tokens, alias)
		}
	}
	return tokens
}

func addPluginListCommandToken(tokens map[string]struct{}, value string) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return
	}
	tokens[value] = struct{}{}
}

func pluginListCallerPermissionRank(cfg config.Config, event pluginruntime.Event) int {
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

func pluginListBuildCommands(commands []plugins.CommandView) []map[string]any {
	if len(commands) == 0 {
		return []map[string]any{}
	}

	items := make([]map[string]any, 0, len(commands))
	for _, command := range commands {
		item := map[string]any{
			"name":           command.Name,
			"command_source": command.CommandSource,
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

func pluginListBuildHelp(help *plugins.HelpView) map[string]any {
	result := map[string]any{}
	if help.Title != "" {
		result["title"] = help.Title
	}
	if help.Summary != "" {
		result["summary"] = help.Summary
	}
	groups := make([]map[string]any, 0, len(help.Groups))
	for _, group := range help.Groups {
		items := make([]map[string]any, 0, len(group.Items))
		for _, item := range group.Items {
			entry := map[string]any{
				"title": item.Title,
			}
			if item.Description != "" {
				entry["description"] = item.Description
			}
			if item.Usage != "" {
				entry["usage"] = item.Usage
			}
			if item.Command != "" {
				entry["command"] = item.Command
			}
			if item.Permission != "" {
				entry["permission"] = item.Permission
			}
			items = append(items, entry)
		}
		if len(items) == 0 {
			continue
		}
		groups = append(groups, map[string]any{
			"title": group.Title,
			"items": items,
		})
	}
	if len(groups) > 0 {
		result["groups"] = groups
	}
	return result
}