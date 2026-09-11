package management

import (
	"context"
	"errors"
	"net/http"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
	"github.com/go-chi/chi/v5"
)

func (h *SystemHandlers) HandleSystemSchedulerJobList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.scheduler == nil {
			writeSystemHTTPError(w, r, missingSchedulerJobHTTPError(""))
			return
		}
		query, ok := readCollectionQuery(w, r)
		if !ok {
			return
		}
		status, ok := readCollectionChoice(w, r, "status", "success", "error")
		if !ok {
			return
		}
		order, ok := readCollectionChoice(w, r, "sort", "name", "last_run", "duration")
		if !ok {
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, h.scheduler.ListJobsPage(scheduler.JobQuery{Query: query, Status: status, Sort: order}))
	}
}

func (h *SystemHandlers) HandleSystemSchedulerJobTrigger() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.scheduler == nil {
			writeSystemHTTPError(w, r, missingSchedulerJobHTTPError(""))
			return
		}
		jobID := chi.URLParam(r, "job_id")
		response, err := h.scheduler.TriggerJob(context.WithoutCancel(r.Context()), jobID)
		if err != nil {
			if errors.Is(err, scheduler.ErrJobNotFound) {
				writeSystemHTTPError(w, r, missingSchedulerJobHTTPError(jobID))
			} else {
				writeSystemHTTPError(w, r, internalSystemHTTPError())
			}
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, response)
	}
}

func missingSchedulerJobHTTPError(jobID string) *systemHTTPError {
	details := map[string]any{"resource_type": "scheduler_job"}
	if jobID != "" {
		details["job_id"] = jobID
	}
	return missingSystemResourceHTTPError(details)
}
