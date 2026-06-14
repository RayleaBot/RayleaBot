package managementhttp

import (
	"context"
	"errors"
	"sort"

	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
)

func (s *schedulerHTTPServiceImpl) ListJobs() (schedulerJobListResponse, bool) {
	if s == nil || s.scheduler == nil {
		return schedulerJobListResponse{}, false
	}
	jobs := s.scheduler.Jobs()
	sort.Slice(jobs, func(i, j int) bool {
		if jobs[i].PluginID == jobs[j].PluginID {
			return jobs[i].JobID < jobs[j].JobID
		}
		return jobs[i].PluginID < jobs[j].PluginID
	})
	items := make([]schedulerJobSummary, 0, len(jobs))
	for _, job := range jobs {
		items = append(items, s.schedulerJobSummary(job))
	}
	return schedulerJobListResponse{Items: items}, true
}

func (s *schedulerHTTPServiceImpl) TriggerJob(ctx context.Context, jobID string) (schedulerJobTriggerResponse, *SystemHTTPError) {
	if s == nil || s.scheduler == nil {
		return schedulerJobTriggerResponse{}, missingSchedulerJobHTTPError("")
	}
	job, err := s.scheduler.Trigger(ctx, jobID)
	if err != nil {
		if errors.Is(err, scheduler.ErrJobNotFound) {
			return schedulerJobTriggerResponse{}, missingSchedulerJobHTTPError(jobID)
		}
		return schedulerJobTriggerResponse{}, InternalSystemHTTPError()
	}
	return schedulerJobTriggerResponse{
		JobID:     job.JobID,
		PluginID:  job.PluginID,
		Triggered: true,
	}, nil
}

func missingSchedulerJobHTTPError(jobID string) *SystemHTTPError {
	details := map[string]any{"resource_type": "scheduler_job"}
	if jobID != "" {
		details["job_id"] = jobID
	}
	return MissingSystemResourceHTTPError(details)
}
