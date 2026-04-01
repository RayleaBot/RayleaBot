package app

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"rayleabot/server/internal/render"
	"rayleabot/server/internal/tasks"
)

type renderPreviewRequest struct {
	Template string         `json:"template"`
	Theme    string         `json:"theme,omitempty"`
	Output   string         `json:"output,omitempty"`
	Data     map[string]any `json:"data"`
}

func (a *App) handleSystemRenderPreview() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if a == nil || a.taskExecutor == nil || a.renderer == nil {
			writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}

		var request renderPreviewRequest
		if err := decodeStrictJSON(w, r, &request, maxManagementJSONBodyBytes); err != nil || request.Template == "" || request.Data == nil {
			writeAppError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}

		taskID, err := a.taskExecutor.Submit("render.preview", "生成渲染预览", func(ctx context.Context, progress tasks.ProgressReporter) (*tasks.ResultSummary, error) {
			progress.Update(20, "准备渲染模板")
			result, renderErr := a.renderer.Render(ctx, render.Request{
				Template: request.Template,
				Theme:    request.Theme,
				Output:   request.Output,
				Data:     request.Data,
			})
			if renderErr != nil {
				return nil, mapRenderTaskError(renderErr)
			}

			progress.Update(90, "生成渲染产物")
			return &tasks.ResultSummary{
				Summary: "渲染预览已生成",
				Details: map[string]any{
					"artifact_id": result.ArtifactID,
					"image_url":   a.renderer.ArtifactURL(result.ArtifactID),
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

func (a *App) handleSystemRenderArtifact() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if a == nil || a.renderer == nil {
			writeAppError(w, r, http.StatusNotFound, codeResourceMissing, "必要运行时资源缺失", "errors.platform.resource_missing", map[string]any{
				"resource_type": "render_artifact",
			})
			return
		}

		artifactID := chi.URLParam(r, "artifact_id")
		artifact, err := a.renderer.LookupArtifact(artifactID)
		if err != nil {
			var renderErr *render.Error
			if errors.As(err, &renderErr) && renderErr.Code == codeResourceMissing {
				writeAppError(w, r, http.StatusNotFound, codeResourceMissing, "必要运行时资源缺失", "errors.platform.resource_missing", map[string]any{
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
