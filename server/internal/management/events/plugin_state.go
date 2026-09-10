package events

import "github.com/RayleaBot/RayleaBot/server/internal/plugins"

func pluginStateEventFrame(snapshot plugins.Snapshot, snapshots []plugins.Snapshot) Frame {
	state, diagnosis := plugins.ProjectState(snapshot)
	return NewReceivedFrame(PluginStatePayload{
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

func pluginStateEventCommands(commands []plugins.Command) []PluginCommandItem {
	if len(commands) == 0 {
		return []PluginCommandItem{}
	}
	items := make([]PluginCommandItem, 0, len(commands))
	for _, command := range commands {
		if command.ID == "" || command.DisplayName == "" {
			continue
		}
		item := PluginCommandItem{
			ID:             command.ID,
			Name:           command.DisplayName,
			EffectiveNames: plugins.EffectiveCommandNames(command.TriggerType, command.Name, command.Aliases),
			Description:    command.Description,
			Usage:          command.Usage,
			Permission:     command.Permission,
			Trigger: PluginCommandTrigger{
				Type: command.TriggerType, Names: append([]string(nil), command.TriggerNames...),
				Pattern: command.MatchPattern, SettingsKey: command.SettingsKey,
			},
		}
		items = append(items, item)
	}
	if len(items) == 0 {
		return []PluginCommandItem{}
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
