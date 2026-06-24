package source

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	bilibilidiagnostics "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/diagnostics"
	bilibilimonitoring "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/monitoring"
	bilibiliproxy "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/proxy"
	bilibiliSession "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/session"
	bilibilisubscriptions "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/subscriptions"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
)

const (
	StateDisabled   = bilibilidiagnostics.StateDisabled
	StateIdle       = bilibilidiagnostics.StateIdle
	StateConnecting = bilibilidiagnostics.StateConnecting
	StateConnected  = bilibilidiagnostics.StateConnected
	StateDegraded   = bilibilidiagnostics.StateDegraded
	StateFailed     = bilibilidiagnostics.StateFailed

	EventLiveStarted      = bilibilimonitoring.EventLiveStarted
	EventLiveEnded        = bilibilimonitoring.EventLiveEnded
	EventDynamicPublished = bilibilimonitoring.EventDynamicPublished

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

type Status = bilibilidiagnostics.Status
type Diagnosis = bilibilidiagnostics.Diagnosis
type DiagnosisCause = bilibilidiagnostics.DiagnosisCause
type DiagnosisAction = bilibilidiagnostics.DiagnosisAction
type LiveStatus = bilibilidiagnostics.LiveStatus
type DynamicStatus = bilibilidiagnostics.DynamicStatus
type requestCooldown = bilibilidiagnostics.Cooldown

type MonitorSnapshot = bilibilimonitoring.MonitorSnapshot
type MonitorItem = bilibilimonitoring.MonitorItem
type MonitorDynamic = bilibilimonitoring.MonitorDynamic
type MonitorLive = bilibilimonitoring.MonitorLive
type BilibiliEvent = bilibilimonitoring.Event
type BilibiliOriginal = bilibilimonitoring.Original
type BilibiliTopic = bilibilimonitoring.Topic
type Author = bilibilimonitoring.Author
type Image = bilibilimonitoring.Image
type Subject = bilibilisubscriptions.Subject

func NewProxyPool(configs []ProxyConfig) *ProxyPool {
	return bilibiliproxy.NewProxyPool(configs)
}

type SubjectProvider interface {
	LoadSubjects(context.Context) (map[string]bilibilisubscriptions.Subject, error)
}

type Deps struct {
	Store         Store
	Accounts      *thirdparty.Service
	Subjects      SubjectProvider
	Dispatcher    Dispatcher
	NotifyStatus  func(Status)
	HTTPTransport http.RoundTripper
	Session       *bilibiliSession.SessionClient
	Identity      *bilibiliSession.IdentityProvider
	ProxyPool     *ProxyPool
	Now           func() time.Time
}
