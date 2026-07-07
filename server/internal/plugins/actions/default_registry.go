package actions

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
)

type Metadata struct {
	Action             string
	Capability         string
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
		"plugin.capability_violation",
		"plugin.internal_error",
		"plugin.protocol_violation",
	}
	return append(codes, extra...)
}

func defaultRegistrarItems() []registrar {
	items := []registrar{
		webhookExposeRegistrar(),
		schedulerCreateRegistrar(),
		secretReadRegistrar(),
		httpRequestRegistrar(),
		renderImageRegistrar(),
		logWriteRegistrar(),
		pluginListRegistrar(),
		thirdPartyAccountReadRegistrar(),
	}
	items = append(items, configRegistrars()...)
	items = append(items, governanceRegistrars()...)
	items = append(items, oneBotRegistrars()...)
	items = append(items, storageRegistrars()...)
	return items
}

func webhookExposeRegistrar() registrar {
	return registrar{
		metadata: Metadata{
			Action:          "event.expose_webhook",
			Capability:      "event.expose_webhook",
			RequestSchema:   "plugin-protocol.action_event_expose_webhook",
			ResponseSchema:  "plugin-protocol.local_action_result",
			AccessesNetwork: true,
			AuditFields:     []string{"plugin_id", "route_id"},
			ErrorCodes:      commonErrorCodes("platform.invalid_request"),
		},
		factory: func(deps Deps) ActionHandler {
			return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
				return executeWebhookExpose(ctx, deps, req)
			}
		},
	}
}

func executeWebhookExpose(ctx context.Context, deps Deps, req ActionRequest) (map[string]any, error) {
	if deps.WebhookGateway == nil || deps.WebhookGateway() == nil {
		return nil, &pluginruntime.Error{Code: "plugin.internal_error", Message: "webhook gateway is not available"}
	}
	return deps.WebhookGateway().Expose(ctx, req.PluginID, req.Action)
}

func schedulerCreateRegistrar() registrar {
	return registrar{
		metadata: Metadata{
			Action:         "scheduler.create",
			Capability:     "scheduler.create",
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
	if deps.Capabilities == nil || !deps.Capabilities.CapabilityDeclared(ctx, req.PluginID, "scheduler.create") {
		return nil, &pluginruntime.Error{Code: "plugin.capability_violation", Message: "scheduler.create capability is not declared"}
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
			Capability:     "secret.read",
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
	if deps.Capabilities == nil || !deps.Capabilities.CapabilityDeclared(ctx, req.PluginID, "secret.read") {
		return nil, &pluginruntime.Error{Code: "plugin.capability_violation", Message: "secret.read capability is not declared"}
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
				Action:         "config.read",
				Capability:     "config.read",
				RequestSchema:  "plugin-protocol.action_config_read",
				ResponseSchema: "plugin-protocol.local_action_result",
				AuditFields:    []string{"plugin_id", "keys"},
				ErrorCodes:     commonErrorCodes(),
			},
			factory: func(deps Deps) ActionHandler {
				return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
					return executeConfigRead(ctx, deps, req)
				}
			},
		},
		{
			metadata: Metadata{
				Action:         "config.write",
				Capability:     "config.write",
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

func executeConfigRead(ctx context.Context, deps Deps, req ActionRequest) (map[string]any, error) {
	if deps.Capabilities == nil || !deps.Capabilities.CapabilityDeclared(ctx, req.PluginID, "config.read") {
		return nil, &pluginruntime.Error{Code: "plugin.capability_violation", Message: "config.read capability is not declared"}
	}
	if deps.PluginConfig == nil {
		return nil, &pluginruntime.Error{Code: "plugin.internal_error", Message: "config.read repository is not available"}
	}
	values, err := deps.PluginConfig.Read(ctx, req.PluginID, req.Action.ConfigKeys)
	if err != nil {
		return nil, &pluginruntime.Error{Code: "plugin.internal_error", Message: "config.read failed", Err: err}
	}
	return map[string]any{"values": values}, nil
}

func executeConfigWrite(ctx context.Context, deps Deps, req ActionRequest) (map[string]any, error) {
	if deps.Capabilities == nil || !deps.Capabilities.CapabilityDeclared(ctx, req.PluginID, "config.write") {
		return nil, &pluginruntime.Error{Code: "plugin.capability_violation", Message: "config.write capability is not declared"}
	}
	if deps.PluginConfig == nil {
		return nil, &pluginruntime.Error{Code: "plugin.internal_error", Message: "config.write repository is not available"}
	}

	changedKeys, err := deps.PluginConfig.Write(ctx, req.PluginID, req.Action.ConfigValues)
	if err != nil {
		return nil, &pluginruntime.Error{Code: "plugin.internal_error", Message: "config.write failed", Err: err}
	}
	if len(changedKeys) > 0 && deps.RefreshCommands != nil {
		settings, readErr := deps.PluginConfig.ReadAll(ctx, req.PluginID)
		if readErr != nil {
			return nil, &pluginruntime.Error{Code: "plugin.internal_error", Message: "config.write failed", Err: readErr}
		}
		deps.RefreshCommands(ctx, req.PluginID, settings)
	}
	dispatchConfigChanged(ctx, req.PluginID, deps.Dispatcher, deps.Logger)
	return map[string]any{"changed_keys": changedKeys}, nil
}

func dispatchConfigChanged(ctx context.Context, pluginID string, dispatcher ConfigChangeDispatcher, logger interface {
	Warn(string, ...any)
}) {
	if dispatcher == nil {
		return
	}
	result := dispatcher(ctx, pluginID)
	if result.Delivered || logger == nil {
		return
	}
	logger.Warn(
		"插件 "+pluginID+" 配置变更事件未能投递到运行时",
		"component", "app",
		"plugin_id", pluginID,
		"outcome", result.Outcome,
		"error_code", result.ErrorCode,
	)
}
