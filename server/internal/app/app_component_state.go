package app

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	adaptershell "github.com/RayleaBot/RayleaBot/server/internal/adapter/shell"
	"github.com/RayleaBot/RayleaBot/server/internal/auth"
	bilibilisource "github.com/RayleaBot/RayleaBot/server/internal/bilibili/source"
	"github.com/RayleaBot/RayleaBot/server/internal/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/console"
	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/governance"
	"github.com/RayleaBot/RayleaBot/server/internal/localaction"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	managementevents "github.com/RayleaBot/RayleaBot/server/internal/management/events"
	managementhttp "github.com/RayleaBot/RayleaBot/server/internal/management/http"
	"github.com/RayleaBot/RayleaBot/server/internal/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginconfig"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginfile"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginkv"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/service"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginwebhook"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
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
	loginFailures *managementhttp.LoginFailureTracker
}

type appPlugins struct {
	Plugins           *plugincatalog.Catalog
	Adapter           *adaptershell.Shell
	Bridge            *bridge.Bridge
	Dispatcher        *dispatch.Dispatcher
	replyTargets      *outbound.ReplyTargetCache
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
	renderer          *renderservice.Service
	pluginLogLimiter  *localaction.PluginLogLimiter
	outboundLimiter   *outbound.MessageRateLimiter
}

type appServices struct {
	localActions     *localaction.Service
	pluginLifecycle  *pluginservice.Controller
	eventIngress     *eventIngressService
	protocol         *managementhttp.ProtocolService
	pluginWebhooks   *pluginwebhook.Service
	governance       *governance.Service
	governanceEvents *managementevents.GovernanceService
	logs             *logService
	system           *systemService
	thirdParty       *thirdparty.Service
	bilibiliSource   *bilibilisource.Source
	bilibiliEvents   *managementevents.BilibiliSourceService
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
