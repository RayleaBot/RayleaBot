package defaultmodules

import (
	"context"
	"encoding/json"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
)

func init() {
	register(Metadata{
		Action:         "scheduler.create",
		Capability:     "scheduler.create",
		RequestSchema:  "plugin-protocol.action_scheduler_create",
		ResponseSchema: "plugin-protocol.local_action_result",
		AuditFields:    []string{"plugin_id", "task_id", "cron"},
		ErrorCodes:     commonErrorCodes(),
	}, func(deps actions.Deps) actions.ActionHandler {
		return func(ctx context.Context, req actions.ActionRequest) (map[string]any, error) {
			return executeSchedulerCreate(ctx, deps, req)
		}
	})
}

func executeSchedulerCreate(ctx context.Context, deps actions.Deps, req actions.ActionRequest) (map[string]any, error) {
	if deps.Capabilities == nil || !deps.Capabilities.CapabilityDeclared(ctx, req.PluginID, "scheduler.create") {
		return nil, &runtimemanager.Error{Code: "plugin.capability_violation", Message: "scheduler.create capability is not declared"}
	}
	if deps.Scheduler == nil {
		return nil, &runtimemanager.Error{Code: "plugin.internal_error", Message: "scheduler engine is not available"}
	}

	payloadBytes, err := json.Marshal(req.Action.SchedulerPayload)
	if err != nil {
		return nil, &runtimemanager.Error{Code: "plugin.internal_error", Message: "scheduler.create payload is invalid", Err: err}
	}
	job, err := deps.Scheduler(ctx, req.PluginID, req.Action.SchedulerTaskID, req.Action.SchedulerLogLabel, req.Action.SchedulerCron, payloadBytes)
	if err != nil {
		return nil, &runtimemanager.Error{Code: "plugin.internal_error", Message: "scheduler.create failed", Err: err}
	}
	return map[string]any{
		"task_id":  job.JobID,
		"next_run": job.NextRun.UTC().Format(time.RFC3339),
	}, nil
}
