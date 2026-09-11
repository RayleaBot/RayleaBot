package management

import (
	"context"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/pagination"
	"net/http"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/render"
)

const (
	renderCodeInvalidRequest  = errorcodes.PlatformInvalidRequest
	renderCodeResourceMissing = errorcodes.PlatformResourceNotFound
	renderCodeInternalError   = errorcodes.PlatformInternalError
)

type RenderHandlers struct {
	renderer   renderTemplateService
	pluginName func(string) string
}

type renderTemplateService interface {
	PreviewHTML(context.Context, render.Request) (render.PreviewHTML, error)
	LookupTemplateAsset(context.Context, string, string) (render.TemplateAsset, error)
	ListTemplates(context.Context) ([]render.TemplateSummary, error)
	GetTemplateDetailSnapshot(context.Context, string) (render.TemplateDetailSnapshot, error)
}

func NewRenderHandlers(renderer renderTemplateService, pluginName func(string) string) *RenderHandlers {
	return &RenderHandlers{renderer: renderer, pluginName: pluginName}
}

func (h *RenderHandlers) RegisterProtectedRoutes(router chi.Router) {
	router.Get("/api/system/render/templates", h.HandleSystemRenderTemplateList())
	router.Post("/api/system/render/templates/{template_id}/preview-html", h.HandleSystemRenderTemplatePreviewHTML())
	router.Get("/api/system/render/templates/{template_id}/asset", h.HandleSystemRenderTemplateAsset())
	router.Get("/api/system/render/templates/{template_id}", h.HandleSystemRenderTemplateDetail())
}

type renderTemplateSummary struct {
	ID             string               `json:"id"`
	Name           string               `json:"name"`
	Description    string               `json:"description,omitempty"`
	Version        string               `json:"version"`
	Width          int                  `json:"width"`
	Height         int                  `json:"height"`
	HasInputSchema bool                 `json:"has_input_schema"`
	UpdatedAt      string               `json:"updated_at"`
	Source         renderTemplateSource `json:"source"`
}

type renderTemplateDetail struct {
	ID              string               `json:"id"`
	Name            string               `json:"name"`
	Description     string               `json:"description,omitempty"`
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
	pagination.Metadata
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
	TemplateID   string `json:"template_id"`
	SourceDigest string `json:"source_digest"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	HTML         string `json:"html"`
}

func (h *RenderHandlers) HandleSystemRenderTemplateList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query, ok := readCollectionQuery(w, r)
		if !ok {
			return
		}
		items, err := h.renderer.ListTemplates(r.Context())
		if err != nil {
			httpapi.WriteError(w, r, renderCodeInternalError, nil)
			return
		}

		filtered := items[:0]
		for _, item := range items {
			pluginName := ""
			if query.Text != "" && h.pluginName != nil && item.Source.PluginID != "" {
				pluginName = h.pluginName(item.Source.PluginID)
			}
			if pagination.Matches(query.Text, item.ID, item.Name, item.Description, item.Source.PluginID, pluginName) {
				filtered = append(filtered, item)
			}
		}
		sort.Slice(filtered, func(i, j int) bool { return filtered[i].ID < filtered[j].ID })
		items, meta := pagination.Slice(filtered, query)
		response := renderListResponse{Metadata: meta,
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
			httpapi.WriteError(w, r, renderCodeInternalError, nil)
			return
		}

		templateID := chi.URLParam(r, "template_id")
		var request renderPreviewHTMLRequest
		if err := httpapi.DecodeStrictJSON(w, r, &request, httpapi.MaxManagementJSONBodyBytes); err != nil || request.Data == nil {
			httpapi.WriteError(w, r, renderCodeInvalidRequest, nil)
			return
		}

		result, err := h.renderer.PreviewHTML(r.Context(), render.Request{
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
			httpapi.WriteError(w, r, renderCodeResourceMissing, map[string]any{
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

func toRenderTemplateSummary(item render.TemplateSummary) renderTemplateSummary {
	return renderTemplateSummary{
		ID:             item.ID,
		Name:           item.Name,
		Description:    item.Description,
		Version:        item.Version,
		Width:          item.Width,
		Height:         item.Height,
		HasInputSchema: item.HasInputSchema,
		UpdatedAt:      item.UpdatedAt,
		Source:         toRenderTemplateSource(item.Source),
	}
}

func toRenderTemplateDetail(detail render.TemplateDetail, source render.TemplateSource, previewData map[string]any) renderTemplateDetail {
	return renderTemplateDetail{
		ID:              detail.ID,
		Name:            detail.Name,
		Description:     detail.Description,
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

func toRenderPreviewHTMLResponse(result render.PreviewHTML) renderPreviewHTMLResponse {
	return renderPreviewHTMLResponse{
		TemplateID:   result.TemplateID,
		SourceDigest: result.SourceDigest,
		Width:        result.Width,
		Height:       result.Height,
		HTML:         result.HTML,
	}
}

func toRenderTemplateSource(source render.TemplateSourceInfo) renderTemplateSource {
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
	renderErr, ok := render.AsTemplateError(err)
	if !ok {
		httpapi.WriteError(w, r, renderCodeInternalError, nil)
		return
	}

	switch renderErr.Code {
	case errorcodes.PlatformTemplateNotFound:
		httpapi.WriteError(w, r, renderErr.Code, nil)
	case errorcodes.PlatformInvalidRequest:
		httpapi.WriteError(w, r, renderErr.Code, nil)
	case errorcodes.PlatformRenderInputTooLarge:
		httpapi.WriteError(w, r, renderErr.Code, nil)
	case errorcodes.PlatformResourceMissing:
		httpapi.WriteError(w, r, errorcodes.PlatformResourceNotFound, map[string]any{
			"resource_type": "render_template_asset",
		})
	default:
		httpapi.WriteError(w, r, renderCodeInternalError, nil)
	}
}
