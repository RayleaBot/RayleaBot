package actions

import (
	"context"
	"encoding/json"
	"time"

	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/action"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
)

func (s *Service) executeSchedulerCreate(ctx context.Context, pluginID string, action runtimeaction.Action) (map[string]any, error) {
	if s == nil || s.grants == nil || !s.grants.CapabilityGranted(ctx, pluginID, "scheduler.create") {
		return nil, &runtimemanager.Error{
			Code:    "permission.scope_violation",
			Message: "scheduler.create capability is not granted",
		}
	}
	if s.scheduler == nil {
		return nil, &runtimemanager.Error{
			Code:    "plugin.internal_error",
			Message: "scheduler engine is not available",
		}
	}

	payloadBytes, err := json.Marshal(action.SchedulerPayload)
	if err != nil {
		return nil, &runtimemanager.Error{
			Code:    "plugin.internal_error",
			Message: "scheduler.create payload is invalid",
			Err:     err,
		}
	}
	job, err := s.scheduler(ctx, pluginID, action.SchedulerTaskID, action.SchedulerLogLabel, action.SchedulerCron, payloadBytes)
	if err != nil {
		return nil, &runtimemanager.Error{Code: "plugin.internal_error", Message: "scheduler.create failed", Err: err}
	}
	return map[string]any{
		"task_id":  job.JobID,
		"next_run": job.NextRun.UTC().Format(time.RFC3339),
	}, nil
}
