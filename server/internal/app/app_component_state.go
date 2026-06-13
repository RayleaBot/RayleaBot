package app

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/auth"
	source "github.com/RayleaBot/RayleaBot/server/internal/bilibili"
	"github.com/RayleaBot/RayleaBot/server/internal/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/console"
	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/governance"
	"github.com/RayleaBot/RayleaBot/server/internal/localaction"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginconfig"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginfile"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginkv"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginwebhook"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
	"github.com/RayleaBot/RayleaBot/server/internal/render"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

type appCore struct {
	Config     config.Config
	Summary    config.Summary
	Logger     *slog.Logger
	LogLevel   *logging.LevelController
	repoRoot   string
	redactText func(string) string
	startedAt  time.Time
}

type appPlatform struct {
	Auth          *auth.Manager
	Storage       *storage.Store
	Secrets       secrets.Store
	Tasks         *tasks.Registry
	taskExecutor  *tasks.Executor
	Scheduler     *scheduler.Engine
	Logs          *logging.Stream
	LogRepository logging.Repository
	Console       *console.Stream
	loginFailures *loginFailureTracker
}

type appPlugins struct {
	Plugins           *plugins.Catalog
	Adapter           *adapter.Shell
	Bridge            *bridge.Bridge
	Dispatcher        *dispatch.Dispatcher
	replyTargets      *replyTargetCache
	outboundSender    outboundActionSender
	PluginInstaller   plugins.InstallCoordinator
	PluginUninstaller plugins.UninstallCoordinator
	pluginRepository  plugins.DesiredStateRepository
	pluginConfig      pluginconfig.Repository
	pluginFiles       *pluginfile.Service
	pluginKV          pluginkv.Repository
	grantRepository   plugins.GrantRepository
	blacklistRepo     permission.BlacklistRepository
	whitelistRepo     permission.WhitelistRepository
	whitelistState    permission.WhitelistStateRepository
	webhooks          *pluginwebhook.Registry
	renderer          *render.Service
	pluginLogLimiter  *localaction.PluginLogLimiter
	outboundLimiter   *outbound.MessageRateLimiter
}

type appServices struct {
	localActions     *localaction.Service
	pluginLifecycle  *pluginLifecycleController
	eventIngress     *eventIngressService
	protocol         *protocolService
	pluginWebhooks   *pluginwebhook.Service
	governance       *governance.Service
	governanceEvents *governanceEventService
	logs             *logService
	system           *systemService
	thirdParty       *thirdparty.Service
	bilibiliSource   *source.Source
	bilibiliEvents   *bilibiliSourceEventService
}

type appProcessState struct {
	router       http.Handler
	server       *http.Server
	shuttingDown atomic.Bool
	runCancelMu  sync.Mutex
	runCancel    context.CancelFunc
	shutdownOnce sync.Once
}

type appRuntimeState struct {
	Config     config.Config
	Summary    config.Summary
	Logger     *slog.Logger
	LogLevel   *logging.LevelController
	repoRoot   string
	redactText func(string) string
	startedAt  time.Time

	recoveryMu           sync.RWMutex
	recoverySummary      *recovery.CompatibilitySummary
	startupRuntimeMu     sync.RWMutex
	startupRuntimeStates map[string]startupRuntimeState
}
