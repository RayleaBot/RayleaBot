package app

import (
	"maps"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"time"

	internalconfig "github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/render"
)

const redactedConfigValue = "__REDACTED__"

type configResponse struct {
	Config         map[string]any `json:"config"`
	RedactedFields []string       `json:"redacted_fields,omitempty"`
}

type configUpdateResponse struct {
	Config          map[string]any     `json:"config"`
	RedactedFields  []string           `json:"redacted_fields,omitempty"`
	RestartRequired bool               `json:"restart_required"`
	ApplyEffects    configApplyEffects `json:"apply_effects"`
}

type configApplyEffects struct {
	AppliedNow            []string `json:"applied_now"`
	ReloadedNow           []string `json:"reloaded_now"`
	RestartRequiredFields []string `json:"restart_required_fields"`
}

func newConfigApplyEffects() configApplyEffects {
	return configApplyEffects{
		AppliedNow:            []string{},
		ReloadedNow:           []string{},
		RestartRequiredFields: []string{},
	}
}

func (e configApplyEffects) restartRequired() bool {
	return len(e.RestartRequiredFields) > 0
}

func (e *configApplyEffects) normalize() {
	e.AppliedNow = normalizeConfigEffectPaths(e.AppliedNow)
	e.ReloadedNow = normalizeConfigEffectPaths(e.ReloadedNow)
	e.RestartRequiredFields = normalizeConfigEffectPaths(e.RestartRequiredFields)
}

func (h *configHTTPHandlers) handleConfigGet() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		document, redactedFields := sanitizeConfigDocument(configDocumentFromTyped(h.state.Config))
		writeAuthJSON(w, http.StatusOK, configResponse{
			Config:         document,
			RedactedFields: redactedFields,
		})
	}
}

func (h *configHTTPHandlers) handleConfigPut() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request map[string]any
		if err := decodeStrictJSON(w, r, &request, maxManagementJSONBodyBytes); err != nil {
			writeAppError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}

		resolved := resolveRedactedConfigValues(request, h.state.Config)
		newCfg, newSummary, err := internalconfig.SaveDocument(h.state.Summary.ConfigPath, h.state.Summary.SchemaPath, resolved)
		if err != nil {
			writeAppError(w, r, http.StatusBadRequest, "platform.invalid_config", "配置校验失败", "errors.platform.invalid_config", nil)
			return
		}

		applyEffects := h.applyHotReloadableFields(newCfg)
		h.state.Summary = newSummary

		document, redactedFields := sanitizeConfigDocument(resolved)
		writeAuthJSON(w, http.StatusOK, configUpdateResponse{
			Config:          document,
			RedactedFields:  redactedFields,
			RestartRequired: applyEffects.restartRequired(),
			ApplyEffects:    applyEffects,
		})
	}
}

// applyHotReloadableFields compares the new config with the current config,
// applies fields that can take effect immediately, and reports how each
// changed canonical config path is applied.
func (h *configHTTPHandlers) applyHotReloadableFields(newCfg internalconfig.Config) configApplyEffects {
	oldCfg := h.state.Config
	effects := classifyConfigApplyEffects(oldCfg, newCfg)
	oneBotHotChanged := len(effects.ReloadedNow) > 0

	// logging.level — immediate effect via LevelController.
	if newCfg.Logging.Level != oldCfg.Logging.Level {
		if h.state.LogLevel != nil {
			if err := h.state.LogLevel.SetLevel(newCfg.Logging.Level); err == nil {
				h.state.Logger.Info("log level changed",
					"component", "config",
					"old_level", oldCfg.Logging.Level,
					"new_level", newCfg.Logging.Level,
				)
			}
		}
	}
	if newCfg.Logging.RetentionDays != oldCfg.Logging.RetentionDays && h.logs != nil {
		h.logs.SetRepository(h.logRepository, newCfg.Logging.RetentionDays)
	}
	if newCfg.Logging.RateLimitPerPlugin != oldCfg.Logging.RateLimitPerPlugin && h.pluginLogLimiter != nil {
		h.pluginLogLimiter.SetLimit(parsePluginLogRateLimit(newCfg))
	}
	if h.renderer != nil && (newCfg.Render.TimeoutSeconds != oldCfg.Render.TimeoutSeconds ||
		newCfg.Render.QueueWaitTimeoutSeconds != oldCfg.Render.QueueWaitTimeoutSeconds ||
		newCfg.Render.QueueMaxLength != oldCfg.Render.QueueMaxLength) {
		h.renderer.UpdateRuntimeConfig(render.RuntimeConfig{
			QueueMaxLength:   newCfg.Render.QueueMaxLength,
			QueueWaitTimeout: time.Duration(newCfg.Render.QueueWaitTimeoutSeconds) * time.Second,
			RenderTimeout:    time.Duration(newCfg.Render.TimeoutSeconds) * time.Second,
		})
	}

	// Update in-memory config to reflect the saved state.
	h.state.Config = newCfg
	if h.eventIngress != nil {
		h.eventIngress.UpdateConfig(newCfg)
	}
	if oneBotHotChanged && h.protocol != nil && h.protocol.adapter != nil {
		if err := h.protocol.adapter.Reload(newCfg.OneBot); err != nil {
			effects.RestartRequiredFields = append(effects.RestartRequiredFields, effects.ReloadedNow...)
			effects.ReloadedNow = effects.ReloadedNow[:0]
			h.state.Logger.Warn("adapter shell hot reload failed",
				"component", "config",
				"err", err.Error(),
			)
		}
	}
	if h.protocol != nil {
		h.protocol.PublishSnapshot()
	}

	effects.normalize()
	return effects
}

func classifyConfigApplyEffects(oldCfg internalconfig.Config, newCfg internalconfig.Config) configApplyEffects {
	effects := newConfigApplyEffects()

	for _, path := range diffConfigDocumentPaths(configDocumentFromTyped(oldCfg), configDocumentFromTyped(newCfg)) {
		switch {
		case isConfigReloadPath(path):
			effects.ReloadedNow = append(effects.ReloadedNow, path)
		case isConfigRestartPath(path):
			effects.RestartRequiredFields = append(effects.RestartRequiredFields, path)
		default:
			effects.AppliedNow = append(effects.AppliedNow, path)
		}
	}

	effects.normalize()
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
		keys := make(map[string]struct{}, len(currentMap)+len(nextMap))
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

func joinConfigPath(prefix string, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

func isConfigReloadPath(path string) bool {
	return strings.HasPrefix(path, "onebot.") || strings.HasPrefix(path, "adapter.")
}

func isConfigRestartPath(path string) bool {
	switch {
	case strings.HasPrefix(path, "server."):
		return true
	case strings.HasPrefix(path, "database."):
		return true
	case strings.HasPrefix(path, "web."):
		return true
	}

	switch path {
	case "admin.session_ttl_days",
		"admin.sliding_renewal",
		"admin.max_sessions",
		"render.worker_count",
		"render.browser_path",
		"render.browser_args":
		return true
	default:
		return false
	}
}

func normalizeConfigEffectPaths(paths []string) []string {
	if len(paths) == 0 {
		return []string{}
	}

	normalized := append([]string(nil), paths...)
	slices.Sort(normalized)
	return slices.Compact(normalized)
}

func configDocumentFromTyped(cfg internalconfig.Config) map[string]any {
	return internalconfig.CanonicalDocumentFromTyped(cfg)
}

func sanitizeConfigDocument(document map[string]any) (map[string]any, []string) {
	cloned := internalconfig.CloneDocument(document)
	if cloned == nil {
		return nil, nil
	}

	redactedFields := make([]string, 0, 1)
	onebotSection, ok := cloned["onebot"].(map[string]any)
	if !ok {
		return cloned, redactedFields
	}

	accessToken, ok := onebotSection["access_token"].(string)
	if ok && accessToken != "" {
		onebotSection["access_token"] = redactedConfigValue
		redactedFields = append(redactedFields, "onebot.access_token")
	}

	return cloned, redactedFields
}

func resolveRedactedConfigValues(document map[string]any, current internalconfig.Config) map[string]any {
	cloned := internalconfig.CloneDocument(document)
	if cloned == nil {
		return nil
	}

	onebotSection, ok := cloned["onebot"].(map[string]any)
	if !ok {
		return cloned
	}

	accessToken, ok := onebotSection["access_token"].(string)
	if ok && accessToken == redactedConfigValue {
		onebotSection["access_token"] = current.OneBot.AccessToken
	}

	return cloned
}

func oneBotHotReloadChanged(oldCfg internalconfig.Config, newCfg internalconfig.Config) bool {
	return newCfg.OneBot.Provider != oldCfg.OneBot.Provider ||
		newCfg.OneBot.AccessToken != oldCfg.OneBot.AccessToken ||
		newCfg.OneBot.ReverseWS != oldCfg.OneBot.ReverseWS ||
		newCfg.OneBot.ForwardWS != oldCfg.OneBot.ForwardWS ||
		newCfg.OneBot.HTTPAPI != oldCfg.OneBot.HTTPAPI ||
		newCfg.OneBot.Webhook != oldCfg.OneBot.Webhook ||
		newCfg.Adapter.ConnectTimeoutSeconds != oldCfg.Adapter.ConnectTimeoutSeconds ||
		newCfg.Adapter.ReconnectInitialSeconds != oldCfg.Adapter.ReconnectInitialSeconds ||
		newCfg.Adapter.ReconnectMultiplier != oldCfg.Adapter.ReconnectMultiplier ||
		newCfg.Adapter.ReconnectMaxSeconds != oldCfg.Adapter.ReconnectMaxSeconds ||
		newCfg.Adapter.ReconnectJitterRatio != oldCfg.Adapter.ReconnectJitterRatio
}
