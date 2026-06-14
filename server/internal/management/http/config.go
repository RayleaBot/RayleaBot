package managementhttp

import (
	"net/http"

	internalconfig "github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
)

type ConfigApplyEffects struct {
	AppliedNow            []string `json:"applied_now"`
	ReloadedNow           []string `json:"reloaded_now"`
	RestartRequiredFields []string `json:"restart_required_fields"`
}

func NewConfigApplyEffects() ConfigApplyEffects {
	return ConfigApplyEffects{
		AppliedNow:            []string{},
		ReloadedNow:           []string{},
		RestartRequiredFields: []string{},
	}
}

func (e ConfigApplyEffects) RestartRequired() bool {
	return len(e.RestartRequiredFields) > 0
}

type ConfigResponse struct {
	Config         map[string]any `json:"config"`
	RedactedFields []string       `json:"redacted_fields,omitempty"`
}

type ConfigUpdateResponse struct {
	Config          map[string]any     `json:"config"`
	RedactedFields  []string           `json:"redacted_fields,omitempty"`
	RestartRequired bool               `json:"restart_required"`
	ApplyEffects    ConfigApplyEffects `json:"apply_effects"`
}

type ConfigService interface {
	CurrentConfigDocument() ConfigResponse
	UpdateConfigDocument(map[string]any) (ConfigUpdateResponse, error)
	ApplyHotReloadableFields(internalconfig.Config) ConfigApplyEffects
}

type ConfigHandlers struct {
	config ConfigService
}

func NewConfigHandlers(config ConfigService) *ConfigHandlers {
	return &ConfigHandlers{config: config}
}

func (h *ConfigHandlers) HandleConfigGet() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeAuthJSON(w, http.StatusOK, h.config.CurrentConfigDocument())
	}
}

func (h *ConfigHandlers) HandleConfigPut() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request map[string]any
		if err := httpapi.DecodeStrictJSON(w, r, &request, httpapi.MaxManagementJSONBodyBytes); err != nil {
			writeAppError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}

		response, err := h.config.UpdateConfigDocument(request)
		if err != nil {
			writeAppError(w, r, http.StatusBadRequest, "platform.invalid_config", "配置校验失败", "errors.platform.invalid_config", nil)
			return
		}

		writeAuthJSON(w, http.StatusOK, response)
	}
}

func (h *ConfigHandlers) ApplyHotReloadableFields(newCfg internalconfig.Config) ConfigApplyEffects {
	if h == nil || h.config == nil {
		return NewConfigApplyEffects()
	}
	return h.config.ApplyHotReloadableFields(newCfg)
}
