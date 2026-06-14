package app

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/auth"
	adaptershell "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/shell"
	"github.com/RayleaBot/RayleaBot/server/internal/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/console"
	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/eventingress"
	"github.com/RayleaBot/RayleaBot/server/internal/governance"
	bilibilisource "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/source"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/management/authapi"
	managementevents "github.com/RayleaBot/RayleaBot/server/internal/management/events"
	"github.com/RayleaBot/RayleaBot/server/internal/management/protocolapi"
	"github.com/RayleaBot/RayleaBot/server/internal/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	localaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginconfig "github.com/RayleaBot/RayleaBot/server/internal/plugins/configstore"
	pluginfile "github.com/RayleaBot/RayleaBot/server/internal/plugins/filestore"
	pluginkv "github.com/RayleaBot/RayleaBot/server/internal/plugins/kvstore"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/lifecycle"
	pluginwebhook "github.com/RayleaBot/RayleaBot/server/internal/plugins/webhook"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
	systemsvc "github.com/RayleaBot/RayleaBot/server/internal/system"
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
	loginFailures *authapi.LoginFailureTracker
}

type appPlugins struct {
	Plugins           *plugincatalog.Catalog
	Adapter           *adaptershell.Shell
	Bridge            *bridge.Bridge
	Dispatcher        *dispatch.Dispatcher
	replyTargets      *outbound.ReplyTargetCache
	outboundSender    eventingress.OutboundActionSender
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
	eventIngress     *eventingress.Service
	protocol         *protocolapi.Service
	pluginWebhooks   *pluginwebhook.Service
	governance       *governance.Service
	governanceEvents *managementevents.GovernanceService
	logs             *logging.ManagementService
	system           *systemsvc.Service
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
}
