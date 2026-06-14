package source

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

const (
	subscriptionHubPluginID = "raylea.subscription-hub"

	defaultDynamicIntervalSeconds     = 10
	defaultFallbackIntervalSeconds    = 10
	defaultRefreshIntervalSeconds     = 15
	defaultRequestTimeout             = 20 * time.Second
	bilibiliRiskControlCooldownBase   = 5 * time.Minute
	bilibiliRiskControlCooldownMax    = 30 * time.Minute
	bilibiliAutoFollowCheckInterval   = 6 * time.Hour
	bilibiliRequestCooldownLive       = "live"
	bilibiliRequestCooldownDynamic    = "dynamic"
	bilibiliRequestCooldownAutoFollow = "auto_follow"
)

type Source struct {
	read         *sql.DB
	write        *sql.DB
	accounts     *thirdparty.Service
	pluginConfig interface {
		ReadAll(context.Context, string) (map[string]any, error)
	}
	dispatcher   Dispatcher
	notifyStatus func(Status)
	client       *http.Client
	session      *SessionClient
	identity     *IdentityProvider
	now          func() time.Time

	mu                   sync.RWMutex
	requestMu            sync.Mutex
	status               Status
	roomTasks            map[string]liveRoomTask
	cooldowns            map[string]requestCooldown
	autoFollowChecked    map[string]time.Time
	restart              chan struct{}
	liveAccountOffset    int
	dynamicAccountOffset int
	griskID              string
	griskMu              sync.Mutex
	captchaClient        *CaptchaClient
}
type liveRoomTask struct {
	ctx               context.Context
	cancel            context.CancelFunc
	cookieFingerprint string
	accountID         string
}
type requestCooldown struct {
	Attempts  int
	Until     time.Time
	LastError string
	Scope     string
	Code      string
}

func NewSource(deps Deps) (*Source, error) {
	if deps.Store == nil || deps.Store.Read == nil || deps.Store.Write == nil {
		return nil, errors.New("sqlite store is required")
	}
	if deps.Accounts == nil {
		return nil, errors.New("third-party account service is required")
	}
	if deps.PluginConfig == nil {
		return nil, errors.New("plugin config repository is required")
	}
	if deps.Dispatcher == nil {
		return nil, errors.New("dispatcher is required")
	}
	transport := deps.HTTPTransport
	if transport == nil {
		transport = http.DefaultTransport
	}
	if deps.ProxyPool != nil {
		if proxyTransport := deps.ProxyPool.Transport(); proxyTransport != nil {
			transport = proxyTransport
		}
	}
	now := deps.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	identity := deps.Identity
	if identity == nil {
		identity = NewIdentityProvider(now)
	}
	source := &Source{
		read:         deps.Store.Read,
		write:        deps.Store.Write,
		accounts:     deps.Accounts,
		pluginConfig: deps.PluginConfig,
		dispatcher:   deps.Dispatcher,
		notifyStatus: deps.NotifyStatus,
		client: &http.Client{
			Transport: transport,
			Timeout:   defaultRequestTimeout,
		},
		session:           deps.Session,
		identity:          identity,
		now:               now,
		roomTasks:         make(map[string]liveRoomTask),
		cooldowns:         make(map[string]requestCooldown),
		autoFollowChecked: make(map[string]time.Time),
		restart:           make(chan struct{}, 1),
		captchaClient:     NewCaptchaClient(transport, identity),
	}
	if source.session == nil {
		source.session = NewSessionClient(transport, now, identity)
	}
	source.status = Status{
		Status:  StateIdle,
		Summary: sourceSummary(StateIdle),
		Dynamic: DynamicStatus{
			IntervalSeconds: defaultDynamicIntervalSeconds,
			AutoFollow:      true,
		},
	}
	source.status.Diagnosis = source.diagnosisForStatus(source.status, nil)
	return source, nil
}
