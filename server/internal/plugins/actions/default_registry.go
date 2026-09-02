package actions

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins/pluginstore"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
)

type Metadata struct {
	Action             string
	Permission         string
	RequestSchema      string
	ResponseSchema     string
	RequiredPermission string
	ReadsSecret        bool
	WritesSecret       bool
	AccessesNetwork    bool
	WritesFile         bool
	AuditFields        []string
	ErrorCodes         []string
}

type registrar struct {
	metadata Metadata
	factory  func(Deps) ActionHandler
}

func (r registrar) RegisterActions(registry *Registry, deps Deps) {
	if registry == nil || r.factory == nil || r.metadata.Action == "" {
		return
	}
	registry.Register(r.metadata.Action, r.factory(deps))
}

func DefaultRegistrars() []Registrar {
	items := defaultRegistrarItems()
	result := make([]Registrar, 0, len(items))
	for _, item := range items {
		result = append(result, item)
	}
	return result
}

func NewDefaultRegistry(deps Deps) *Registry {
	return NewRegistryWithRegistrars(deps, DefaultRegistrars()...)
}

func DefaultMetadataList() []Metadata {
	registrars := defaultRegistrarItems()
	items := make([]Metadata, 0, len(registrars))
	for _, item := range registrars {
		metadata := item.metadata
		metadata.AuditFields = append([]string(nil), metadata.AuditFields...)
		metadata.ErrorCodes = append([]string(nil), metadata.ErrorCodes...)
		items = append(items, metadata)
	}
	return items
}

func commonErrorCodes(extra ...string) []string {
	codes := []string{
		"plugin.permission_denied",
		"plugin.internal_error",
		"plugin.protocol_violation",
	}
	return append(codes, extra...)
}

func defaultRegistrarItems() []registrar {
	items := []registrar{
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
	}
	items = append(items, configRegistrars()...)
	items = append(items, governanceRegistrars()...)
	items = append(items, oneBotRegistrars()...)
	items = append(items, storageRegistrars()...)
	return items
}

func schedulerCreateRegistrar() registrar {
	return registrar{
		metadata: Metadata{
			Action:         "scheduler.create",
			Permission:     "scheduler.create",
			RequestSchema:  "plugin-protocol.action_scheduler_create",
			ResponseSchema: "plugin-protocol.local_action_result",
			AuditFields:    []string{"plugin_id", "task_id", "cron"},
			ErrorCodes:     commonErrorCodes(),
		},
		factory: func(deps Deps) ActionHandler {
			return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
				return executeSchedulerCreate(ctx, deps, req)
			}
		},
	}
}

func executeSchedulerCreate(ctx context.Context, deps Deps, req ActionRequest) (map[string]any, error) {
	if deps.Permissions == nil || !deps.Permissions.PermissionDeclared(ctx, req.PluginID, "scheduler.create") {
		return nil, &pluginruntime.Error{Code: "plugin.permission_denied", Message: "scheduler.create permission is not declared"}
	}
	if deps.Scheduler == nil {
		return nil, &pluginruntime.Error{Code: "plugin.internal_error", Message: "scheduler engine is not available"}
	}

	payloadBytes, err := json.Marshal(req.Action.SchedulerPayload)
	if err != nil {
		return nil, &pluginruntime.Error{Code: "plugin.internal_error", Message: "scheduler.create payload is invalid", Err: err}
	}
	job, err := deps.Scheduler(ctx, req.PluginID, req.Action.SchedulerTaskID, req.Action.SchedulerLogLabel, req.Action.SchedulerCron, payloadBytes)
	if err != nil {
		return nil, &pluginruntime.Error{Code: "plugin.internal_error", Message: "scheduler.create failed", Err: err}
	}
	return map[string]any{
		"task_id":  job.JobID,
		"next_run": job.NextRun.UTC().Format(time.RFC3339),
	}, nil
}

var pluginSecretKeyPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9_.-]{0,126}[a-z0-9])?$`)

func secretReadRegistrar() registrar {
	return registrar{
		metadata: Metadata{
			Action:         "secret.read",
			Permission:     "secret.read",
			RequestSchema:  "plugin-protocol.action_secret_read",
			ResponseSchema: "plugin-protocol.local_action_result",
			ReadsSecret:    true,
			AuditFields:    []string{"plugin_id", "key", "exists"},
			ErrorCodes:     commonErrorCodes(),
		},
		factory: func(deps Deps) ActionHandler {
			return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
				return executeSecretRead(ctx, deps, req)
			}
		},
	}
}

func executeSecretRead(ctx context.Context, deps Deps, req ActionRequest) (map[string]any, error) {
	if deps.Permissions == nil || !deps.Permissions.PermissionDeclared(ctx, req.PluginID, "secret.read") {
		return nil, &pluginruntime.Error{Code: "plugin.permission_denied", Message: "secret.read permission is not declared"}
	}

	key := strings.TrimSpace(req.Action.SecretKey)
	if !isPluginSecretKey(key) {
		return nil, &pluginruntime.Error{Code: "plugin.protocol_violation", Message: "secret.read key is required"}
	}
	if deps.Secrets == nil {
		return nil, &pluginruntime.Error{Code: "plugin.internal_error", Message: "secret.read store is not available"}
	}

	value, exists, err := deps.Secrets.ReadPluginSecret(ctx, pluginSecretStorageKey(req.PluginID, key))
	if err != nil {
		return nil, &pluginruntime.Error{Code: "plugin.internal_error", Message: "secret.read failed", Err: err}
	}
	if !exists {
		return map[string]any{"key": key, "exists": false}, nil
	}
	return map[string]any{"key": key, "exists": true, "value": value}, nil
}

func pluginSecretStorageKey(pluginID, key string) string {
	return "plugin:" + strings.TrimSpace(pluginID) + ":secret:" + strings.TrimSpace(key)
}

func isPluginSecretKey(key string) bool {
	return pluginSecretKeyPattern.MatchString(strings.TrimSpace(key))
}

func configRegistrars() []registrar {
	return []registrar{
		{
			metadata: Metadata{
				Action:         "config.write",
				Permission:     "config.write",
				RequestSchema:  "plugin-protocol.action_config_write",
				ResponseSchema: "plugin-protocol.local_action_result",
				AuditFields:    []string{"plugin_id", "changed_keys"},
				ErrorCodes:     commonErrorCodes(),
			},
			factory: func(deps Deps) ActionHandler {
				return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
					return executeConfigWrite(ctx, deps, req)
				}
			},
		},
	}
}

func executeConfigWrite(ctx context.Context, deps Deps, req ActionRequest) (map[string]any, error) {
	if deps.PluginConfig == nil {
		return nil, &pluginruntime.Error{Code: "plugin.internal_error", Message: "config.write repository is not available"}
	}

	changedKeys, err := deps.PluginConfig.Write(ctx, req.PluginID, req.Action.ConfigValues)
	if err != nil {
		return nil, &pluginruntime.Error{Code: "plugin.internal_error", Message: "config.write failed", Err: err}
	}
	settings, readErr := deps.PluginConfig.ReadAll(ctx, req.PluginID)
	if readErr != nil {
		return nil, &pluginruntime.Error{Code: "plugin.internal_error", Message: "config.write failed", Err: readErr}
	}
	if deps.Plugins != nil {
		if snapshot, ok := deps.Plugins.Get(req.PluginID); ok {
			settings = pluginstore.MergeValues(snapshot.DefaultConfig, settings)
		}
	}
	if len(changedKeys) > 0 && deps.RefreshCommands != nil {
		deps.RefreshCommands(ctx, req.PluginID, settings)
	}
	dispatchConfigChanged(ctx, req.PluginID, settings, changedKeys, deps.Dispatcher, deps.Logger)
	return map[string]any{"changed_keys": changedKeys}, nil
}

func dispatchConfigChanged(ctx context.Context, pluginID string, config map[string]any, changedKeys []string, dispatcher ConfigChangeDispatcher, logger interface {
	Warn(string, ...any)
}) {
	if dispatcher == nil {
		return
	}
	result := dispatcher(ctx, pluginID, config, changedKeys)
	if result.Delivered || logger == nil {
		return
	}
	logger.Warn(
		"插件 "+pluginID+" 的配置变更事件未能投递到运行时；新配置可能要等插件下次启动后生效。结果："+string(result.Outcome)+"，错误码："+result.ErrorCode,
		"component", "app",
		"plugin_id", pluginID,
		"outcome", result.Outcome,
		"error_code", result.ErrorCode,
	)
}
