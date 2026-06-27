package configruntime

import (
	"time"

	internalconfig "github.com/RayleaBot/RayleaBot/server/internal/config"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
)

func normalizeConfigApplyEffects(e *ApplyEffects) {
	e.AppliedNow = normalizeConfigEffectPaths(e.AppliedNow)
	e.ReloadedNow = normalizeConfigEffectPaths(e.ReloadedNow)
	e.RestartRequiredFields = normalizeConfigEffectPaths(e.RestartRequiredFields)
}

func (s *Service) ApplyHotReloadableFields(newCfg internalconfig.Config) ApplyEffects {
	oldCfg := s.config()
	effects := ClassifyApplyEffects(oldCfg, newCfg)
	oneBotHotChanged := len(effects.ReloadedNow) > 0

	if s.addRedactionValues != nil {
		s.addRedactionValues(configSecretValues(newCfg)...)
	}
	if newCfg.Log.Level != oldCfg.Log.Level {
		if s.logLevel != nil {
			if err := s.logLevel.SetLevel(newCfg.Log.Level); err == nil && s.logger != nil {
				s.logger.Info("日志级别已从 "+oldCfg.Log.Level+" 调整为 "+newCfg.Log.Level,
					"component", "config",
					"old_level", oldCfg.Log.Level,
					"new_level", newCfg.Log.Level,
				)
			}
		}
	}
	if newCfg.Log.RetentionDays != oldCfg.Log.RetentionDays && s.logs != nil {
		s.logs.SetRepository(s.logRepository, newCfg.Log.RetentionDays)
	}
	if newCfg.Log.RateLimitPerPlugin != oldCfg.Log.RateLimitPerPlugin && s.pluginLogLimiter != nil {
		s.pluginLogLimiter.ApplyConfig(newCfg)
	}
	if s.outboundLimiter != nil && (newCfg.Message.RateLimitPerPlugin != oldCfg.Message.RateLimitPerPlugin ||
		newCfg.Message.RateLimitPerTarget != oldCfg.Message.RateLimitPerTarget ||
		newCfg.Message.CircuitBreakerSeconds != oldCfg.Message.CircuitBreakerSeconds) {
		s.outboundLimiter.ApplyConfig(newCfg)
	}
	if s.renderer != nil && (newCfg.Render.TimeoutSeconds != oldCfg.Render.TimeoutSeconds ||
		newCfg.Render.QueueWaitTimeoutSeconds != oldCfg.Render.QueueWaitTimeoutSeconds ||
		newCfg.Render.QueueMaxLength != oldCfg.Render.QueueMaxLength ||
		newCfg.Render.FooterTemplate != oldCfg.Render.FooterTemplate ||
		newCfg.Render.DefaultOutput != oldCfg.Render.DefaultOutput ||
		newCfg.Render.DeviceScalePercent != oldCfg.Render.DeviceScalePercent) {
		s.renderer.UpdateRuntimeConfig(renderservice.RuntimeConfig{
			QueueMaxLength:     newCfg.Render.QueueMaxLength,
			QueueWaitTimeout:   time.Duration(newCfg.Render.QueueWaitTimeoutSeconds) * time.Second,
			RenderTimeout:      time.Duration(newCfg.Render.TimeoutSeconds) * time.Second,
			FooterTemplate:     newCfg.Render.FooterTemplate,
			DefaultOutput:      newCfg.Render.DefaultOutput,
			DeviceScalePercent: newCfg.Render.DeviceScalePercent,
		})
	}

	if s.setConfig != nil {
		s.setConfig(newCfg)
	}
	if s.eventIngress != nil {
		s.eventIngress.UpdateConfig(newCfg)
	}
	if oneBotHotChanged && s.protocol != nil {
		if err := s.protocol.ApplyConfigReload(newCfg); err != nil {
			effects.RestartRequiredFields = append(effects.RestartRequiredFields, effects.ReloadedNow...)
			effects.ReloadedNow = effects.ReloadedNow[:0]
			if err != ErrProtocolStopped && s.logger != nil {
				s.logger.Warn("OneBot 适配器热加载失败，需要重启相关配置",
					"component", "config",
					"err", err.Error(),
				)
			}
		}
	}
	if s.protocol != nil {
		s.protocol.PublishSnapshot()
	}

	normalizeConfigApplyEffects(&effects)
	return effects
}
