package app

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/render"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

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

type renderTemplateListResponse struct {
	Items []renderTemplateSummary `json:"items"`
}

type renderTemplateDetailResponse struct {
	Template renderTemplateDetail `json:"template"`
}

type renderTemplatePreviewHTMLRequest struct {
	Theme string         `json:"theme,omitempty"`
	Data  map[string]any `json:"data"`
}

type renderTemplatePreviewHTMLResponse struct {
	TemplateID string `json:"template_id"`
	RevisionID string `json:"revision_id"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	HTML       string `json:"html"`
}

func toRenderTemplateSummary(item render.TemplateSummary) renderTemplateSummary {
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

func toRenderTemplateDetail(detail render.TemplateDetail, source render.TemplateSource, previewData map[string]any) renderTemplateDetail {
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

func toRenderTemplatePreviewHTMLResponse(result render.PreviewHTML) renderTemplatePreviewHTMLResponse {
	return renderTemplatePreviewHTMLResponse{
		TemplateID: result.TemplateID,
		RevisionID: result.RevisionID,
		Width:      result.Width,
		Height:     result.Height,
		HTML:       result.HTML,
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

func (h *renderHTTPHandlers) handleSystemRenderTemplatePreviewHTML() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h == nil || h.renderer == nil {
			writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}

		templateID := chi.URLParam(r, "template_id")
		var request renderTemplatePreviewHTMLRequest
		if err := httpapi.DecodeStrictJSON(w, r, &request, httpapi.MaxManagementJSONBodyBytes); err != nil || request.Data == nil {
			writeAppError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
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

		writeAuthJSON(w, http.StatusOK, toRenderTemplatePreviewHTMLResponse(result))
	}
}

func (h *renderHTTPHandlers) handleSystemRenderTemplateAsset() http.HandlerFunc {
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
			writeRenderTemplateError(w, r, err)
			return
		}

		http.ServeFile(w, r, asset.Path)
	}
}

func (h *renderHTTPHandlers) handleSystemRenderPreview() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h == nil || h.taskExecutor == nil || h.renderer == nil {
			writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}

		var request render.Request
		if err := httpapi.DecodeStrictJSON(w, r, &request, httpapi.MaxManagementJSONBodyBytes); err != nil || strings.TrimSpace(request.Template) == "" {
			writeAppError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}

		taskID, err := h.taskExecutor.Submit("render.preview", "生成渲染预览", func(ctx context.Context, progress tasks.ProgressReporter) (*tasks.ResultSummary, error) {
			progress.Update(20, "准备渲染模板")
			result, renderErr := h.renderer.Render(ctx, request)
			if renderErr != nil {
				return nil, mapRenderTaskError(renderErr)
			}

			progress.Update(90, "生成渲染产物")
			return &tasks.ResultSummary{
				Summary: "渲染预览已生成",
				Details: map[string]any{
					"artifact_id": result.ArtifactID,
					"image_url":   h.renderer.ArtifactURL(result.ArtifactID),
					"mime":        result.MIME,
					"cache_key":   result.CacheKey,
					"template":    result.Template,
					"theme":       result.Theme,
					"from_cache":  result.FromCache,
				},
			}, nil
		})
		if err != nil {
			writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}

		writeAuthJSON(w, http.StatusAccepted, taskAcceptedResponse{TaskID: taskID})
	}
}

func (h *renderHTTPHandlers) handleSystemRenderArtifact() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h == nil || h.renderer == nil {
			writeAppError(w, r, http.StatusNotFound, codeResourceMissing, "缺少必要资源", "errors.platform.resource_missing", map[string]any{
				"resource_type": "render_artifact",
			})
			return
		}

		artifactID := chi.URLParam(r, "artifact_id")
		artifact, err := h.renderer.LookupArtifact(artifactID)
		if err != nil {
			var renderErr *render.Error
			if errors.As(err, &renderErr) && renderErr.Code == codeResourceMissing {
				writeAppError(w, r, http.StatusNotFound, codeResourceMissing, "缺少必要资源", "errors.platform.resource_missing", map[string]any{
					"resource_type": "render_artifact",
					"artifact_id":   artifactID,
				})
				return
			}
			writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}

		w.Header().Set("Content-Type", artifact.MIME)
		http.ServeFile(w, r, artifact.Path)
	}
}

func (h *renderHTTPHandlers) handleSystemRenderTemplateList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := h.renderer.ListTemplates(r.Context())
		if err != nil {
			writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}

		response := renderTemplateListResponse{
			Items: make([]renderTemplateSummary, 0, len(items)),
		}
		for _, item := range items {
			response.Items = append(response.Items, toRenderTemplateSummary(item))
		}

		writeAuthJSON(w, http.StatusOK, response)
	}
}

func (h *renderHTTPHandlers) handleSystemRenderTemplateDetail() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		templateID := chi.URLParam(r, "template_id")
		detail, err := h.renderer.GetTemplate(r.Context(), templateID)
		if err != nil {
			writeRenderTemplateError(w, r, err)
			return
		}

		_, source, err := h.renderer.GetTemplateSource(r.Context(), templateID)
		if err != nil {
			writeRenderTemplateError(w, r, err)
			return
		}
		previewData, err := h.renderer.GetTemplatePreviewData(r.Context(), templateID)
		if err != nil {
			writeRenderTemplateError(w, r, err)
			return
		}

		writeAuthJSON(w, http.StatusOK, renderTemplateDetailResponse{
			Template: toRenderTemplateDetail(detail, source, previewData),
		})
	}
}

func writeRenderTemplateError(w http.ResponseWriter, r *http.Request, err error) {
	var renderErr *render.Error
	if !errors.As(err, &renderErr) {
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

func mapRenderTaskError(err error) error {
	var renderErr *render.Error
	if errors.As(err, &renderErr) {
		return &tasks.TaskError{
			Code:    renderErr.Code,
			Message: renderErr.Message,
		}
	}
	return err
}
