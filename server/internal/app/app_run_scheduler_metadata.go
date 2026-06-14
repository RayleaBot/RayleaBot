package app

import (
	"strings"

	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/service"
)

func (s *systemService) SchedulerPluginName(pluginID string) string {
	pluginName := strings.TrimSpace(pluginID)
	if s != nil && s.plugins != nil {
		if snapshot, ok := s.plugins.Get(pluginID); ok {
			pluginName = pluginservice.SchedulerPluginDisplayName(snapshot, pluginID)
		}
	}
	if pluginName == "" {
		return "未知插件"
	}
	return pluginName
}

func (s *systemService) SchedulerTimezone() string {
	if s != nil && s.state != nil {
		if tz := strings.TrimSpace(s.state.Config.Scheduler.Timezone); tz != "" {
			return tz
		}
	}
	return "UTC"
}
