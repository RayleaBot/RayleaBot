package app

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
)

type schedulerJobTriggerResponse struct {
	JobID     string `json:"job_id"`
	PluginID  string `json:"plugin_id"`
	Triggered bool   `json:"triggered"`
}

func (h *systemHTTPHandlers) handleSystemSchedulerJobTrigger() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h == nil || h.scheduler == nil {
			writeAppError(w, r, http.StatusNotFound, codeResourceMissing, "缺少必要资源", "errors.platform.resource_missing", map[string]any{
				"resource_type": "scheduler_job",
			})
			return
		}

		jobID := chi.URLParam(r, "job_id")
		job, err := h.scheduler.Trigger(r.Context(), jobID)
		if err != nil {
			if errors.Is(err, scheduler.ErrJobNotFound) {
				writeAppError(w, r, http.StatusNotFound, codeResourceMissing, "缺少必要资源", "errors.platform.resource_missing", map[string]any{
					"resource_type": "scheduler_job",
					"job_id":        jobID,
				})
				return
			}
			writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}

		writeAuthJSON(w, http.StatusOK, schedulerJobTriggerResponse{
			JobID:     job.JobID,
			PluginID:  job.PluginID,
			Triggered: true,
		})
	}
}
