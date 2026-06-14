package configapi

import (
	"net/http"

	internalconfig "github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/configruntime"
	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
)

const codeInvalidRequest = "platform.invalid_request"

type ApplyEffects = configruntime.ApplyEffects

func NewApplyEffects() ApplyEffects {
	return configruntime.NewApplyEffects()
}

type Response struct {
	Config         map[string]any `json:"config"`
	RedactedFields []string       `json:"redacted_fields,omitempty"`
}

type UpdateResponse struct {
	Config          map[string]any `json:"config"`
	RedactedFields  []string       `json:"redacted_fields,omitempty"`
	RestartRequired bool           `json:"restart_required"`
	ApplyEffects    ApplyEffects   `json:"apply_effects"`
}

type Service interface {
	CurrentConfigDocument() configruntime.Document
	UpdateConfigDocument(map[string]any) (configruntime.UpdateResult, error)
	ApplyHotReloadableFields(internalconfig.Config) configruntime.ApplyEffects
}

type Handlers struct {
	config Service
}

func NewHandlers(config Service) *Handlers {
	return &Handlers{config: config}
}

func (h *Handlers) HandleConfigGet() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeAuthJSON(w, http.StatusOK, responseFromDocument(h.config.CurrentConfigDocument()))
	}
}

func (h *Handlers) HandleConfigPut() http.HandlerFunc {
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

		writeAuthJSON(w, http.StatusOK, updateResponseFromResult(response))
	}
}

func (h *Handlers) ApplyHotReloadableFields(newCfg internalconfig.Config) ApplyEffects {
	if h == nil || h.config == nil {
		return NewApplyEffects()
	}
	return h.config.ApplyHotReloadableFields(newCfg)
}

func responseFromDocument(doc configruntime.Document) Response {
	return Response{
		Config:         doc.Config,
		RedactedFields: doc.RedactedFields,
	}
}

func updateResponseFromResult(result configruntime.UpdateResult) UpdateResponse {
	return UpdateResponse{
		Config:          result.Document.Config,
		RedactedFields:  result.Document.RedactedFields,
		RestartRequired: result.RestartRequired,
		ApplyEffects:    result.ApplyEffects,
	}
}

func writeAppError(w http.ResponseWriter, r *http.Request, statusCode int, code, message, messageKey string, details map[string]any) {
	httpapi.WriteError(w, r, statusCode, code, message, messageKey, details)
}

func writeAuthJSON(w http.ResponseWriter, statusCode int, body any) {
	httpapi.WriteJSON(w, statusCode, body)
}
