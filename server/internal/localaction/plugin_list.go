package localaction

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

func (s *Service) executePluginList(ctx context.Context, pluginID string) (map[string]any, error) {
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
		items = append(items, map[string]any{
			"id":                 view.ID,
			"name":               view.Name,
			"description":        view.Description,
			"role":               view.Role,
			"registration_state": view.RegistrationState,
			"desired_state":      view.DesiredState,
			"runtime_state":      view.RuntimeState,
			"display_state":      view.DisplayState,
			"commands":           buildPluginListCommands(view.Commands),
			"command_conflicts":  append([]string(nil), view.CommandConflicts...),
		})
	}

	return map[string]any{
		"items": items,
	}, nil
}

func buildPluginListCommands(commands []plugins.CommandView) []map[string]any {
	if len(commands) == 0 {
		return []map[string]any{}
	}

	items := make([]map[string]any, 0, len(commands))
	for _, command := range commands {
		item := map[string]any{
			"name": command.Name,
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
		items = append(items, item)
	}
	return items
}
