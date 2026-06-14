package system

import (
	"strings"

	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/service"
)

func (s *Service) SchedulerPluginName(pluginID string) string {
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

func (s *Service) SchedulerTimezone() string {
	if s != nil {
		if tz := strings.TrimSpace(s.config().Scheduler.Timezone); tz != "" {
			return tz
		}
	}
	return "UTC"
}
