package plugins

// CommandEntry is the read-only command surface of one plugin whose commands
// are enabled. Catalogs publish a fresh slice whenever they change so
// per-message matching never clones snapshots.
type CommandEntry struct {
	PluginID string
	Commands []Command
}

// CommandEntries projects the enabled commands of snapshots in order.
func CommandEntries(snapshots []Snapshot) []CommandEntry {
	entries := make([]CommandEntry, 0, len(snapshots))
	for _, snapshot := range snapshots {
		if !snapshot.CommandsEnabled() || len(snapshot.Commands) == 0 {
			continue
		}
		entries = append(entries, CommandEntry{PluginID: snapshot.PluginID, Commands: append([]Command(nil), snapshot.Commands...)})
	}
	return entries
}
