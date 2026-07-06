package configruntime

import (
	"context"
	"errors"
	"maps"
	"reflect"
	"slices"
	"time"

	internalconfig "github.com/RayleaBot/RayleaBot/server/internal/config"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
)

var ErrProtocolStopped = errors.New("protocol adapter stopped")

type ApplyEffects struct {
	AppliedNow            []string `json:"applied_now"`
	ReloadedNow           []string `json:"reloaded_now"`
	RestartRequiredFields []string `json:"restart_required_fields"`
}

func NewApplyEffects() ApplyEffects {
	return ApplyEffects{
		AppliedNow:            []string{},
		ReloadedNow:           []string{},
		RestartRequiredFields: []string{},
	}
}

func (e ApplyEffects) RestartRequired() bool {
	return len(e.RestartRequiredFields) > 0
}

type Document struct {
	Config         map[string]any
	RedactedFields []string
}

type UpdateResult struct {
	Document        Document
	RestartRequired bool
	ApplyEffects    ApplyEffects
}

func (s *Service) CurrentConfigDocument() Document {
	document, redactedFields := sanitizeConfigDocument(ConfigDocumentFromTyped(s.config()))
	return Document{
		Config:         document,
		RedactedFields: redactedFields,
	}
}

func (s *Service) UpdateConfigDocument(ctx context.Context, request map[string]any) (UpdateResult, error) {
	s.updateMu.Lock()
	defer s.updateMu.Unlock()

	summary := s.summary()
	request = restoreRedactedConfigSecrets(request, ConfigDocumentFromTyped(s.config()))
	if _, _, _, err := internalconfig.NormalizeDocument(summary.ConfigPath, summary.SchemaPath, request); err != nil {
		return UpdateResult{}, err
	}
	storedRequest, err := StoreConfigSecrets(ctx, s.secrets, request)
	if err != nil {
		return UpdateResult{}, err
	}
	newCfg, newSummary, err := internalconfig.SaveDocument(summary.ConfigPath, summary.SchemaPath, storedRequest)
	if err != nil {
		return UpdateResult{}, err
	}
	newCfg, err = ResolveConfigSecretRefs(ctx, s.secrets, newCfg)
	if err != nil {
		return UpdateResult{}, err
	}

	applyEffects := s.applyHotReloadableFieldsLocked(newCfg)
	if s.setSummary != nil {
		s.setSummary(newSummary)
	}

	document, redactedFields := sanitizeConfigDocument(ConfigDocumentFromTyped(newCfg))
	return UpdateResult{
		Document: Document{
			Config:         document,
			RedactedFields: redactedFields,
		},
		RestartRequired: applyEffects.RestartRequired(),
		ApplyEffects:    applyEffects,
	}, nil
}

func ConfigDocumentFromTyped(cfg internalconfig.Config) map[string]any {
	return internalconfig.CanonicalDocumentFromTyped(cfg)
}

func (s *Service) config() internalconfig.Config {
	if s == nil || s.currentConfig == nil {
		return internalconfig.Config{}
	}
	return s.currentConfig()
}

func (s *Service) summary() internalconfig.Summary {
	if s == nil || s.currentSummary == nil {
		return internalconfig.Summary{}
	}
	return s.currentSummary()
}

type ConfigApplyPolicy string

const (
	ConfigApplyPolicyHotReload       ConfigApplyPolicy = "hot_reload"
	ConfigApplyPolicyAdapterReload   ConfigApplyPolicy = "adapter_reload"
	ConfigApplyPolicyRestartRequired ConfigApplyPolicy = "restart_required"
	ConfigApplyPolicySecretOnly      ConfigApplyPolicy = "secret_only"
	ConfigApplyPolicyReadOnly        ConfigApplyPolicy = "read_only"
)

func ClassifyApplyEffects(oldCfg internalconfig.Config, newCfg internalconfig.Config) ApplyEffects {
	effects := NewApplyEffects()

	for _, path := range diffConfigDocumentPaths(ConfigDocumentFromTyped(oldCfg), ConfigDocumentFromTyped(newCfg)) {
		policy, ok := ConfigApplyPolicyForPath(path)
		switch {
		case !ok:
			effects.RestartRequiredFields = append(effects.RestartRequiredFields, path)
		case policy == ConfigApplyPolicyAdapterReload || policy == ConfigApplyPolicySecretOnly:
			effects.ReloadedNow = append(effects.ReloadedNow, path)
		case policy == ConfigApplyPolicyRestartRequired || policy == ConfigApplyPolicyReadOnly:
			effects.RestartRequiredFields = append(effects.RestartRequiredFields, path)
		default:
			effects.AppliedNow = append(effects.AppliedNow, path)
		}
	}

	normalizeConfigApplyEffects(&effects)
	return effects
}

func diffConfigDocumentPaths(current, next map[string]any) []string {
	paths := make([]string, 0)
	collectConfigPathChanges("", current, next, &paths)
	return normalizeConfigEffectPaths(paths)
}

func collectConfigPathChanges(prefix string, current, next any, paths *[]string) {
	currentMap, currentIsMap := current.(map[string]any)
	nextMap, nextIsMap := next.(map[string]any)
	if currentIsMap && nextIsMap {
		keys := make(map[string]struct{}, configDiffKeyCapacity(len(currentMap), len(nextMap)))
		for key := range currentMap {
			keys[key] = struct{}{}
		}
		for key := range nextMap {
			keys[key] = struct{}{}
		}
		sortedKeys := slices.Collect(maps.Keys(keys))
		slices.Sort(sortedKeys)
		for _, key := range sortedKeys {
			collectConfigPathChanges(joinConfigPath(prefix, key), currentMap[key], nextMap[key], paths)
		}
		return
	}

	if reflect.DeepEqual(current, next) || prefix == "" {
		return
	}

	*paths = append(*paths, prefix)
}

func configDiffKeyCapacity(currentCount int, nextCount int) int {
	maxInt := int(^uint(0) >> 1)
	if currentCount < 0 || nextCount < 0 || currentCount > maxInt-nextCount {
		return currentCount
	}
	return currentCount + nextCount
}

func joinConfigPath(prefix string, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

func normalizeConfigEffectPaths(paths []string) []string {
	if len(paths) == 0 {
		return []string{}
	}

	normalized := append([]string(nil), paths...)
	slices.Sort(normalized)
	return slices.Compact(normalized)
}

func normalizeConfigApplyEffects(e *ApplyEffects) {
	e.AppliedNow = normalizeConfigEffectPaths(e.AppliedNow)
	e.ReloadedNow = normalizeConfigEffectPaths(e.ReloadedNow)
	e.RestartRequiredFields = normalizeConfigEffectPaths(e.RestartRequiredFields)
}

func (s *Service) ApplyHotReloadableFields(newCfg internalconfig.Config) ApplyEffects {
	s.updateMu.Lock()
	defer s.updateMu.Unlock()
	return s.applyHotReloadableFieldsLocked(newCfg)
}

func (s *Service) applyHotReloadableFieldsLocked(newCfg internalconfig.Config) ApplyEffects {
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
