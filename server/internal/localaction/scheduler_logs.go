package localaction

import (
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
)

func (s *Service) pluginDisplayName(pluginID string) string {
	pluginID = strings.TrimSpace(pluginID)
	if pluginID == "" {
		return "未知插件"
	}
	if s != nil && s.grants != nil {
		for _, snapshot := range s.grants.ListPluginSnapshots() {
			if strings.TrimSpace(snapshot.PluginID) != pluginID {
				continue
			}
			if name := strings.TrimSpace(snapshot.Name); name != "" {
				return name
			}
			return pluginID
		}
	}
	return pluginID
}

func schedulerLogLabel(values ...string) string { return scheduler.DisplayLabel(values...) }

func schedulerLogMessage(pluginName, taskName, logLabel, status string) string {
	return scheduler.DisplayMessage(pluginName, taskName, logLabel, status)
}
