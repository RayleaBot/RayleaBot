package managementhttp

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (h *SystemHandlers) HandleSystemSchedulerJobList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h == nil || h.scheduler == nil {
			WriteSystemHTTPError(w, r, missingSchedulerJobHTTPError(""))
			return
		}

		response, ok := h.scheduler.ListJobs()
		if !ok {
			WriteSystemHTTPError(w, r, missingSchedulerJobHTTPError(""))
			return
		}
		writeAuthJSON(w, http.StatusOK, response)
	}
}

func (h *SystemHandlers) HandleSystemSchedulerJobTrigger() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h == nil || h.scheduler == nil {
			WriteSystemHTTPError(w, r, missingSchedulerJobHTTPError(""))
			return
		}

		jobID := chi.URLParam(r, "job_id")
		response, systemErr := h.scheduler.TriggerJob(context.WithoutCancel(r.Context()), jobID)
		if systemErr != nil {
			WriteSystemHTTPError(w, r, systemErr)
			return
		}

		writeAuthJSON(w, http.StatusOK, response)
	}
}
