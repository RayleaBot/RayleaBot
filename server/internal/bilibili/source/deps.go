package source

import (
	"context"
	"net/http"
	"time"

	bilibiliSession "github.com/RayleaBot/RayleaBot/server/internal/bilibili/session"
	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginconfig"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/runtime/protocol"
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
	Dispatch(context.Context, runtimeprotocol.Event, string) []dispatch.DeliveryResult
}

type Deps struct {
	Store         *storage.Store
	Accounts      *thirdparty.Service
	PluginConfig  pluginconfig.Repository
	Dispatcher    Dispatcher
	NotifyStatus  func(Status)
	HTTPTransport http.RoundTripper
	Session       *bilibiliSession.SessionClient
	Identity      *bilibiliSession.IdentityProvider
	ProxyPool     *ProxyPool
	Now           func() time.Time
}
