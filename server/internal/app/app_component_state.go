package app

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/app/eventstack"
	"github.com/RayleaBot/RayleaBot/server/internal/app/httpwire"
	appplatform "github.com/RayleaBot/RayleaBot/server/internal/app/platform"
	"github.com/RayleaBot/RayleaBot/server/internal/app/pluginstack"
	"github.com/RayleaBot/RayleaBot/server/internal/app/renderstack"
	"github.com/RayleaBot/RayleaBot/server/internal/app/servicegraph"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
)

type appPlatform = appplatform.State
type schedulerTriggerProxy = appplatform.TriggerProxy
type appPlugins = pluginstack.State
type appRender = renderstack.Module
type appEvents = eventstack.Module
type appServices = servicegraph.Services
type appHTTPHandlers = httpwire.Handlers

type appProcessState struct {
	router       http.Handler
	server       *http.Server
	shuttingDown atomic.Bool
	runCancelMu  sync.Mutex
	runCancel    context.CancelFunc
	shutdownOnce sync.Once
}

type appRuntimeState struct {
	Config             config.Config
	Summary            config.Summary
	Logger             *slog.Logger
	LogLevel           *logging.LevelController
	repoRoot           string
	redactText         func(string) string
	addRedactionValues func(...string)
	startedAt          time.Time
}
