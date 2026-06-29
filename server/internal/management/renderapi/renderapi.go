package renderapi

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
)

const (
	codeInvalidRequest  = "platform.invalid_request"
	codeResourceMissing = "platform.resource_missing"
	codeInternalError   = "platform.internal_error"
)

type Handlers struct {
	renderer templateService
}

type templateService interface {
	PreviewHTML(context.Context, renderservice.Request) (renderservice.PreviewHTML, error)
	LookupTemplateAsset(context.Context, string, string) (renderservice.TemplateAsset, error)
	ListTemplates(context.Context) ([]renderservice.TemplateSummary, error)
	GetTemplateDetailSnapshot(context.Context, string) (renderservice.TemplateDetailSnapshot, error)
}

type Service = templateService

type ModuleDeps struct {
	Renderer Service
}

func NewHandlers(renderer templateService) *Handlers {
	return &Handlers{renderer: renderer}
}

func NewModule(deps ModuleDeps) *Handlers {
	return NewHandlers(deps.Renderer)
}

type templateSummary struct {
	ID             string         `json:"id"`
	Version        string         `json:"version"`
	Width          int            `json:"width"`
	Height         int            `json:"height"`
	HasInputSchema bool           `json:"has_input_schema"`
	UpdatedAt      string         `json:"updated_at"`
	Source         templateSource `json:"source"`
}

type templateDetail struct {
	ID              string         `json:"id"`
	Version         string         `json:"version"`
	Width           int            `json:"width"`
	Height          int            `json:"height"`
	HasInputSchema  bool           `json:"has_input_schema"`
	UpdatedAt       string         `json:"updated_at"`
	Source          templateSource `json:"source"`
	InputSchemaJSON map[string]any `json:"input_schema_json"`
	PreviewDataJSON map[string]any `json:"preview_data_json"`
}

type templateSource struct {
	Type     string  `json:"type"`
	PluginID *string `json:"plugin_id"`
	LocalID  *string `json:"local_id"`
}

type listResponse struct {
	Items []templateSummary `json:"items"`
}

type detailResponse struct {
	Template templateDetail `json:"template"`
}

type previewHTMLRequest struct {
	Theme string         `json:"theme,omitempty"`
	Data  map[string]any `json:"data"`
}

type previewHTMLResponse struct {
	TemplateID string `json:"template_id"`
	RevisionID string `json:"revision_id"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	HTML       string `json:"html"`
}

func (h *Handlers) HandleSystemRenderTemplateList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := h.renderer.ListTemplates(r.Context())
		if err != nil {
			writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}

		response := listResponse{
			Items: make([]templateSummary, 0, len(items)),
		}
		for _, item := range items {
			response.Items = append(response.Items, toTemplateSummary(item))
		}

		writeAuthJSON(w, http.StatusOK, response)
	}
}

func (h *Handlers) HandleSystemRenderTemplateDetail() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		templateID := chi.URLParam(r, "template_id")
		snapshot, err := h.renderer.GetTemplateDetailSnapshot(r.Context(), templateID)
		if err != nil {
			writeTemplateError(w, r, err)
			return
		}

		writeAuthJSON(w, http.StatusOK, detailResponse{
			Template: toTemplateDetail(snapshot.Detail, snapshot.Source, snapshot.PreviewData),
		})
	}
}

func (h *Handlers) HandleSystemRenderTemplatePreviewHTML() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h == nil || h.renderer == nil {
			writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}

		templateID := chi.URLParam(r, "template_id")
		var request previewHTMLRequest
		if err := httpapi.DecodeStrictJSON(w, r, &request, httpapi.MaxManagementJSONBodyBytes); err != nil || request.Data == nil {
			writeAppError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}

		result, err := h.renderer.PreviewHTML(r.Context(), renderservice.Request{
			Template: templateID,
			Theme:    request.Theme,
			Data:     request.Data,
		})
		if err != nil {
			writeTemplateError(w, r, err)
			return
		}

		writeAuthJSON(w, http.StatusOK, toPreviewHTMLResponse(result))
	}
}

func (h *Handlers) HandleSystemRenderTemplateAsset() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h == nil || h.renderer == nil {
			writeAppError(w, r, http.StatusNotFound, codeResourceMissing, "缺少必要资源", "errors.platform.resource_missing", map[string]any{
				"resource_type": "render_template_asset",
			})
			return
		}

		templateID := chi.URLParam(r, "template_id")
		asset, err := h.renderer.LookupTemplateAsset(r.Context(), templateID, r.URL.Query().Get("path"))
		if err != nil {
			writeTemplateError(w, r, err)
			return
		}

		http.ServeFile(w, r, asset.Path)
	}
}

func toTemplateSummary(item renderservice.TemplateSummary) templateSummary {
	return templateSummary{
		ID:             item.ID,
		Version:        item.Version,
		Width:          item.Width,
		Height:         item.Height,
		HasInputSchema: item.HasInputSchema,
		UpdatedAt:      item.UpdatedAt,
		Source:         toTemplateSource(item.Source),
	}
}

func toTemplateDetail(detail renderservice.TemplateDetail, source renderservice.TemplateSource, previewData map[string]any) templateDetail {
	return templateDetail{
		ID:              detail.ID,
		Version:         detail.Version,
		Width:           detail.Width,
		Height:          detail.Height,
		HasInputSchema:  detail.HasInputSchema,
		UpdatedAt:       detail.UpdatedAt,
		Source:          toTemplateSource(detail.Source),
		InputSchemaJSON: source.InputSchemaJSON,
		PreviewDataJSON: previewData,
	}
}

func toPreviewHTMLResponse(result renderservice.PreviewHTML) previewHTMLResponse {
	return previewHTMLResponse{
		TemplateID: result.TemplateID,
		RevisionID: result.RevisionID,
		Width:      result.Width,
		Height:     result.Height,
		HTML:       result.HTML,
	}
}

func toTemplateSource(source renderservice.TemplateSourceInfo) templateSource {
	if source.Type != "plugin" {
		return templateSource{Type: "system", PluginID: nil, LocalID: nil}
	}
	return templateSource{
		Type:     "plugin",
		PluginID: stringPtr(source.PluginID),
		LocalID:  stringPtr(source.LocalID),
	}
}

func stringPtr(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func writeTemplateError(w http.ResponseWriter, r *http.Request, err error) {
	renderErr, ok := renderservice.AsTemplateError(err)
	if !ok {
		writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
		return
	}

	switch renderErr.Code {
	case "platform.template_not_found":
		writeAppError(w, r, http.StatusNotFound, renderErr.Code, "模板不存在", "errors.platform.template_not_found", nil)
	case "platform.invalid_request":
		writeAppError(w, r, http.StatusBadRequest, renderErr.Code, "请求参数不合法", "errors.platform.invalid_request", nil)
	case "platform.render_input_too_large":
		writeAppError(w, r, http.StatusRequestEntityTooLarge, renderErr.Code, "渲染输入超过大小限制", "errors.platform.render_input_too_large", nil)
	case "platform.resource_missing":
		writeAppError(w, r, http.StatusNotFound, renderErr.Code, "缺少必要资源", "errors.platform.resource_missing", map[string]any{
			"resource_type": "render_template_asset",
		})
	default:
		writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
	}
}

func writeAppError(w http.ResponseWriter, r *http.Request, statusCode int, code, message, messageKey string, details map[string]any) {
	httpapi.WriteError(w, r, statusCode, code, message, messageKey, details)
}

func writeAuthJSON(w http.ResponseWriter, statusCode int, body any) {
	httpapi.WriteJSON(w, statusCode, body)
}
