package app

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (h *systemHTTPHandlers) handleSystemSchedulerJobList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h == nil || h.scheduler == nil {
			writeSystemHTTPError(w, r, missingSchedulerJobHTTPError(""))
			return
		}

		response, ok := h.scheduler.ListJobs()
		if !ok {
			writeSystemHTTPError(w, r, missingSchedulerJobHTTPError(""))
			return
		}
		writeAuthJSON(w, http.StatusOK, response)
	}
}

func (h *systemHTTPHandlers) handleSystemSchedulerJobTrigger() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h == nil || h.scheduler == nil {
			writeSystemHTTPError(w, r, missingSchedulerJobHTTPError(""))
			return
		}

		jobID := chi.URLParam(r, "job_id")
		response, systemErr := h.scheduler.TriggerJob(context.WithoutCancel(r.Context()), jobID)
		if systemErr != nil {
			writeSystemHTTPError(w, r, systemErr)
			return
		}

		writeAuthJSON(w, http.StatusOK, response)
	}
}
