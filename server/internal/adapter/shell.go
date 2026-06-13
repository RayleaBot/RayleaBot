package adapter

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

const (
	errorCodeAuthFailed             = "adapter.auth_failed"
	errorCodeConnectionFail         = "adapter.connection_failed"
	errorCodeConnectionLost         = "adapter.connection_lost"
	errorCodeForwardWSConnectFail   = "adapter.transport_forward_ws_connection_failed"
	errorCodeForwardWSSessionLost   = "adapter.transport_forward_ws_session_lost"
	errorCodeReverseWSAuthFailed    = "adapter.transport_reverse_ws_auth_failed"
	errorCodeHTTPAPIRequestFailed   = "adapter.transport_http_api_request_failed"
	errorCodeHTTPAPIAuthFailed      = "adapter.transport_http_api_auth_failed"
	errorCodeHTTPAPIInvalidResponse = "adapter.transport_http_api_invalid_response"
	errorCodeWebhookAuthFailed      = "adapter.transport_webhook_auth_failed"
	errorCodeWebhookInvalidPayload  = "adapter.transport_webhook_invalid_payload"
	errorCodeWebhookDuplicateEvent  = "adapter.transport_webhook_duplicate_event"
	defaultConnectedReadTimeout     = 2 * time.Minute
	recentEventDedupRetention       = 2 * time.Minute
)

type dialFunc func(context.Context, string, *websocket.DialOptions) (*websocket.Conn, *http.Response, error)
type sleepFunc func(context.Context, time.Duration) error
type shellDeps struct {
	now             func() time.Time
	dial            dialFunc
	sleep           sleepFunc
	connectTimeout  time.Duration
	backoff         *Backoff
	skipRuntimeInfo bool
}
type Shell struct {
	cfg        config.OneBotConfig
	adapterCfg config.AdapterConfig
	logger     *slog.Logger
	deps       shellDeps

	sendMu           sync.Mutex
	mu               sync.RWMutex
	snapshot         Snapshot
	conn             *websocket.Conn
	reverseConn      *websocket.Conn
	reverseDone      chan struct{}
	cancel           context.CancelFunc
	done             chan struct{}
	started          bool
	stopping         bool
	supervisorCtx    context.Context
	eventHandler     func(context.Context, NormalizedEvent)
	readyHandler     func(context.Context)
	stateHandler     func(Snapshot)
	eventQueue       chan NormalizedEvent
	nextEcho         uint64
	pendingResponses map[string]chan apiResponse
	httpClient       *http.Client
	recentEventIDs   map[string]time.Time
	identityCache    *IdentityCache
	dedupDrops       uint64
	metrics          MetricsObserver
}

// MetricsObserver records adapter-side counter increments without coupling
// this package to client_golang directly. Implementations must be safe for
// concurrent use.
type MetricsObserver interface {
	IncAdapterDedupDrop()
	IncEventPipelineStage(stage, outcome string)
}

func New(cfg config.OneBotConfig, adapterCfg config.AdapterConfig, logger *slog.Logger) *Shell {
	return newShell(cfg, adapterCfg, logger, shellDeps{})
}
func NewForTest(cfg config.OneBotConfig, adapterCfg config.AdapterConfig, logger *slog.Logger, skipRuntimeInfo bool) *Shell {
	return newShell(cfg, adapterCfg, logger, shellDeps{skipRuntimeInfo: skipRuntimeInfo})
}
func newShell(cfg config.OneBotConfig, adapterCfg config.AdapterConfig, logger *slog.Logger, deps shellDeps) *Shell {
	if logger == nil {
		logger = slog.New(slog.NewJSONHandler(io.Discard, nil))
	}
	if deps.now == nil {
		deps.now = time.Now
	}
	if deps.dial == nil {
		deps.dial = websocket.Dial
	}
	if deps.sleep == nil {
		deps.sleep = sleepWithContext
	}
	if deps.connectTimeout <= 0 {
		deps.connectTimeout = time.Duration(maxInt(adapterCfg.ConnectTimeoutSeconds, 1)) * time.Second
	}
	if deps.backoff == nil {
		deps.backoff = NewBackoff(
			adapterCfg.ReconnectInitialSeconds,
			adapterCfg.ReconnectMultiplier,
			adapterCfg.ReconnectMaxSeconds,
			adapterCfg.ReconnectJitterRatio,
			nil,
		)
	}

	return &Shell{
		cfg:              cfg,
		adapterCfg:       adapterCfg,
		logger:           logger,
		deps:             deps,
		snapshot:         newTransportSnapshot(cfg),
		eventQueue:       make(chan NormalizedEvent, 16),
		pendingResponses: make(map[string]chan apiResponse),
		httpClient: &http.Client{
			Timeout: deps.connectTimeout,
		},
		recentEventIDs: make(map[string]time.Time),
		identityCache:  NewIdentityCache(defaultIdentityCacheTTL),
	}
}
func (s *Shell) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneSnapshot(s.snapshot)
}
func (s *Shell) SetEventHandler(handler func(context.Context, NormalizedEvent)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.eventHandler = handler
}
func (s *Shell) SetReadyHandler(handler func(context.Context)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.readyHandler = handler
}
func (s *Shell) SetStateHandler(handler func(Snapshot)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stateHandler = handler
}

// SetMetricsObserver wires the adapter dedup and pipeline stage counters
// behind the MetricsObserver interface. Passing nil disables instrumentation.
func (s *Shell) SetMetricsObserver(observer MetricsObserver) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metrics = observer
}
