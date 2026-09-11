package app

import (
	"log/slog"

	adapterservice "github.com/RayleaBot/RayleaBot/server/internal/bot/adapters"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/chatpolicy"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	configruntime "github.com/RayleaBot/RayleaBot/server/internal/config/runtime"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/secrets"
	localaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	"github.com/RayleaBot/RayleaBot/server/internal/render"
)

type configRuntimeState interface {
	CurrentConfig() config.Config
	CurrentSummary() config.Summary
	SetConfig(config.Config)
	SetSummary(config.Summary)
	RuntimeLogger() *slog.Logger
	RuntimeLogLevel() *logging.LevelController
	RepoRoot() string
	AddRedactionValues(...string)
}

type configServiceDeps struct {
	EffectiveTimezone func() string
	Runtime           configRuntimeState
	Logs              *logging.Stream
	LogRepository     logging.Repository
	Renderer          *render.Service
	PluginLogLimiter  *localaction.PluginLogLimiter
	OutboundLimiter   interface{ ApplyConfig(config.Config) }
	AccountValidation interface{ ApplyConfig(config.Config) }
	Protocol          *adapterservice.Service
	EventIngress      *chatpolicy.Ingress
	Secrets           secrets.Store
}

func newConfigService(deps configServiceDeps) *configruntime.Service {
	return configruntime.NewService(configruntime.Deps{
		EffectiveTimezone:  deps.EffectiveTimezone,
		CurrentConfig:      deps.Runtime.CurrentConfig,
		CurrentSummary:     deps.Runtime.CurrentSummary,
		SetConfig:          deps.Runtime.SetConfig,
		SetSummary:         deps.Runtime.SetSummary,
		Logger:             deps.Runtime.RuntimeLogger(),
		LogLevel:           deps.Runtime.RuntimeLogLevel(),
		Logs:               deps.Logs,
		LogRepository:      deps.LogRepository,
		AddRedactionValues: deps.Runtime.AddRedactionValues,
		Renderer:           deps.Renderer,
		PluginLogLimiter:   deps.PluginLogLimiter,
		OutboundLimiter:    deps.OutboundLimiter,
		AccountValidation:  deps.AccountValidation,
		Protocol:           deps.Protocol,
		EventIngress:       deps.EventIngress,
		Secrets:            deps.Secrets,
	})
}
