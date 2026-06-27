package manager

import (
	"strings"

	runtimespec "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/spec"
)

func runtimePluginLabel(spec runtimespec.Spec) string {
	pluginID := strings.TrimSpace(spec.PluginID)
	name := strings.TrimSpace(spec.PluginName)
	switch {
	case name != "" && pluginID != "" && name != pluginID:
		return name + "（" + pluginID + "）"
	case name != "":
		return name
	default:
		return pluginID
	}
}

func pluginIDLabel(pluginID string) string {
	pluginID = strings.TrimSpace(pluginID)
	if pluginID == "" {
		return "未知插件"
	}
	return pluginID
}
