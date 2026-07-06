package management

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	internalconfig "github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/configruntime"
	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
)

const codeInvalidRequest = "platform.invalid_request"

type ConfigResponse struct {
	Config         map[string]any `json:"config"`
	RedactedFields []string       `json:"redacted_fields,omitempty"`
}

type ConfigUpdateResponse struct {
	Config          map[string]any             `json:"config"`
	RedactedFields  []string                   `json:"redacted_fields,omitempty"`
	RestartRequired bool                       `json:"restart_required"`
	ApplyEffects    configruntime.ApplyEffects `json:"apply_effects"`
}

type ConfigService interface {
	CurrentConfigDocument() configruntime.Document
	UpdateConfigDocument(context.Context, map[string]any) (configruntime.UpdateResult, error)
	ApplyHotReloadableFields(internalconfig.Config) configruntime.ApplyEffects
}

type ConfigHandlers struct {
	config ConfigService
}

func NewConfigHandlers(config ConfigService) *ConfigHandlers {
	return &ConfigHandlers{config: config}
}

func (h *ConfigHandlers) RegisterProtectedRoutes(router chi.Router) {
	router.Get("/api/config", h.HandleConfigGet())
	router.Put("/api/config", h.HandleConfigPut())
}

func (h *ConfigHandlers) HandleConfigGet() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		httpapi.WriteJSON(w, http.StatusOK, responseFromDocument(h.config.CurrentConfigDocument()))
	}
}

func (h *ConfigHandlers) HandleConfigPut() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request map[string]any
		if err := httpapi.DecodeStrictJSON(w, r, &request, httpapi.MaxManagementJSONBodyBytes); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}

		response, err := h.config.UpdateConfigDocument(r.Context(), request)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "platform.invalid_config", "配置校验失败", "errors.platform.invalid_config", configValidationDetails(err))
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, updateResponseFromResult(response))
	}
}

func (h *ConfigHandlers) ApplyHotReloadableFields(newCfg internalconfig.Config) configruntime.ApplyEffects {
	if h == nil || h.config == nil {
		return configruntime.NewApplyEffects()
	}
	return h.config.ApplyHotReloadableFields(newCfg)
}

func responseFromDocument(doc configruntime.Document) ConfigResponse {
	return ConfigResponse{
		Config:         doc.Config,
		RedactedFields: doc.RedactedFields,
	}
}

func updateResponseFromResult(result configruntime.UpdateResult) ConfigUpdateResponse {
	return ConfigUpdateResponse{
		Config:          result.Document.Config,
		RedactedFields:  result.Document.RedactedFields,
		RestartRequired: result.RestartRequired,
		ApplyEffects:    result.ApplyEffects,
	}
}

func configValidationDetails(err error) map[string]any {
	fields := internalconfig.ValidationErrorDetails(err)
	if len(fields) == 0 {
		return nil
	}
	return map[string]any{"fields": fields}
}
