package app

import (
	"net/http"

	internalconfig "github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
)

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

func (h *configHTTPHandlers) handleConfigGet() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeAuthJSON(w, http.StatusOK, h.config.currentConfigDocument())
	}
}

func (h *configHTTPHandlers) handleConfigPut() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request map[string]any
		if err := httpapi.DecodeStrictJSON(w, r, &request, httpapi.MaxManagementJSONBodyBytes); err != nil {
			writeAppError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}

		response, err := h.config.updateConfigDocument(request)
		if err != nil {
			writeAppError(w, r, http.StatusBadRequest, "platform.invalid_config", "配置校验失败", "errors.platform.invalid_config", nil)
			return
		}

		writeAuthJSON(w, http.StatusOK, response)
	}
}

func (h *configHTTPHandlers) applyHotReloadableFields(newCfg internalconfig.Config) configApplyEffects {
	if h == nil || h.config == nil {
		return newConfigApplyEffects()
	}
	return h.config.applyHotReloadableFields(newCfg)
}

func (s *configHTTPServiceImpl) currentConfigDocument() configResponse {
	document, redactedFields := sanitizeConfigDocument(configDocumentFromTyped(s.state.Config))
	return configResponse{
		Config:         document,
		RedactedFields: redactedFields,
	}
}

func (s *configHTTPServiceImpl) updateConfigDocument(request map[string]any) (configUpdateResponse, error) {
	newCfg, newSummary, err := internalconfig.SaveDocument(s.state.Summary.ConfigPath, s.state.Summary.SchemaPath, request)
	if err != nil {
		return configUpdateResponse{}, err
	}

	applyEffects := s.applyHotReloadableFields(newCfg)
	s.state.Summary = newSummary

	document, redactedFields := sanitizeConfigDocument(configDocumentFromTyped(newCfg))
	return configUpdateResponse{
		Config:          document,
		RedactedFields:  redactedFields,
		RestartRequired: applyEffects.restartRequired(),
		ApplyEffects:    applyEffects,
	}, nil
}

func configDocumentFromTyped(cfg internalconfig.Config) map[string]any {
	return internalconfig.CanonicalDocumentFromTyped(cfg)
}

func sanitizeConfigDocument(document map[string]any) (map[string]any, []string) {
	cloned := internalconfig.CloneDocument(document)
	if cloned == nil {
		return nil, nil
	}

	return cloned, []string{}
}
