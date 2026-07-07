package management

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
)

const (
	renderCodeInvalidRequest  = "platform.invalid_request"
	renderCodeResourceMissing = "platform.resource_missing"
	renderCodeInternalError   = "platform.internal_error"
)

type RenderHandlers struct {
	renderer renderTemplateService
}

type renderTemplateService interface {
	PreviewHTML(context.Context, renderservice.Request) (renderservice.PreviewHTML, error)
	LookupTemplateAsset(context.Context, string, string) (renderservice.TemplateAsset, error)
	ListTemplates(context.Context) ([]renderservice.TemplateSummary, error)
	GetTemplateDetailSnapshot(context.Context, string) (renderservice.TemplateDetailSnapshot, error)
}

func NewRenderHandlers(renderer renderTemplateService) *RenderHandlers {
	return &RenderHandlers{renderer: renderer}
}

func (h *RenderHandlers) RegisterProtectedRoutes(router chi.Router) {
	router.Get("/api/system/render/templates", h.HandleSystemRenderTemplateList())
	router.Post("/api/system/render/templates/{template_id}/preview-html", h.HandleSystemRenderTemplatePreviewHTML())
	router.Get("/api/system/render/templates/{template_id}/asset", h.HandleSystemRenderTemplateAsset())
	router.Get("/api/system/render/templates/{template_id}", h.HandleSystemRenderTemplateDetail())
}

type renderTemplateSummary struct {
	ID             string               `json:"id"`
	Version        string               `json:"version"`
	Width          int                  `json:"width"`
	Height         int                  `json:"height"`
	HasInputSchema bool                 `json:"has_input_schema"`
	UpdatedAt      string               `json:"updated_at"`
	Source         renderTemplateSource `json:"source"`
}

type renderTemplateDetail struct {
	ID              string               `json:"id"`
	Version         string               `json:"version"`
	Width           int                  `json:"width"`
	Height          int                  `json:"height"`
	HasInputSchema  bool                 `json:"has_input_schema"`
	UpdatedAt       string               `json:"updated_at"`
	Source          renderTemplateSource `json:"source"`
	InputSchemaJSON map[string]any       `json:"input_schema_json"`
	PreviewDataJSON map[string]any       `json:"preview_data_json"`
}

type renderTemplateSource struct {
	Type     string  `json:"type"`
	PluginID *string `json:"plugin_id"`
	LocalID  *string `json:"local_id"`
}

type renderListResponse struct {
	Items []renderTemplateSummary `json:"items"`
}

type renderDetailResponse struct {
	Template renderTemplateDetail `json:"template"`
}

type renderPreviewHTMLRequest struct {
	Theme string         `json:"theme,omitempty"`
	Data  map[string]any `json:"data"`
}

type renderPreviewHTMLResponse struct {
	TemplateID string `json:"template_id"`
	RevisionID string `json:"revision_id"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	HTML       string `json:"html"`
}

func (h *RenderHandlers) HandleSystemRenderTemplateList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := h.renderer.ListTemplates(r.Context())
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, renderCodeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}

		response := renderListResponse{
			Items: make([]renderTemplateSummary, 0, len(items)),
		}
		for _, item := range items {
			response.Items = append(response.Items, toRenderTemplateSummary(item))
		}

		httpapi.WriteJSON(w, http.StatusOK, response)
	}
}

func (h *RenderHandlers) HandleSystemRenderTemplateDetail() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		templateID := chi.URLParam(r, "template_id")
		snapshot, err := h.renderer.GetTemplateDetailSnapshot(r.Context(), templateID)
		if err != nil {
			writeRenderTemplateError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, renderDetailResponse{
			Template: toRenderTemplateDetail(snapshot.Detail, snapshot.Source, snapshot.PreviewData),
		})
	}
}

func (h *RenderHandlers) HandleSystemRenderTemplatePreviewHTML() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.renderer == nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, renderCodeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}

		templateID := chi.URLParam(r, "template_id")
		var request renderPreviewHTMLRequest
		if err := httpapi.DecodeStrictJSON(w, r, &request, httpapi.MaxManagementJSONBodyBytes); err != nil || request.Data == nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, renderCodeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}

		result, err := h.renderer.PreviewHTML(r.Context(), renderservice.Request{
			Template: templateID,
			Theme:    request.Theme,
			Data:     request.Data,
		})
		if err != nil {
			writeRenderTemplateError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, toRenderPreviewHTMLResponse(result))
	}
}

func (h *RenderHandlers) HandleSystemRenderTemplateAsset() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.renderer == nil {
			httpapi.WriteError(w, r, http.StatusNotFound, renderCodeResourceMissing, "缺少必要资源", "errors.platform.resource_missing", map[string]any{
				"resource_type": "render_template_asset",
			})
			return
		}

		templateID := chi.URLParam(r, "template_id")
		asset, err := h.renderer.LookupTemplateAsset(r.Context(), templateID, r.URL.Query().Get("path"))
		if err != nil {
			writeRenderTemplateError(w, r, err)
			return
		}

		http.ServeFile(w, r, asset.Path)
	}
}

func toRenderTemplateSummary(item renderservice.TemplateSummary) renderTemplateSummary {
	return renderTemplateSummary{
		ID:             item.ID,
		Version:        item.Version,
		Width:          item.Width,
		Height:         item.Height,
		HasInputSchema: item.HasInputSchema,
		UpdatedAt:      item.UpdatedAt,
		Source:         toRenderTemplateSource(item.Source),
	}
}

func toRenderTemplateDetail(detail renderservice.TemplateDetail, source renderservice.TemplateSource, previewData map[string]any) renderTemplateDetail {
	return renderTemplateDetail{
		ID:              detail.ID,
		Version:         detail.Version,
		Width:           detail.Width,
		Height:          detail.Height,
		HasInputSchema:  detail.HasInputSchema,
		UpdatedAt:       detail.UpdatedAt,
		Source:          toRenderTemplateSource(detail.Source),
		InputSchemaJSON: source.InputSchemaJSON,
		PreviewDataJSON: previewData,
	}
}

func toRenderPreviewHTMLResponse(result renderservice.PreviewHTML) renderPreviewHTMLResponse {
	return renderPreviewHTMLResponse{
		TemplateID: result.TemplateID,
		RevisionID: result.RevisionID,
		Width:      result.Width,
		Height:     result.Height,
		HTML:       result.HTML,
	}
}

func toRenderTemplateSource(source renderservice.TemplateSourceInfo) renderTemplateSource {
	if source.Type != "plugin" {
		return renderTemplateSource{Type: "system", PluginID: nil, LocalID: nil}
	}
	return renderTemplateSource{
		Type:     "plugin",
		PluginID: renderStringPtr(source.PluginID),
		LocalID:  renderStringPtr(source.LocalID),
	}
}

func renderStringPtr(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func writeRenderTemplateError(w http.ResponseWriter, r *http.Request, err error) {
	renderErr, ok := renderservice.AsTemplateError(err)
	if !ok {
		httpapi.WriteError(w, r, http.StatusInternalServerError, renderCodeInternalError, "内部错误", "errors.platform.internal_error", nil)
		return
	}

	switch renderErr.Code {
	case "platform.template_not_found":
		httpapi.WriteError(w, r, http.StatusNotFound, renderErr.Code, "模板不存在", "errors.platform.template_not_found", nil)
	case "platform.invalid_request":
		httpapi.WriteError(w, r, http.StatusBadRequest, renderErr.Code, "请求参数不合法", "errors.platform.invalid_request", nil)
	case "platform.render_input_too_large":
		httpapi.WriteError(w, r, http.StatusRequestEntityTooLarge, renderErr.Code, "渲染输入超过大小限制", "errors.platform.render_input_too_large", nil)
	case "platform.resource_missing":
		httpapi.WriteError(w, r, http.StatusNotFound, renderErr.Code, "缺少必要资源", "errors.platform.resource_missing", map[string]any{
			"resource_type": "render_template_asset",
		})
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, renderCodeInternalError, "内部错误", "errors.platform.internal_error", nil)
	}
}
