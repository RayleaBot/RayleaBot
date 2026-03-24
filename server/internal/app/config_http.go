package app

import (
	"net/http"

	internalconfig "rayleabot/server/internal/config"
)

const redactedConfigValue = "__REDACTED__"

type configResponse struct {
	Config         map[string]any `json:"config"`
	RedactedFields []string       `json:"redacted_fields,omitempty"`
}

type configUpdateResponse struct {
	Config          map[string]any `json:"config"`
	RedactedFields  []string       `json:"redacted_fields,omitempty"`
	RestartRequired bool           `json:"restart_required"`
}

func (a *App) handleConfigGet() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		document, redactedFields := sanitizeConfigDocument(configDocumentFromTyped(a.Config))
		writeAuthJSON(w, http.StatusOK, configResponse{
			Config:         document,
			RedactedFields: redactedFields,
		})
	}
}

func (a *App) handleConfigPut() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request map[string]any
		if err := decodeStrictJSON(r, &request); err != nil {
			writeAppError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}

		resolved := resolveRedactedConfigValues(request, a.Config)
		newCfg, _, err := internalconfig.SaveDocument(a.Summary.ConfigPath, a.Summary.SchemaPath, resolved)
		if err != nil {
			writeAppError(w, r, http.StatusBadRequest, "platform.invalid_config", "配置校验失败", "errors.platform.invalid_config", nil)
			return
		}

		restartRequired := applyHotReloadableFields(a, newCfg)

		document, redactedFields := sanitizeConfigDocument(resolved)
		writeAuthJSON(w, http.StatusOK, configUpdateResponse{
			Config:          document,
			RedactedFields:  redactedFields,
			RestartRequired: restartRequired,
		})
	}
}

// applyHotReloadableFields compares the new config with the current config,
// applies fields that can take effect immediately, and returns true if any
// non-hot-reloadable field has changed (requiring a restart).
func applyHotReloadableFields(a *App, newCfg internalconfig.Config) bool {
	oldCfg := a.Config
	restartRequired := false

	// logging.level — immediate effect via LevelController.
	if newCfg.Logging.Level != oldCfg.Logging.Level {
		if a.LogLevel != nil {
			if err := a.LogLevel.SetLevel(newCfg.Logging.Level); err == nil {
				a.Logger.Info("log level changed",
					"component", "config",
					"old_level", oldCfg.Logging.Level,
					"new_level", newCfg.Logging.Level,
				)
			}
		}
	}
	if newCfg.Logging.RetentionDays != oldCfg.Logging.RetentionDays && a.Logs != nil {
		a.Logs.SetRepository(a.LogRepository, newCfg.Logging.RetentionDays)
	}
	if newCfg.Logging.RateLimitPerPlugin != oldCfg.Logging.RateLimitPerPlugin && a.pluginLogLimiter != nil {
		a.pluginLogLimiter.SetLimit(parsePluginLogRateLimit(newCfg))
	}

	// Fields that require a restart when changed.
	if newCfg.Server.Host != oldCfg.Server.Host ||
		newCfg.Server.Port != oldCfg.Server.Port {
		restartRequired = true
	}
	if newCfg.Database.Engine != oldCfg.Database.Engine ||
		newCfg.Database.Path != oldCfg.Database.Path {
		restartRequired = true
	}
	if newCfg.OneBot.WSURL != oldCfg.OneBot.WSURL ||
		newCfg.OneBot.AccessToken != oldCfg.OneBot.AccessToken ||
		newCfg.OneBot.ConnectTimeoutSeconds != oldCfg.OneBot.ConnectTimeoutSeconds ||
		newCfg.OneBot.ReconnectInitialSeconds != oldCfg.OneBot.ReconnectInitialSeconds ||
		newCfg.OneBot.ReconnectMultiplier != oldCfg.OneBot.ReconnectMultiplier ||
		newCfg.OneBot.ReconnectMaxSeconds != oldCfg.OneBot.ReconnectMaxSeconds ||
		newCfg.OneBot.ReconnectJitterRatio != oldCfg.OneBot.ReconnectJitterRatio {
		restartRequired = true
	}
	if newCfg.Auth.SessionTTLDays != oldCfg.Auth.SessionTTLDays ||
		newCfg.Auth.SlidingRenewal != oldCfg.Auth.SlidingRenewal ||
		newCfg.Auth.MaxSessions != oldCfg.Auth.MaxSessions {
		restartRequired = true
	}
	if newCfg.Web.ExposureMode != oldCfg.Web.ExposureMode ||
		newCfg.Web.SetupLocalOnly != oldCfg.Web.SetupLocalOnly {
		restartRequired = true
	}
	if newCfg.Render.WorkerCount != oldCfg.Render.WorkerCount ||
		newCfg.Render.BrowserPath != oldCfg.Render.BrowserPath {
		restartRequired = true
	}

	// Update in-memory config to reflect the saved state.
	a.Config = newCfg
	a.commandParser = newCommandParser(newCfg)
	a.permissionChecker = newPermissionChecker(newCfg, a.blacklistRepo)

	return restartRequired
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
