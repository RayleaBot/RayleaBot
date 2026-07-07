package onebot11

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
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
	pendingResponses map[string]chan APIResponse
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
		pendingResponses: make(map[string]chan APIResponse),
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
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metrics = observer
}

func (s *Shell) currentWSConn() (*websocket.Conn, TransportKey, Snapshot) {
	return s.currentWSConnForTransport("")
}

func (s *Shell) currentWSConnForTransport(transport TransportKey) (*websocket.Conn, TransportKey, Snapshot) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot := cloneSnapshot(s.snapshot)
	switch transport {
	case TransportForwardWS:
		if s.conn != nil && snapshot.ForwardWS.State == TransportStateConnected {
			return s.conn, TransportForwardWS, snapshot
		}
		return nil, "", snapshot
	case TransportReverseWS:
		if s.reverseConn != nil && snapshot.ReverseWS.State == TransportStateConnected {
			return s.reverseConn, TransportReverseWS, snapshot
		}
		return nil, "", snapshot
	case TransportHTTPAPI:
		return nil, "", snapshot
	}

	switch {
	case s.conn != nil && snapshot.ForwardWS.State == TransportStateConnected:
		return s.conn, TransportForwardWS, snapshot
	case s.reverseConn != nil && snapshot.ReverseWS.State == TransportStateConnected:
		return s.reverseConn, TransportReverseWS, snapshot
	default:
		return nil, "", snapshot
	}
}

func (s *Shell) isDuplicateEvent(eventID string, observedAt time.Time) bool {
	if strings.TrimSpace(eventID) == "" {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := observedAt.Add(-recentEventDedupRetention)
	for key, seenAt := range s.recentEventIDs {
		if seenAt.Before(cutoff) {
			delete(s.recentEventIDs, key)
		}
	}
	if _, ok := s.recentEventIDs[eventID]; ok {
		s.dedupDrops++
		if s.metrics != nil {
			s.metrics.IncAdapterDedupDrop()
			s.metrics.IncEventPipelineStage("adapter", "dedup_drop")
		}
		return true
	}
	s.recentEventIDs[eventID] = observedAt
	if s.metrics != nil {
		s.metrics.IncEventPipelineStage("adapter", "accepted")
	}
	return false
}

// DedupDropsSnapshot returns the cumulative number of inbound events dropped
// because their event id matched a recently observed event within the
// dedup retention window. The counter is monotonically non-decreasing and
// safe to read from the bridge observability path.
func (s *Shell) DedupDropsSnapshot() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dedupDrops
}

func (s *Shell) refreshRuntimeInfo(ctx context.Context, transport TransportKey) {
	if transport == TransportWebhook || s.deps.skipRuntimeInfo {
		return
	}

	lookupCtx, cancel := context.WithTimeout(ctx, defaultIdentityLookupTimeout)
	defer cancel()

	version, versionErr := s.getVersionInfoOnTransport(lookupCtx, transport)
	login, loginErr := s.getLoginInfoOnTransport(lookupCtx, transport)
	if versionErr != nil && loginErr != nil {
		s.clearTransportRuntimeInfo(transport)
		return
	}

	info := TransportRuntimeInfo{
		Provider:        DetectProvider(version.AppName),
		AppName:         version.AppName,
		ProtocolVersion: version.ProtocolVersion,
		AppVersion:      version.AppVersion,
		UserID:          login.ID,
		Nickname:        login.Nickname,
	}
	s.updateTransportRuntimeInfo(transport, info)
}

func (s *Shell) updateTransportRuntimeInfo(transport TransportKey, info TransportRuntimeInfo) {
	s.mu.Lock()
	switch transport {
	case TransportForwardWS:
		s.snapshot.ForwardWS.RuntimeInfo = info
	case TransportReverseWS:
		s.snapshot.ReverseWS.RuntimeInfo = info
	case TransportHTTPAPI:
		s.snapshot.HTTPAPI.RuntimeInfo = info
	default:
		s.mu.Unlock()
		return
	}
	s.refreshAggregateStateLocked()
	snapshot := cloneSnapshot(s.snapshot)
	handler := s.stateHandler
	s.mu.Unlock()
	s.emitStateSnapshot(handler, snapshot)
}

func (s *Shell) clearTransportRuntimeInfo(transport TransportKey) {
	s.mu.Lock()
	s.clearTransportRuntimeInfoLocked(transport)
	snapshot := cloneSnapshot(s.snapshot)
	handler := s.stateHandler
	s.mu.Unlock()
	s.emitStateSnapshot(handler, snapshot)
}

func (s *Shell) clearTransportRuntimeInfoLocked(transport TransportKey) {
	switch transport {
	case TransportForwardWS:
		s.snapshot.ForwardWS.RuntimeInfo = TransportRuntimeInfo{}
	case TransportReverseWS:
		s.snapshot.ReverseWS.RuntimeInfo = TransportRuntimeInfo{}
	case TransportHTTPAPI:
		s.snapshot.HTTPAPI.RuntimeInfo = TransportRuntimeInfo{}
	case TransportWebhook:
		s.snapshot.Webhook.RuntimeInfo = TransportRuntimeInfo{}
	}
}

func (s *Shell) AcceptWebhookPayload(ctx context.Context, payload []byte) error {
	frame := classifyFrame(websocket.MessageText, payload, s.deps.now())
	if err := s.recordAndValidateFrame(TransportWebhook, frame); err != nil {
		s.markTransportFailure(TransportWebhook, TransportStateListening, errorCodeWebhookInvalidPayload, err)
		return errorf(errorCodeWebhookInvalidPayload, "webhook payload is invalid", err)
	}

	s.mu.Lock()
	s.snapshot.Webhook.State = TransportStateListening
	s.snapshot.Webhook.LastErrorCode = ""
	s.snapshot.Webhook.LastErrorMessage = ""
	s.syncLastErrorLocked()
	s.refreshAggregateStateLocked()
	snapshot := cloneSnapshot(s.snapshot)
	handler := s.stateHandler
	s.mu.Unlock()
	s.emitStateSnapshot(handler, snapshot)

	s.routeAPIResponse(frame)
	s.forwardSupportedEvent(ctx, TransportWebhook, frame)
	return nil
}

func (s *Shell) MarkReverseWSAuthFailed() {
	s.markTransportFailure(TransportReverseWS, TransportStateAuthFailed, errorCodeReverseWSAuthFailed, errors.New("reverse websocket authentication failed"))
}

func (s *Shell) MarkWebhookAuthFailed() {
	s.markTransportFailure(TransportWebhook, TransportStateAuthFailed, errorCodeWebhookAuthFailed, errors.New("webhook authentication failed"))
}

func isAuthFailure(response *http.Response) bool {
	if response == nil {
		return false
	}

	return response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden
}

func sanitizeWSURL(raw string) string {
	if raw == "" {
		return ""
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}

	if parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}

	return parsed.Scheme + "://" + parsed.Host
}

func dialURL(raw, accessToken string, includeTokenQuery bool) string {
	if raw == "" || accessToken == "" || !includeTokenQuery {
		return raw
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}

	query := parsed.Query()
	query.Set("access_token", accessToken)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func (s *Shell) waitContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if s.deps.connectTimeout <= 0 {
		return context.WithCancel(ctx)
	}

	return context.WithTimeout(ctx, s.deps.connectTimeout)
}

func (s *Shell) provisionalReadTimeout(snapshot Snapshot) time.Duration {
	if snapshot.HeartbeatInterval > 0 {
		return snapshot.HeartbeatInterval * 3
	}
	if snapshot.State == StateConnected {
		if s.deps.connectTimeout > defaultConnectedReadTimeout {
			return s.deps.connectTimeout
		}
		return defaultConnectedReadTimeout
	}
	if s.deps.connectTimeout > 0 {
		return s.deps.connectTimeout
	}

	return time.Second
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func waitForClosed(ctx context.Context, ch <-chan struct{}) error {
	if ch == nil {
		return nil
	}

	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func maxInt(value, fallback int) int {
	if value <= 0 {
		return fallback
	}

	return value
}
