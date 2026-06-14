package configruntime

import (
	"log/slog"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/localaction"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
)

type Service struct {
	currentConfig    func() config.Config
	currentSummary   func() config.Summary
	setConfig        func(config.Config)
	setSummary       func(config.Summary)
	logger           *slog.Logger
	logLevel         *logging.LevelController
	logs             *logging.Stream
	logRepository    logging.Repository
	renderer         renderRuntimeConfigUpdater
	pluginLogLimiter *localaction.PluginLogLimiter
	outboundLimiter  interface{ ApplyConfig(config.Config) }
	protocol         configProtocolReloader
	eventIngress     configEventIngress
}

type Deps struct {
	CurrentConfig    func() config.Config
	CurrentSummary   func() config.Summary
	SetConfig        func(config.Config)
	SetSummary       func(config.Summary)
	Logger           *slog.Logger
	LogLevel         *logging.LevelController
	Logs             *logging.Stream
	LogRepository    logging.Repository
	Renderer         renderRuntimeConfigUpdater
	PluginLogLimiter *localaction.PluginLogLimiter
	OutboundLimiter  interface{ ApplyConfig(config.Config) }
	Protocol         configProtocolReloader
	EventIngress     configEventIngress
}

func NewService(deps Deps) *Service {
	var protocol configProtocolReloader
	if deps.Protocol != nil {
		protocol = deps.Protocol
	}
	return &Service{
		currentConfig:    deps.CurrentConfig,
		currentSummary:   deps.CurrentSummary,
		setConfig:        deps.SetConfig,
		setSummary:       deps.SetSummary,
		logger:           deps.Logger,
		logLevel:         deps.LogLevel,
		logs:             deps.Logs,
		logRepository:    deps.LogRepository,
		renderer:         deps.Renderer,
		pluginLogLimiter: deps.PluginLogLimiter,
		outboundLimiter:  deps.OutboundLimiter,
		protocol:         protocol,
		eventIngress:     deps.EventIngress,
	}
}

type renderRuntimeConfigUpdater interface {
	UpdateRuntimeConfig(renderservice.RuntimeConfig)
}

type configProtocolReloader interface {
	ApplyConfigReload(config.Config) error
	PublishSnapshot()
}

type configEventIngress interface {
	UpdateConfig(config.Config)
}
