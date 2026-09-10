package management

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	internalconfig "github.com/RayleaBot/RayleaBot/server/internal/config"
	configruntime "github.com/RayleaBot/RayleaBot/server/internal/config/runtime"
	"github.com/RayleaBot/RayleaBot/server/internal/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/httpapi"
)

const codeInvalidRequest = errorcodes.PlatformInvalidRequest

type ConfigResponse struct {
	Revision          uint64         `json:"revision"`
	EffectiveTimezone string         `json:"effective_timezone"`
	Config            map[string]any `json:"config"`
	RedactedFields    []string       `json:"redacted_fields,omitempty"`
}

type ConfigUpdateResponse struct {
	Revision          uint64                     `json:"revision"`
	EffectiveTimezone string                     `json:"effective_timezone"`
	Config            map[string]any             `json:"config"`
	RedactedFields    []string                   `json:"redacted_fields,omitempty"`
	RestartRequired   bool                       `json:"restart_required"`
	ApplyEffects      configruntime.ApplyEffects `json:"apply_effects"`
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
			httpapi.WriteError(w, r, codeInvalidRequest, nil)
			return
		}

		response, err := h.config.UpdateConfigDocument(r.Context(), request)
		if err != nil {
			var persistenceError *configruntime.PersistenceError
			if errors.As(err, &persistenceError) {
				httpapi.WriteError(w, r, errorcodes.PlatformInternalError, nil)
				return
			}
			httpapi.WriteError(w, r, errorcodes.PlatformInvalidConfig, configValidationDetails(err))
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, updateResponseFromResult(response))
	}
}

func (h *ConfigHandlers) ApplyHotReloadableFields(newCfg internalconfig.Config) configruntime.ApplyEffects {
	if h.config == nil {
		return configruntime.NewApplyEffects()
	}
	return h.config.ApplyHotReloadableFields(newCfg)
}

func responseFromDocument(doc configruntime.Document) ConfigResponse {
	return ConfigResponse{
		Revision:          doc.Revision,
		EffectiveTimezone: doc.EffectiveTimezone,
		Config:            doc.Config,
		RedactedFields:    doc.RedactedFields,
	}
}

func updateResponseFromResult(result configruntime.UpdateResult) ConfigUpdateResponse {
	return ConfigUpdateResponse{
		Revision:          result.Document.Revision,
		EffectiveTimezone: result.Document.EffectiveTimezone,
		Config:            result.Document.Config,
		RedactedFields:    result.Document.RedactedFields,
		RestartRequired:   result.RestartRequired,
		ApplyEffects:      result.ApplyEffects,
	}
}

func configValidationDetails(err error) map[string]any {
	fields := internalconfig.ValidationErrorDetails(err)
	if len(fields) == 0 {
		return nil
	}
	return map[string]any{"fields": fields}
}
