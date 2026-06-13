package app

import (
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/localaction"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/render"
)

type configHTTPHandlers struct {
	config configHTTPService
}

type configHTTPService interface {
	currentConfigDocument() configResponse
	updateConfigDocument(map[string]any) (configUpdateResponse, error)
	applyHotReloadableFields(config.Config) configApplyEffects
}

type configHTTPServiceImpl struct {
	state            *appRuntimeState
	logs             *logging.Stream
	logRepository    logging.Repository
	renderer         renderRuntimeConfigUpdater
	pluginLogLimiter *localaction.PluginLogLimiter
	outboundLimiter  interface{ ApplyConfig(config.Config) }
	protocol         configProtocolReloader
	eventIngress     *eventIngressService
	blacklistRepo    permission.BlacklistRepository
}

func newConfigHTTPHandlers(deps configHTTPDeps) *configHTTPHandlers {
	return &configHTTPHandlers{config: newConfigHTTPService(deps)}
}

func newConfigHTTPService(deps configHTTPDeps) *configHTTPServiceImpl {
	var protocol configProtocolReloader
	if deps.protocol != nil {
		protocol = deps.protocol
	}
	return &configHTTPServiceImpl{
		state:            deps.state,
		logs:             deps.logs,
		logRepository:    deps.logRepository,
		renderer:         deps.renderer,
		pluginLogLimiter: deps.pluginLogLimiter,
		outboundLimiter:  deps.outboundLimiter,
		protocol:         protocol,
		eventIngress:     deps.eventIngress,
		blacklistRepo:    deps.blacklistRepo,
	}
}

type renderRuntimeConfigUpdater interface {
	UpdateRuntimeConfig(render.RuntimeConfig)
}

type configProtocolReloader interface {
	ApplyConfigReload(config.Config) error
	PublishSnapshot()
}
