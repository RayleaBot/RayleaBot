package pluginlist

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/action"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
)

type CapabilityView interface {
	CapabilityDeclared(context.Context, string, string) bool
	ListPluginSnapshots() []plugins.Snapshot
}

type Request struct {
	PluginID      string
	Action        runtimeaction.Action
	ParentEvent   runtimeprotocol.Event
	Capabilities  CapabilityView
	CurrentConfig func() config.Config
}

func Execute(ctx context.Context, req Request) (map[string]any, error) {
	if req.Capabilities == nil || !req.Capabilities.CapabilityDeclared(ctx, req.PluginID, "plugin.list") {
		return nil, &runtimemanager.Error{
			Code:    "plugin.capability_violation",
			Message: "plugin.list capability is not declared",
		}
	}

	snapshots := req.Capabilities.ListPluginSnapshots()
	conflicts := plugins.DetectCommandConflicts(snapshots)
	items := make([]map[string]any, 0, len(snapshots))
	for _, snapshot := range snapshots {
		view := plugins.BuildSummaryView(snapshot, conflicts[snapshot.PluginID])
		commands := view.Commands
		help := view.Help
		if req.Action.PluginListVisibility == "caller" {
			cfg := currentConfig(req)
			commands = VisibleCommandsForCaller(commands, cfg, req.ParentEvent)
			help = VisibleHelpForCaller(view.Help, view.Commands, commands, cfg, req.ParentEvent)
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
			"commands":           BuildCommands(commands),
			"command_conflicts":  append([]string(nil), view.CommandConflicts...),
		}
		if help != nil {
			item["help"] = BuildHelp(help)
		}
		items = append(items, item)
	}

	return map[string]any{
		"items": items,
	}, nil
}

func currentConfig(req Request) config.Config {
	if req.CurrentConfig == nil {
		return config.Config{}
	}
	return req.CurrentConfig()
}
