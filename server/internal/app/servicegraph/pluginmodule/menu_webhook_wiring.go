package pluginmodule

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/app/eventstack"
	appplatform "github.com/RayleaBot/RayleaBot/server/internal/app/platform"
	"github.com/RayleaBot/RayleaBot/server/internal/app/pluginstack"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/outbound"
	menuext "github.com/RayleaBot/RayleaBot/server/internal/extensions/menu"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/lifecycle"
	pluginwebhook "github.com/RayleaBot/RayleaBot/server/internal/plugins/webhook"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
)

func buildBuiltinMenuService(runtimeState RuntimeState, pluginStack pluginstack.State, eventStack eventstack.State, renderer *renderservice.Service) *menuext.Service {
	return menuext.New(menuext.Deps{
		CurrentConfig: runtimeState.CurrentConfig,
		Plugins:       pluginStack.Plugins,
		Renderer:      renderer,
		Sender:        eventStack.OutboundSender,
		WaitOutbound: func(ctx context.Context, request outbound.MessageLimitRequest) error {
			if eventStack.OutboundLimiter == nil {
				return nil
			}
			return eventStack.OutboundLimiter.Wait(ctx, request)
		},
		Logger: runtimeState.RuntimeLogger(),
	})
}

func buildPluginWebhookGateway(
	runtimeState RuntimeState,
	platform appplatform.State,
	pluginStack pluginstack.State,
	eventStack eventstack.State,
	lifecycle *pluginservice.Controller,
	capabilityView pluginwebhook.CapabilityView,
) *pluginwebhook.Service {
	return pluginwebhook.New(pluginwebhook.Deps{
		CurrentConfig: runtimeState.CurrentConfig,
		Logger:        runtimeState.RuntimeLogger(),
		Registry:      pluginStack.Webhooks,
		Secrets:       platform.Secrets,
		Plugins:       pluginStack.Plugins,
		Dispatcher:    eventStack.Dispatcher,
		Runtime:       lifecycle,
		Capabilities:  capabilityView,
	})
}
