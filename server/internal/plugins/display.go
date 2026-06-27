package plugins

import "strings"

func DisplayName(snapshot Snapshot) string {
	if name := strings.TrimSpace(snapshot.Name); name != "" {
		return name
	}
	return strings.TrimSpace(snapshot.PluginID)
}

func DisplayLabel(snapshot Snapshot) string {
	pluginID := strings.TrimSpace(snapshot.PluginID)
	name := strings.TrimSpace(snapshot.Name)
	switch {
	case name != "" && pluginID != "" && name != pluginID:
		return name + "（" + pluginID + "）"
	case name != "":
		return name
	default:
		return pluginID
	}
}
