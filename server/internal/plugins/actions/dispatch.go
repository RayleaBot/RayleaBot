package actions

import (
	"context"
	"log/slog"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions/configaction"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions/governanceaction"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions/httpaction"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions/logaction"
	localonebot "github.com/RayleaBot/RayleaBot/server/internal/plugins/actions/onebot"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions/pluginlist"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions/renderaction"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions/scheduleraction"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions/secretaction"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions/storageaction"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions/webhookaction"
	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/action"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
)

type ActionRequest struct {
	PluginID    string
	RequestID   string
	Action      runtimeaction.Action
	ParentEvent runtimeprotocol.Event
}

type ActionHandler func(context.Context, ActionRequest) (map[string]any, error)

type handlerFactory func(registryDeps) ActionHandler

type registryDeps struct {
	currentConfig    func() config.Config
	logger           *slog.Logger
	redactText       func(string) string
	capabilities     CapabilityView
	pluginConfig     PluginConfigRepository
	pluginFiles      storageaction.FileStore
	pluginKV         storageaction.KVRepository
	secrets          SecretReader
	scheduler        SchedulerCreateFunc
	dispatcher       ConfigChangeDispatcher
	renderer         Renderer
	adapter          OneBotAdapter
	pluginLogLimiter *PluginLogLimiter
	governance       GovernanceService
	httpCredentials  HTTPCredentialInjector
	runtimeHooks     *runtimeHooks
}

type Registry struct {
	handlers map[string]ActionHandler
}

func NewRegistry() *Registry {
	return &Registry{handlers: make(map[string]ActionHandler)}
}

func DefaultRegistry() *Registry {
	return defaultRegistry(registryDeps{})
}

func defaultRegistry(deps registryDeps) *Registry {
	registry := NewRegistry()
	for _, module := range defaultActionModules {
		module.RegisterActions(registry, deps)
	}
	return registry
}

func (r *Registry) Register(kind string, handler ActionHandler) {
	if r == nil || kind == "" || handler == nil {
		return
	}
	r.handlers[kind] = handler
}

func (r *Registry) Dispatch(ctx context.Context, req ActionRequest) (map[string]any, bool, error) {
	if r == nil {
		return nil, false, nil
	}
	handler, ok := r.handlers[req.Action.Kind]
	if !ok {
		return nil, false, nil
	}
	result, err := handler(ctx, req)
	return result, true, err
}

type actionModule struct {
	name     string
	handlers map[string]handlerFactory
}

func (m actionModule) RegisterActions(registry *Registry, deps registryDeps) {
	if registry == nil {
		return
	}
	for kind, handler := range m.handlers {
		registry.Register(kind, handler(deps))
	}
}

type oneBotActionModule struct{}

func (oneBotActionModule) RegisterActions(registry *Registry, deps registryDeps) {
	if registry == nil {
		return
	}
	for kind := range localonebot.Registry() {
		registry.Register(kind, oneBotHandler(deps))
	}
}

var defaultActionModules = []interface {
	RegisterActions(*Registry, registryDeps)
}{
	logActionModule,
	storageActionModule,
	configActionModule,
	pluginActionModule,
	secretActionModule,
	governanceActionModule,
	httpActionModule,
	schedulerActionModule,
	webhookActionModule,
	renderActionModule,
	oneBotActionModule{},
}

var logActionModule = actionModule{
	name: "log",
	handlers: map[string]handlerFactory{
		"logger.write": func(deps registryDeps) ActionHandler {
			return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
				return logaction.Execute(ctx, logaction.Request{
					PluginID:     req.PluginID,
					RequestID:    req.RequestID,
					Action:       req.Action,
					Capabilities: serviceCapabilities(deps),
					Logger:       serviceLogger(deps),
					RedactText:   serviceRedactor(deps),
					Limiter:      servicePluginLogLimiter(deps),
				})
			}
		},
	},
}

var storageActionModule = actionModule{
	name: "storage",
	handlers: map[string]handlerFactory{
		"storage.kv": func(deps registryDeps) ActionHandler {
			return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
				return storageaction.ExecuteKV(ctx, storageaction.Request{
					PluginID:     req.PluginID,
					Action:       req.Action,
					Config:       serviceConfig(deps),
					Capabilities: serviceCapabilities(deps),
					KV:           servicePluginKV(deps),
				})
			}
		},
		"storage.file": func(deps registryDeps) ActionHandler {
			return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
				return storageaction.ExecuteFile(ctx, storageaction.Request{
					PluginID:     req.PluginID,
					Action:       req.Action,
					Config:       serviceConfig(deps),
					Capabilities: serviceCapabilities(deps),
					Files:        servicePluginFiles(deps),
				})
			}
		},
	},
}

var configActionModule = actionModule{
	name: "config",
	handlers: map[string]handlerFactory{
		"config.read": func(deps registryDeps) ActionHandler {
			return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
				return configaction.ExecuteRead(ctx, configaction.Request{
					PluginID:     req.PluginID,
					Action:       req.Action,
					Capabilities: serviceCapabilities(deps),
					Repository:   servicePluginConfig(deps),
				})
			}
		},
		"config.write": func(deps registryDeps) ActionHandler {
			return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
				return configaction.ExecuteWrite(ctx, configaction.Request{
					PluginID:        req.PluginID,
					Action:          req.Action,
					Capabilities:    serviceCapabilities(deps),
					Repository:      servicePluginConfig(deps),
					RefreshCommands: serviceRefreshCommands(deps),
					Dispatcher:      serviceConfigDispatcher(deps),
					Logger:          serviceLogger(deps),
				})
			}
		},
	},
}

var pluginActionModule = actionModule{
	name: "plugin",
	handlers: map[string]handlerFactory{
		"plugin.list": func(deps registryDeps) ActionHandler {
			return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
				return pluginlist.Execute(ctx, pluginlist.Request{
					PluginID:      req.PluginID,
					Action:        req.Action,
					ParentEvent:   req.ParentEvent,
					Capabilities:  serviceCapabilities(deps),
					CurrentConfig: serviceConfigProvider(deps),
				})
			}
		},
	},
}

var secretActionModule = actionModule{
	name: "secret",
	handlers: map[string]handlerFactory{
		"secret.read": func(deps registryDeps) ActionHandler {
			return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
				return secretaction.ExecuteRead(ctx, secretaction.Request{
					PluginID:     req.PluginID,
					Action:       req.Action,
					Capabilities: serviceCapabilities(deps),
					Reader:       serviceSecrets(deps),
				})
			}
		},
	},
}

var governanceActionModule = actionModule{
	name: "governance",
	handlers: map[string]handlerFactory{
		"governance.blacklist.read": func(deps registryDeps) ActionHandler {
			return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
				return governanceaction.ExecuteBlacklistRead(ctx, governanceRequest(deps, req))
			}
		},
		"governance.blacklist.write": func(deps registryDeps) ActionHandler {
			return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
				return governanceaction.ExecuteBlacklistWrite(ctx, governanceRequest(deps, req))
			}
		},
		"governance.whitelist.read": func(deps registryDeps) ActionHandler {
			return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
				return governanceaction.ExecuteWhitelistRead(ctx, governanceRequest(deps, req))
			}
		},
		"governance.whitelist.write": func(deps registryDeps) ActionHandler {
			return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
				return governanceaction.ExecuteWhitelistWrite(ctx, governanceRequest(deps, req))
			}
		},
		"governance.command_policy.read": func(deps registryDeps) ActionHandler {
			return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
				return governanceaction.ExecuteCommandPolicyRead(ctx, governanceRequest(deps, req))
			}
		},
	},
}

var httpActionModule = actionModule{
	name: "http",
	handlers: map[string]handlerFactory{
		"http.request": func(deps registryDeps) ActionHandler {
			return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
				return httpaction.Execute(ctx, httpaction.Request{
					PluginID:           req.PluginID,
					Action:             req.Action,
					Config:             serviceConfig(deps),
					Capabilities:       serviceCapabilities(deps),
					CredentialInjector: serviceHTTPCredentials(deps),
				})
			}
		},
	},
}

var schedulerActionModule = actionModule{
	name: "scheduler",
	handlers: map[string]handlerFactory{
		"scheduler.create": func(deps registryDeps) ActionHandler {
			return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
				return scheduleraction.ExecuteCreate(ctx, scheduleraction.Request{
					PluginID:     req.PluginID,
					Action:       req.Action,
					Capabilities: serviceCapabilities(deps),
					Create:       serviceScheduler(deps),
				})
			}
		},
	},
}

var webhookActionModule = actionModule{
	name: "webhook",
	handlers: map[string]handlerFactory{
		"event.expose_webhook": func(deps registryDeps) ActionHandler {
			return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
				return webhookaction.ExecuteExpose(ctx, webhookaction.Request{
					PluginID: req.PluginID,
					Action:   req.Action,
					Gateway:  serviceWebhookGateway(deps),
				})
			}
		},
	},
}

var renderActionModule = actionModule{
	name: "render",
	handlers: map[string]handlerFactory{
		"render.image": func(deps registryDeps) ActionHandler {
			return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
				return renderaction.ExecuteImage(ctx, renderaction.Request{
					PluginID:      req.PluginID,
					Action:        req.Action,
					ParentEvent:   req.ParentEvent,
					Capabilities:  serviceCapabilities(deps),
					Renderer:      serviceRenderer(deps),
					CurrentConfig: serviceConfigProvider(deps),
				})
			}
		},
	},
}

var baseActionHandlers = actionHandlersFromModules(defaultActionModules)

func actionHandlersFromModules(modules []interface {
	RegisterActions(*Registry, registryDeps)
}) map[string]handlerFactory {
	handlers := map[string]handlerFactory{}
	for _, module := range modules {
		actionModule, ok := module.(actionModule)
		if !ok {
			continue
		}
		for kind, handler := range actionModule.handlers {
			handlers[kind] = handler
		}
	}
	return handlers
}

func (s *Service) Execute(ctx context.Context, pluginID, requestID string, action runtimeaction.Action, parentEvent runtimeprotocol.Event) (map[string]any, error) {
	if s != nil && s.actionRegistry != nil {
		result, handled, err := s.actionRegistry.Dispatch(ctx, ActionRequest{
			PluginID:    pluginID,
			RequestID:   requestID,
			Action:      action,
			ParentEvent: parentEvent,
		})
		if handled {
			return result, err
		}
	}
	return nil, &runtimemanager.Error{
		Code:    "plugin.protocol_violation",
		Message: "received unsupported local action kind",
	}
}

func oneBotHandler(deps registryDeps) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
		return localonebot.Execute(ctx, localonebot.Request{
			PluginID:     req.PluginID,
			Action:       req.Action,
			Capabilities: serviceCapabilities(deps),
			Adapter:      serviceAdapter(deps),
		})
	}
}

func governanceRequest(deps registryDeps, req ActionRequest) governanceaction.Request {
	return governanceaction.Request{
		PluginID:     req.PluginID,
		Action:       req.Action,
		Capabilities: serviceCapabilities(deps),
		Service:      serviceGovernance(deps),
	}
}

func serviceConfigProvider(deps registryDeps) func() config.Config {
	if deps.currentConfig == nil {
		return nil
	}
	return deps.currentConfig
}

func serviceConfig(deps registryDeps) config.Config {
	if deps.currentConfig == nil {
		return config.Config{}
	}
	return deps.currentConfig()
}

func serviceLogger(deps registryDeps) *slog.Logger {
	return deps.logger
}

func serviceRedactor(deps registryDeps) func(string) string {
	return deps.redactText
}

func serviceCapabilities(deps registryDeps) CapabilityView {
	return deps.capabilities
}

func servicePluginConfig(deps registryDeps) PluginConfigRepository {
	return deps.pluginConfig
}

func servicePluginFiles(deps registryDeps) storageaction.FileStore {
	return deps.pluginFiles
}

func servicePluginKV(deps registryDeps) storageaction.KVRepository {
	return deps.pluginKV
}

func serviceSecrets(deps registryDeps) SecretReader {
	return deps.secrets
}

func serviceScheduler(deps registryDeps) SchedulerCreateFunc {
	return deps.scheduler
}

func serviceConfigDispatcher(deps registryDeps) ConfigChangeDispatcher {
	return deps.dispatcher
}

func serviceRenderer(deps registryDeps) Renderer {
	return deps.renderer
}

func serviceAdapter(deps registryDeps) OneBotAdapter {
	return deps.adapter
}

func serviceWebhookGateway(deps registryDeps) WebhookGateway {
	if deps.runtimeHooks == nil {
		return nil
	}
	return deps.runtimeHooks.webhookGateway
}

func servicePluginLogLimiter(deps registryDeps) *PluginLogLimiter {
	return deps.pluginLogLimiter
}

func serviceGovernance(deps registryDeps) GovernanceService {
	return deps.governance
}

func serviceHTTPCredentials(deps registryDeps) HTTPCredentialInjector {
	return deps.httpCredentials
}

func serviceRefreshCommands(deps registryDeps) func(context.Context, string, map[string]any) {
	if deps.runtimeHooks == nil {
		return nil
	}
	return deps.runtimeHooks.refreshCommands
}
