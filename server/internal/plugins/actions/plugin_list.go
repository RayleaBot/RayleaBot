package actions

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions/pluginlist"
	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/action"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
)

func (s *Service) executePluginList(ctx context.Context, pluginID string, action runtimeaction.Action, parentEvent runtimeprotocol.Event) (map[string]any, error) {
	if s == nil || s.grants == nil || !s.grants.CapabilityGranted(ctx, pluginID, "plugin.list") {
		return nil, &runtimemanager.Error{
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
			cfg := s.config()
			commands = pluginlist.VisibleCommandsForCaller(commands, cfg, parentEvent)
			help = pluginlist.VisibleHelpForCaller(view.Help, view.Commands, commands, cfg, parentEvent)
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
			"commands":           pluginlist.BuildCommands(commands),
			"command_conflicts":  append([]string(nil), view.CommandConflicts...),
		}
		if help != nil {
			item["help"] = pluginlist.BuildHelp(help)
		}
		items = append(items, item)
	}

	return map[string]any{
		"items": items,
	}, nil
}
