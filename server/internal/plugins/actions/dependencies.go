package actions

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions/configaction"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions/governanceaction"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions/httpaction"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions/onebot"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions/renderaction"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions/scheduleraction"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions/secretaction"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions/webhookaction"
)

type GrantView interface {
	CapabilityGranted(context.Context, string, string) bool
	StorageRootGranted(context.Context, string, string) bool
	GrantedHTTPHosts(context.Context, string) []string
	GrantedWebhookScope(context.Context, string, string) (plugins.WebhookScope, bool)
	ListPluginSnapshots() []plugins.Snapshot
}

type WebhookGateway = webhookaction.Gateway

type PluginConfigRepository = configaction.Repository

type OneBotAdapter = onebot.Adapter

type ConfigChangeDispatchResult = configaction.DispatchResult

type ConfigChangeDispatcher = configaction.Dispatcher

type ScheduledTask = scheduleraction.Task

type SchedulerCreateFunc = scheduleraction.CreateFunc

type SecretReader = secretaction.Reader

type Renderer = renderaction.Renderer

type RenderImageRequest = renderaction.ImageRequest

type RenderPluginContext = renderaction.PluginContext

type RenderImageResult = renderaction.ImageResult

type RenderTemplateError = renderaction.TemplateError

type GovernanceService = governanceaction.Service

type HTTPCredentialInjector = httpaction.CredentialInjector
