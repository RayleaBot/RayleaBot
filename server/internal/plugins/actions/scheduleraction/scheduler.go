package scheduleraction

import (
	"context"
	"encoding/json"
	"time"

	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/action"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
)

type CapabilityView interface {
	CapabilityDeclared(context.Context, string, string) bool
}

type Task struct {
	JobID   string
	NextRun time.Time
}

type CreateFunc func(context.Context, string, string, string, string, []byte) (Task, error)

type Request struct {
	PluginID     string
	Action       runtimeaction.Action
	Capabilities CapabilityView
	Create       CreateFunc
}

func ExecuteCreate(ctx context.Context, req Request) (map[string]any, error) {
	if req.Capabilities == nil || !req.Capabilities.CapabilityDeclared(ctx, req.PluginID, "scheduler.create") {
		return nil, &runtimemanager.Error{
			Code:    "plugin.capability_violation",
			Message: "scheduler.create capability is not declared",
		}
	}
	if req.Create == nil {
		return nil, &runtimemanager.Error{
			Code:    "plugin.internal_error",
			Message: "scheduler engine is not available",
		}
	}

	payloadBytes, err := json.Marshal(req.Action.SchedulerPayload)
	if err != nil {
		return nil, &runtimemanager.Error{
			Code:    "plugin.internal_error",
			Message: "scheduler.create payload is invalid",
			Err:     err,
		}
	}
	job, err := req.Create(ctx, req.PluginID, req.Action.SchedulerTaskID, req.Action.SchedulerLogLabel, req.Action.SchedulerCron, payloadBytes)
	if err != nil {
		return nil, &runtimemanager.Error{Code: "plugin.internal_error", Message: "scheduler.create failed", Err: err}
	}
	return map[string]any{
		"task_id":  job.JobID,
		"next_run": job.NextRun.UTC().Format(time.RFC3339),
	}, nil
}
