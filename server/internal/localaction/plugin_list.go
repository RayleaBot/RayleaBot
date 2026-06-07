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
		help := view.Help
		if action.PluginListVisibility == "caller" {
			commands = s.visiblePluginListCommandsForCaller(commands, parentEvent)
			help = s.visiblePluginListHelpForCaller(view.Help, view.Commands, commands, parentEvent)
		}
		item := map[string]any{
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
		}
		if help != nil {
			item["help"] = buildPluginListHelp(help)
		}
		items = append(items, item)
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

func (s *Service) visiblePluginListHelpForCaller(help *plugins.HelpView, allCommands []plugins.CommandView, visibleCommands []plugins.CommandView, event runtime.Event) *plugins.HelpView {
	if help == nil {
		return nil
	}

	visibleTokens := pluginListCommandTokenSet(visibleCommands)
	allTokens := pluginListCommandTokenSet(allCommands)
	cfg := s.config()
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

func buildPluginListHelp(help *plugins.HelpView) map[string]any {
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
