package ws

import (
	managementevents "github.com/RayleaBot/RayleaBot/server/internal/management/events"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func pluginStateEventFrame(snapshot plugins.Snapshot, snapshots []plugins.Snapshot) managementevents.Frame {
	state, diagnosis := plugins.ProjectState(snapshot)
	return managementevents.NewReceivedFrame(managementevents.PluginStatePayload{
		PluginID:         snapshot.PluginID,
		State:            state,
		StateDiagnosis:   diagnosis,
		Commands:         pluginStateEventCommands(snapshot.Commands),
		CommandConflicts: pluginStateEventCommandConflicts(snapshot, snapshots),
	})
}

func pluginSnapshotsForConflicts(catalog interface{ List() []plugins.Snapshot }) []plugins.Snapshot {
	if catalog == nil {
		return nil
	}
	return catalog.List()
}

func pluginStateEventCommands(commands []plugins.Command) []managementevents.PluginCommandItem {
	if len(commands) == 0 {
		return []managementevents.PluginCommandItem{}
	}
	items := make([]managementevents.PluginCommandItem, 0, len(commands))
	for _, command := range commands {
		if command.Name == "" {
			continue
		}
		item := managementevents.PluginCommandItem{
			Name:          command.Name,
			Aliases:       append([]string(nil), command.Aliases...),
			Description:   command.Description,
			Usage:         command.Usage,
			Permission:    command.Permission,
			CommandSource: pluginEventCommandSource(command.CommandSource),
			DeclarationID: command.DeclarationID,
		}
		items = append(items, item)
	}
	if len(items) == 0 {
		return []managementevents.PluginCommandItem{}
	}
	return items
}

func pluginStateEventCommandConflicts(snapshot plugins.Snapshot, snapshots []plugins.Snapshot) []string {
	if len(snapshots) == 0 {
		snapshots = []plugins.Snapshot{snapshot}
	}
	conflicts := plugins.DetectCommandConflicts(snapshots)
	if len(conflicts[snapshot.PluginID]) == 0 {
		return []string{}
	}
	return conflicts[snapshot.PluginID]
}

func pluginEventCommandSource(source string) string {
	if source == plugins.CommandSourceDynamic {
		return plugins.CommandSourceDynamic
	}
	if source == plugins.CommandSourcePattern {
		return plugins.CommandSourcePattern
	}
	return plugins.CommandSourceManifest
}
