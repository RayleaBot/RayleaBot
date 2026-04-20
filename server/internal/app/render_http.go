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

type renderTemplateSourceResponse struct {
	TemplateID string                `json:"template_id"`
	RevisionID string                `json:"revision_id"`
	Source     render.TemplateSource `json:"source"`
}

type renderTemplateListResponse struct {
	Items []render.TemplateSummary `json:"items"`
}

type renderTemplateDetailResponse struct {
	Template render.TemplateDetail `json:"template"`
}

type renderTemplateVersionListResponse struct {
	Items []render.TemplateVersion `json:"items"`
}

type renderTemplateSourceUpdateRequest struct {
	BaseRevisionID string                `json:"base_revision_id"`
	Source         render.TemplateSource `json:"source"`
	Message        string                `json:"message"`
}

type renderTemplateValidateRequest struct {
	Source *render.TemplateSource `json:"source"`
}

type renderTemplateRollbackRequest struct {
	TargetRevisionID string `json:"target_revision_id"`
	BaseRevisionID   string `json:"base_revision_id"`
	Message          string `json:"message"`
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
		writeAuthJSON(w, http.StatusOK, renderTemplateListResponse{Items: items})
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
		writeAuthJSON(w, http.StatusOK, renderTemplateDetailResponse{Template: detail})
	}
}

func (h *renderHTTPHandlers) handleSystemRenderTemplateSource() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		templateID := chi.URLParam(r, "template_id")
		revisionID, source, err := h.renderer.GetTemplateSource(r.Context(), templateID)
		if err != nil {
			writeRenderTemplateError(w, r, err)
			return
		}
		writeAuthJSON(w, http.StatusOK, renderTemplateSourceResponse{
			TemplateID: templateID,
			RevisionID: revisionID,
			Source:     source,
		})
	}
}

func (h *renderHTTPHandlers) handleSystemRenderTemplateSourcePut() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request renderTemplateSourceUpdateRequest
		if err := httpapi.DecodeStrictJSON(w, r, &request, httpapi.MaxManagementJSONBodyBytes); err != nil ||
			strings.TrimSpace(request.BaseRevisionID) == "" ||
			strings.TrimSpace(request.Message) == "" {
			writeAppError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}

		detail, err := h.renderer.UpdateTemplateSource(
			r.Context(),
			chi.URLParam(r, "template_id"),
			request.BaseRevisionID,
			request.Message,
			request.Source,
		)
		if err != nil {
			writeRenderTemplateError(w, r, err)
			return
		}

		writeAuthJSON(w, http.StatusOK, renderTemplateDetailResponse{Template: detail})
	}
}

func (h *renderHTTPHandlers) handleSystemRenderTemplateValidate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request renderTemplateValidateRequest
		if err := httpapi.DecodeStrictJSON(w, r, &request, httpapi.MaxManagementJSONBodyBytes); err != nil {
			writeAppError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}

		result, err := h.renderer.ValidateTemplate(r.Context(), chi.URLParam(r, "template_id"), request.Source)
		if err != nil {
			writeRenderTemplateError(w, r, err)
			return
		}

		writeAuthJSON(w, http.StatusOK, result)
	}
}

func (h *renderHTTPHandlers) handleSystemRenderTemplateVersions() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := h.renderer.ListTemplateVersions(r.Context(), chi.URLParam(r, "template_id"))
		if err != nil {
			writeRenderTemplateError(w, r, err)
			return
		}

		writeAuthJSON(w, http.StatusOK, renderTemplateVersionListResponse{Items: items})
	}
}

func (h *renderHTTPHandlers) handleSystemRenderTemplateRollback() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request renderTemplateRollbackRequest
		if err := httpapi.DecodeStrictJSON(w, r, &request, httpapi.MaxManagementJSONBodyBytes); err != nil ||
			strings.TrimSpace(request.TargetRevisionID) == "" ||
			strings.TrimSpace(request.BaseRevisionID) == "" ||
			strings.TrimSpace(request.Message) == "" {
			writeAppError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}

		detail, err := h.renderer.RollbackTemplate(
			r.Context(),
			chi.URLParam(r, "template_id"),
			request.TargetRevisionID,
			request.BaseRevisionID,
			request.Message,
		)
		if err != nil {
			writeRenderTemplateError(w, r, err)
			return
		}

		writeAuthJSON(w, http.StatusOK, renderTemplateDetailResponse{Template: detail})
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
	case "platform.template_source_invalid":
		writeAppError(w, r, http.StatusBadRequest, renderErr.Code, "模板源码不合法", "errors.platform.template_source_invalid", nil)
	case "platform.template_revision_conflict":
		writeAppError(w, r, http.StatusConflict, renderErr.Code, "模板版本已变化", "errors.platform.template_revision_conflict", nil)
	case "platform.template_revision_not_found":
		writeAppError(w, r, http.StatusNotFound, renderErr.Code, "模板版本不存在", "errors.platform.template_revision_not_found", nil)
	case "platform.template_rollback_target_invalid":
		writeAppError(w, r, http.StatusConflict, renderErr.Code, "回退目标不合法", "errors.platform.template_rollback_target_invalid", nil)
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
