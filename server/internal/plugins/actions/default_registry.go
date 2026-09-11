package actions

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/settings"
)

type registrar struct {
	kind    string
	factory func(Deps) ActionHandler
}

func NewDefaultRegistry(deps Deps) *Registry {
	registry := &Registry{handlers: make(map[string]ActionHandler)}
	for _, item := range defaultRegistrarItems() {
		registry.handlers[item.kind] = item.factory(deps)
	}
	return registry
}

func defaultRegistrarItems() []registrar {
	items := make([]registrar, 0, 16)
	items = append(items,
		schedulerCreateRegistrar(),
		secretReadRegistrar(),
		httpRequestRegistrar(),
		renderImageRegistrar(),
		messageSendRegistrar(),
		logWriteRegistrar(),
		pluginListRegistrar(),
		thirdPartyAccountReadRegistrar(),
		thirdPartyAccountValidateRegistrar(),
		thirdPartyResolveRegistrar(),
	)
	items = append(items, configRegistrars()...)
	items = append(items, governanceRegistrars()...)
	items = append(items, oneBotRegistrars()...)
	items = append(items, storageRegistrars()...)
	return items
}

func schedulerCreateRegistrar() registrar {
	return registrar{
		kind: "scheduler.create",
		factory: func(deps Deps) ActionHandler {
			return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
				return executeSchedulerCreate(ctx, deps, req)
			}
		},
	}
}

func executeSchedulerCreate(ctx context.Context, deps Deps, req ActionRequest) (map[string]any, error) {
	if deps.Permissions == nil || !deps.Permissions.PermissionDeclared(ctx, req.PluginID, "scheduler.create") {
		return nil, &plugins.Error{Code: errorcodes.PluginPermissionDenied, Message: "scheduler.create permission is not declared"}
	}
	if deps.Scheduler == nil {
		return nil, &plugins.Error{Code: errorcodes.PluginInternalError, Message: "scheduler engine is not available"}
	}

	payloadBytes, err := json.Marshal(req.Action.SchedulerPayload)
	if err != nil {
		return nil, &plugins.Error{Code: errorcodes.PluginInternalError, Message: "scheduler.create payload is invalid", Err: err}
	}
	job, err := deps.Scheduler(ctx, req.PluginID, req.Action.SchedulerTaskID, req.Action.SchedulerLogLabel, req.Action.SchedulerCron, payloadBytes)
	if err != nil {
		return nil, &plugins.Error{Code: errorcodes.PluginInternalError, Message: "scheduler.create failed", Err: err}
	}
	return map[string]any{
		"task_id":  job.JobID,
		"next_run": job.NextRun.UTC().Format(time.RFC3339),
	}, nil
}

func secretReadRegistrar() registrar {
	return registrar{
		kind: "secret.read",
		factory: func(deps Deps) ActionHandler {
			return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
				return executeSecretRead(ctx, deps, req)
			}
		},
	}
}

func executeSecretRead(ctx context.Context, deps Deps, req ActionRequest) (map[string]any, error) {
	if deps.Permissions == nil || !deps.Permissions.PermissionDeclared(ctx, req.PluginID, "secret.read") {
		return nil, &plugins.Error{Code: errorcodes.PluginPermissionDenied, Message: "secret.read permission is not declared"}
	}

	key := req.Action.SecretKey
	if !settings.ValidSecretKey(key) {
		return nil, &plugins.Error{Code: errorcodes.PluginProtocolViolation, Message: "secret.read key is required"}
	}
	if deps.Settings == nil {
		return nil, &plugins.Error{Code: errorcodes.PluginInternalError, Message: "secret.read store is not available"}
	}

	value, exists, err := deps.Settings.ReadSecret(ctx, req.PluginID, key)
	if err != nil {
		return nil, &plugins.Error{Code: errorcodes.PluginInternalError, Message: "secret.read failed", Err: err}
	}
	if !exists {
		return map[string]any{"key": key, "exists": false}, nil
	}
	return map[string]any{"key": key, "exists": true, "value": value}, nil
}

func configRegistrars() []registrar {
	return []registrar{
		{
			kind: "config.write",
			factory: func(deps Deps) ActionHandler {
				return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
					return executeConfigWrite(ctx, deps, req)
				}
			},
		},
	}
}

func executeConfigWrite(ctx context.Context, deps Deps, req ActionRequest) (map[string]any, error) {
	result, err := deps.Settings.Write(ctx, req.PluginID, req.Action.ConfigValues)
	if err != nil {
		var applyErr *settings.ApplyError
		if errors.As(err, &applyErr) {
			definition, _ := errorcodes.Lookup(errorcodes.PluginSettingsApplyFailed)
			return nil, &plugins.Error{Code: errorcodes.PluginSettingsApplyFailed, Message: definition.Message, Details: applyErr.Details(), Err: err}
		}
		if errors.Is(err, settings.ErrInvalidValues) {
			return nil, &plugins.Error{Code: errorcodes.PluginProtocolViolation, Message: "config.write values are invalid"}
		}
		return nil, &plugins.Error{Code: errorcodes.PluginInternalError, Message: "config.write failed", Err: err}
	}
	return map[string]any{"changed_keys": result.ChangedKeys}, nil
}
