package bilibili

import (
	"context"
	"net/http"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginconfig"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

const (
	StateDisabled   = "disabled"
	StateIdle       = "idle"
	StateConnecting = "connecting"
	StateConnected  = "connected"
	StateDegraded   = "degraded"
	StateFailed     = "failed"

	EventLiveStarted      = "bilibili.live.started"
	EventLiveEnded        = "bilibili.live.ended"
	EventDynamicPublished = "bilibili.dynamic.published"

	sourceProtocol = "bilibili"
	sourceAdapter  = "bilibili.source"
)

type Dispatcher interface {
	Dispatch(context.Context, runtime.Event, string) []dispatch.DeliveryResult
}

type Deps struct {
	Store         *storage.Store
	Accounts      *thirdparty.Service
	PluginConfig  pluginconfig.Repository
	Dispatcher    Dispatcher
	NotifyStatus  func(Status)
	HTTPTransport http.RoundTripper
	Session       *SessionClient
	Identity      *IdentityProvider
	ProxyPool     *ProxyPool
	Now           func() time.Time
}
