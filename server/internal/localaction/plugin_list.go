package localaction

import (
	"context"

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
