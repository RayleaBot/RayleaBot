package source

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	bilibiliproxy "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/proxy"
	bilibiliSession "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/session"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
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
	Dispatch(context.Context, runtimeprotocol.Event, string)
}

type Store struct {
	Read  *sql.DB
	Write *sql.DB
}

type ProxyConfig = bilibiliproxy.ProxyConfig
type ProxyPool = bilibiliproxy.ProxyPool

func NewProxyPool(configs []ProxyConfig) *ProxyPool {
	return bilibiliproxy.NewProxyPool(configs)
}

type PluginConfigReader interface {
	ReadAll(context.Context, string) (map[string]any, error)
}

type Deps struct {
	Store         Store
	Accounts      *thirdparty.Service
	PluginConfig  PluginConfigReader
	Dispatcher    Dispatcher
	NotifyStatus  func(Status)
	HTTPTransport http.RoundTripper
	Session       *bilibiliSession.SessionClient
	Identity      *bilibiliSession.IdentityProvider
	ProxyPool     *ProxyPool
	Now           func() time.Time
}
