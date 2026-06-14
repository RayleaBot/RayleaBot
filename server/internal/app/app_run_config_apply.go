package app

import (
	"time"

	internalconfig "github.com/RayleaBot/RayleaBot/server/internal/config"
	managementhttp "github.com/RayleaBot/RayleaBot/server/internal/management/http"
	"github.com/RayleaBot/RayleaBot/server/internal/render"
)

func normalizeConfigApplyEffects(e *managementhttp.ConfigApplyEffects) {
	e.AppliedNow = normalizeConfigEffectPaths(e.AppliedNow)
	e.ReloadedNow = normalizeConfigEffectPaths(e.ReloadedNow)
	e.RestartRequiredFields = normalizeConfigEffectPaths(e.RestartRequiredFields)
}

func (s *configHTTPServiceImpl) ApplyHotReloadableFields(newCfg internalconfig.Config) managementhttp.ConfigApplyEffects {
	oldCfg := s.state.Config
	effects := classifyConfigApplyEffects(oldCfg, newCfg)
	oneBotHotChanged := len(effects.ReloadedNow) > 0

	if newCfg.Log.Level != oldCfg.Log.Level {
		if s.state.LogLevel != nil {
			if err := s.state.LogLevel.SetLevel(newCfg.Log.Level); err == nil {
				s.state.Logger.Info("log level changed",
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
		s.renderer.UpdateRuntimeConfig(render.RuntimeConfig{
			QueueMaxLength:     newCfg.Render.QueueMaxLength,
			QueueWaitTimeout:   time.Duration(newCfg.Render.QueueWaitTimeoutSeconds) * time.Second,
			RenderTimeout:      time.Duration(newCfg.Render.TimeoutSeconds) * time.Second,
			FooterTemplate:     newCfg.Render.FooterTemplate,
			DefaultOutput:      newCfg.Render.DefaultOutput,
			DeviceScalePercent: newCfg.Render.DeviceScalePercent,
		})
	}

	s.state.Config = newCfg
	if s.eventIngress != nil {
		s.eventIngress.UpdateConfig(newCfg)
	}
	if oneBotHotChanged && s.protocol != nil {
		if err := s.protocol.ApplyConfigReload(newCfg); err != nil {
			effects.RestartRequiredFields = append(effects.RestartRequiredFields, effects.ReloadedNow...)
			effects.ReloadedNow = effects.ReloadedNow[:0]
			if err != managementhttp.ErrProtocolStopped {
				s.state.Logger.Warn("adapter shell hot reload failed",
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
