package management

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
)

func (h *SystemHandlers) HandleSystemSchedulerJobList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.scheduler == nil {
			WriteSystemHTTPError(w, r, missingSchedulerJobHTTPError(""))
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, h.scheduler.ListJobs())
	}
}

func (h *SystemHandlers) HandleSystemSchedulerJobTrigger() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.scheduler == nil {
			WriteSystemHTTPError(w, r, missingSchedulerJobHTTPError(""))
			return
		}
		jobID := chi.URLParam(r, "job_id")
		response, err := h.scheduler.TriggerJob(context.WithoutCancel(r.Context()), jobID)
		if err != nil {
			if errors.Is(err, scheduler.ErrJobNotFound) {
				WriteSystemHTTPError(w, r, missingSchedulerJobHTTPError(jobID))
			} else {
				WriteSystemHTTPError(w, r, InternalSystemHTTPError())
			}
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, response)
	}
}

func missingSchedulerJobHTTPError(jobID string) *SystemHTTPError {
	details := map[string]any{"resource_type": "scheduler_job"}
	if jobID != "" {
		details["job_id"] = jobID
	}
	return MissingSystemResourceHTTPError(details)
}
