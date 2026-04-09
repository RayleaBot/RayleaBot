package app

import (
	"net/http"
	"slices"
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
	Config          map[string]any `json:"config"`
	RedactedFields  []string       `json:"redacted_fields,omitempty"`
	RestartRequired bool           `json:"restart_required"`
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
		newCfg, _, err := internalconfig.SaveDocument(h.state.Summary.ConfigPath, h.state.Summary.SchemaPath, resolved)
		if err != nil {
			writeAppError(w, r, http.StatusBadRequest, "platform.invalid_config", "配置校验失败", "errors.platform.invalid_config", nil)
			return
		}

		restartRequired := h.applyHotReloadableFields(newCfg)

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
func (h *configHTTPHandlers) applyHotReloadableFields(newCfg internalconfig.Config) bool {
	oldCfg := h.state.Config
	restartRequired := false

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
		newCfg.OneBot.Provider != oldCfg.OneBot.Provider ||
		newCfg.OneBot.AccessToken != oldCfg.OneBot.AccessToken ||
		newCfg.OneBot.ReverseWS != oldCfg.OneBot.ReverseWS ||
		newCfg.OneBot.ForwardWS != oldCfg.OneBot.ForwardWS ||
		newCfg.OneBot.HTTPAPI != oldCfg.OneBot.HTTPAPI ||
		newCfg.OneBot.Webhook != oldCfg.OneBot.Webhook ||
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
		newCfg.Render.BrowserPath != oldCfg.Render.BrowserPath ||
		!slices.Equal(newCfg.Render.BrowserArgs, oldCfg.Render.BrowserArgs) {
		restartRequired = true
	}

	// Update in-memory config to reflect the saved state.
	h.state.Config = newCfg
	if h.eventIngress != nil {
		h.eventIngress.UpdateConfig(newCfg)
	}
	if h.protocol != nil {
		h.protocol.PublishSnapshot()
	}

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
