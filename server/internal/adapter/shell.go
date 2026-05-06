package adapter

import (
	"context"
	"errors"
	"fmt"
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
	now            func() time.Time
	dial           dialFunc
	sleep          sleepFunc
	connectTimeout time.Duration
	backoff        *Backoff
}

type Shell struct {
	cfg    config.OneBotConfig
	logger *slog.Logger
	deps   shellDeps

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
}

func New(cfg config.OneBotConfig, logger *slog.Logger) *Shell {
	return newShell(cfg, logger, shellDeps{})
}

func newShell(cfg config.OneBotConfig, logger *slog.Logger, deps shellDeps) *Shell {
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
		deps.connectTimeout = time.Duration(maxInt(cfg.ConnectTimeoutSeconds, 1)) * time.Second
	}
	if deps.backoff == nil {
		deps.backoff = NewBackoff(
			cfg.ReconnectInitialSeconds,
			cfg.ReconnectMultiplier,
			cfg.ReconnectMaxSeconds,
			cfg.ReconnectJitterRatio,
			nil,
		)
	}

	return &Shell{
		cfg:              cfg,
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

func (s *Shell) Start(ctx context.Context) {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return
	}

	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.done = make(chan struct{})
	s.started = true
	s.stopping = false
	s.supervisorCtx = ctx
	s.mu.Unlock()

	s.logger.Info(
		"adapter shell starting",
		"component", "adapter",
		"adapter_state", StateIdle,
		"forward_ws_url", sanitizeWSURL(s.forwardWSURL()),
	)

	s.markTransportPrimed()

	go s.dispatchEvents(runCtx)
	go s.run(runCtx)
}

func (s *Shell) Stop(ctx context.Context) error {
	s.mu.Lock()
	cancel := s.cancel
	done := s.done
	conn := s.conn
	reverseConn := s.reverseConn
	reverseDone := s.reverseDone
	started := s.started
	s.stopping = true
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if conn != nil {
		_ = conn.Close(websocket.StatusNormalClosure, "")
	}
	if reverseConn != nil {
		_ = reverseConn.CloseNow()
	}

	if !started {
		if err := waitForClosed(ctx, reverseDone); err != nil {
			return err
		}
		s.markStopped()
		return nil
	}

	s.logger.Info(
		"adapter shell stopping",
		"component", "adapter",
		"adapter_state", s.Snapshot().State,
	)

	if err := waitForClosed(ctx, done); err != nil {
		return err
	}
	return waitForClosed(ctx, reverseDone)
}

func (s *Shell) Reload(nextCfg config.OneBotConfig) error {
	if s == nil {
		return nil
	}

	s.mu.RLock()
	started := s.started
	supervisorCtx := s.supervisorCtx
	previousCfg := s.cfg
	s.mu.RUnlock()

	if started {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.Stop(stopCtx); err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
	}

	s.applyConfig(nextCfg, previousCfg)
	if !started {
		return nil
	}
	if supervisorCtx == nil {
		supervisorCtx = context.Background()
	}
	if err := supervisorCtx.Err(); err != nil {
		return err
	}

	s.Start(supervisorCtx)
	return nil
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

func (s *Shell) run(ctx context.Context) {
	defer func() {
		s.clearConn(nil)
		s.clearReverseConn(nil)
		s.markStopped()
		s.logger.Info(
			"adapter shell stopped",
			"component", "adapter",
			"adapter_state", StateStopped,
		)

		s.mu.Lock()
		if s.done != nil {
			close(s.done)
		}
		s.started = false
		s.cancel = nil
		s.done = nil
		s.mu.Unlock()
	}()

	snapshot := s.Snapshot()
	if !snapshot.ForwardWS.Enabled || !snapshot.ForwardWS.Configured {
		s.logger.Info(
			"adapter forward websocket is idle",
			"component", "adapter",
			"adapter_state", StateIdle,
		)
		<-ctx.Done()
		return
	}

	retryAttempt := 0
	for {
		if ctx.Err() != nil {
			return
		}

		reachedConnected, terminal := s.runAttempt(ctx)
		if terminal {
			return
		}

		if reachedConnected {
			retryAttempt = 0
		}

		delay := s.deps.backoff.Duration(retryAttempt)
		s.logger.Warn(
			"adapter reconnect scheduled",
			"component", "adapter",
			"adapter_state", StateReconnecting,
			"retry_in", delay.String(),
			"error_code", s.Snapshot().LastErrorCode,
		)

		if err := s.deps.sleep(ctx, delay); err != nil {
			return
		}

		retryAttempt++
	}
}

func (s *Shell) applyConfig(nextCfg config.OneBotConfig, previousCfg config.OneBotConfig) {
	s.mu.Lock()
	s.cfg = nextCfg
	s.deps.connectTimeout = nextConnectTimeout(previousCfg, nextCfg, s.deps.connectTimeout)
	s.deps.backoff = nextBackoff(previousCfg, nextCfg, s.deps.backoff)
	s.httpClient = &http.Client{
		Timeout: s.deps.connectTimeout,
	}
	s.snapshot = newTransportSnapshot(nextCfg)
	s.pendingResponses = make(map[string]chan apiResponse)
	s.recentEventIDs = make(map[string]time.Time)
	s.identityCache = NewIdentityCache(defaultIdentityCacheTTL)
	snapshot := cloneSnapshot(s.snapshot)
	handler := s.stateHandler
	s.mu.Unlock()

	s.emitStateSnapshot(handler, snapshot)
}

func nextConnectTimeout(previousCfg config.OneBotConfig, nextCfg config.OneBotConfig, current time.Duration) time.Duration {
	if nextCfg.ConnectTimeoutSeconds == previousCfg.ConnectTimeoutSeconds && current > 0 {
		return current
	}

	return time.Duration(maxInt(nextCfg.ConnectTimeoutSeconds, 1)) * time.Second
}

func nextBackoff(previousCfg config.OneBotConfig, nextCfg config.OneBotConfig, current *Backoff) *Backoff {
	if reconnectSettingsEqual(previousCfg, nextCfg) && current != nil {
		return current
	}

	var randFloat func() float64
	if current != nil {
		randFloat = current.randFloat
	}

	return NewBackoff(
		nextCfg.ReconnectInitialSeconds,
		nextCfg.ReconnectMultiplier,
		nextCfg.ReconnectMaxSeconds,
		nextCfg.ReconnectJitterRatio,
		randFloat,
	)
}

func reconnectSettingsEqual(left config.OneBotConfig, right config.OneBotConfig) bool {
	return left.ReconnectInitialSeconds == right.ReconnectInitialSeconds &&
		left.ReconnectMultiplier == right.ReconnectMultiplier &&
		left.ReconnectMaxSeconds == right.ReconnectMaxSeconds &&
		left.ReconnectJitterRatio == right.ReconnectJitterRatio
}

func (s *Shell) runAttempt(ctx context.Context) (bool, bool) {
	s.markConnecting()
	s.logger.Info(
		"adapter forward websocket connecting",
		"component", "adapter",
		"adapter_state", StateConnecting,
		"transport", string(TransportForwardWS),
		"ws_url", sanitizeWSURL(s.forwardWSURL()),
	)

	conn, response, err := s.dial(ctx)
	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}
	if err != nil {
		if isAuthFailure(response) {
			s.markAuthFailed(err)
			s.logger.Error(
				"adapter forward websocket authentication failed",
				"component", "adapter",
				"adapter_state", StateAuthFailed,
				"transport", string(TransportForwardWS),
				"ws_url", sanitizeWSURL(s.forwardWSURL()),
				"error_code", errorCodeForwardWSConnectFail,
				"err", summarizeError(err),
			)

			<-ctx.Done()
			return false, true
		}

		if ctx.Err() != nil {
			return false, true
		}

		s.markReconnecting(errorCodeForwardWSConnectFail, err)
		return false, false
	}

	s.setConn(conn)
	defer func() {
		_ = conn.Close(websocket.StatusNormalClosure, "")
		s.clearConn(conn)
	}()

	ready, err := s.waitForReadyFrame(ctx, TransportForwardWS, conn)
	if err != nil {
		if ctx.Err() != nil {
			return false, true
		}

		s.markReconnecting(errorCodeForwardWSConnectFail, err)
		return false, false
	}

	s.markConnected(ready.ObservedAt)
	s.logger.Info(
		"adapter forward websocket connected",
		"component", "adapter",
		"adapter_state", StateConnected,
		"transport", string(TransportForwardWS),
		"ws_url", sanitizeWSURL(s.forwardWSURL()),
	)
	if handler := s.currentReadyHandler(); handler != nil {
		go handler(ctx)
	}

	err = s.readLoop(ctx, TransportForwardWS, conn)
	if err == nil {
		return true, true
	}
	if ctx.Err() != nil {
		return true, true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		s.logger.Warn(
			"adapter forward websocket heartbeat timeout",
			"component", "adapter",
			"adapter_state", StateConnected,
			"error_code", errorCodeForwardWSSessionLost,
			"transport", string(TransportForwardWS),
			"ws_url", sanitizeWSURL(s.forwardWSURL()),
		)
	}

	s.markReconnecting(errorCodeForwardWSSessionLost, err)
	return true, false
}

func (s *Shell) waitForReadyFrame(ctx context.Context, transport TransportKey, conn *websocket.Conn) (FrameSummary, error) {
	waitingForFirstFrame := true

	for {
		readyCtx, cancel := s.waitContext(ctx)
		frame, err := s.readFrame(readyCtx, conn)
		cancel()
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				if waitingForFirstFrame {
					return FrameSummary{}, fmt.Errorf("timed out waiting for first frame: %w", err)
				}
				return FrameSummary{}, fmt.Errorf("timed out waiting for ready frame: %w", err)
			}
			return FrameSummary{}, err
		}

		if err := s.recordAndValidateFrame(transport, frame); err != nil {
			return FrameSummary{}, err
		}
		if isReadySummary(frame.Summary) {
			return frame.Summary, nil
		}

		waitingForFirstFrame = false
	}
}

func (s *Shell) readLoop(ctx context.Context, transport TransportKey, conn *websocket.Conn) error {
	for {
		readCtx, cancel := s.readContext(ctx)
		frame, err := s.readFrame(readCtx, conn)
		cancel()
		if err != nil {
			return err
		}

		if err := s.recordAndValidateFrame(transport, frame); err != nil {
			return err
		}

		s.routeAPIResponse(frame)
		s.forwardSupportedEvent(ctx, transport, frame)
	}
}

func (s *Shell) readContext(ctx context.Context) (context.Context, context.CancelFunc) {
	snapshot := s.Snapshot()
	timeout := s.provisionalReadTimeout(snapshot)
	return context.WithTimeout(ctx, timeout)
}

func (s *Shell) recordAndValidateFrame(transport TransportKey, frame classifiedFrame) error {
	snapshot := s.recordFrame(frame)

	switch {
	case isIgnoredAPIResponse(frame):
		s.logger.Warn(
			"ignored OneBot API response with unsupported echo",
			"component", "adapter",
			"adapter_state", s.Snapshot().State,
			"direction", "inbound",
			"frame_type", frame.Summary.Type,
			"reason", frame.InvalidSummary,
			"echo_value_type", echoValueType(frame.Frame.Echo),
			"payload_preview", frame.PayloadPreview,
			"transport", string(transport),
			"endpoint", s.transportEndpoint(transport),
		)
		return nil
	case frame.Summary.Category == FrameCategoryInvalid:
		s.logger.Warn(
			"invalid OneBot frame received",
			"component", "adapter",
			"adapter_state", s.Snapshot().State,
			"direction", "inbound",
			"frame_type", frame.Summary.Type,
			"invalid_frame_count", snapshot.InvalidReceivedFrames,
			"reason", frame.InvalidSummary,
			"payload_preview", frame.PayloadPreview,
			"transport", string(transport),
			"endpoint", s.transportEndpoint(transport),
		)
		return fmt.Errorf("invalid frame: %s", frame.InvalidSummary)
	case isLifecycleDisable(frame.Frame):
		s.logger.Warn(
			"adapter lifecycle disable observed",
			"component", "adapter",
			"adapter_state", s.Snapshot().State,
			"frame_type", frame.Summary.Type,
			"transport", string(transport),
			"endpoint", s.transportEndpoint(transport),
		)
	}

	return nil
}

func isIgnoredAPIResponse(frame classifiedFrame) bool {
	return frame.Summary.Category == FrameCategoryUnknown && frame.Summary.Type == "api.response.ignored"
}

func echoValueType(value any) string {
	if value == nil {
		return "nil"
	}
	return fmt.Sprintf("%T", value)
}

func (s *Shell) readFrame(ctx context.Context, conn *websocket.Conn) (classifiedFrame, error) {
	messageType, payload, err := conn.Read(ctx)
	if err != nil {
		return classifiedFrame{}, err
	}

	return classifyFrame(messageType, payload, s.deps.now()), nil
}

func (s *Shell) dial(ctx context.Context) (*websocket.Conn, *http.Response, error) {
	dialCtx, cancel := context.WithTimeout(ctx, s.deps.connectTimeout)
	defer cancel()

	headers := http.Header{}
	accessToken := strings.TrimSpace(s.cfg.ForwardWS.AccessToken)
	if accessToken != "" {
		headers.Set("Authorization", "Bearer "+accessToken)
	}

	return s.deps.dial(dialCtx, dialURL(s.forwardWSURL(), accessToken), &websocket.DialOptions{
		HTTPHeader: headers,
	})
}

func (s *Shell) recordFrame(frame classifiedFrame) Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	applyFrameSummary(&s.snapshot, frame)
	return cloneSnapshot(s.snapshot)
}

func (s *Shell) setConn(conn *websocket.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.conn = conn
}

func (s *Shell) clearConn(target *websocket.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if target == nil || s.conn == target {
		s.conn = nil
	}
}

func (s *Shell) clearReverseConn(target *websocket.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if target == nil || s.reverseConn == target {
		s.reverseConn = nil
	}
}

func (s *Shell) markConnecting() {
	s.mu.Lock()
	s.snapshot.ForwardWS.State = TransportStateConnecting
	s.snapshot.ForwardWS.LastErrorCode = ""
	s.snapshot.ForwardWS.LastErrorMessage = ""
	s.snapshot.LastErrorCode = ""
	s.snapshot.LastErrorMessage = ""
	s.snapshot.ReadyFrameSeen = false
	s.snapshot.ConnectedAt = nil
	s.snapshot.LastFrameCategory = ""
	s.snapshot.LastFrameType = ""
	s.snapshot.LastFrameAt = nil
	s.snapshot.HeartbeatSeen = false
	s.snapshot.LastHeartbeatAt = nil
	s.snapshot.HeartbeatInterval = 0
	s.refreshAggregateStateLocked()
	snapshot := cloneSnapshot(s.snapshot)
	handler := s.stateHandler
	s.mu.Unlock()
	s.emitStateSnapshot(handler, snapshot)
}

func (s *Shell) markConnected(now time.Time) {
	s.mu.Lock()
	s.snapshot.ForwardWS.State = TransportStateConnected
	s.snapshot.ForwardWS.LastErrorCode = ""
	s.snapshot.ForwardWS.LastErrorMessage = ""
	s.snapshot.LastErrorCode = ""
	s.snapshot.LastErrorMessage = ""
	s.snapshot.ReadyFrameSeen = true
	s.snapshot.ConnectedAt = cloneTime(&now)
	s.refreshAggregateStateLocked()
	snapshot := cloneSnapshot(s.snapshot)
	handler := s.stateHandler
	s.mu.Unlock()
	s.emitStateSnapshot(handler, snapshot)
}

func (s *Shell) markAuthFailed(err error) {
	s.mu.Lock()
	s.snapshot.ForwardWS.State = TransportStateAuthFailed
	s.snapshot.ForwardWS.LastErrorCode = errorCodeForwardWSConnectFail
	s.snapshot.ForwardWS.LastErrorMessage = summarizeError(err)
	s.snapshot.LastErrorCode = errorCodeForwardWSConnectFail
	s.snapshot.LastErrorMessage = summarizeError(err)
	s.snapshot.ReadyFrameSeen = false
	s.snapshot.ConnectedAt = nil
	s.refreshAggregateStateLocked()
	snapshot := cloneSnapshot(s.snapshot)
	handler := s.stateHandler
	s.mu.Unlock()
	s.emitStateSnapshot(handler, snapshot)
}

func (s *Shell) markReconnecting(code string, err error) {
	s.mu.Lock()
	s.snapshot.ForwardWS.State = TransportStateReconnecting
	s.snapshot.ForwardWS.LastErrorCode = code
	s.snapshot.ForwardWS.LastErrorMessage = summarizeError(err)
	s.snapshot.LastErrorCode = code
	s.snapshot.LastErrorMessage = summarizeError(err)
	s.snapshot.ReadyFrameSeen = false
	s.snapshot.ConnectedAt = nil
	s.refreshAggregateStateLocked()
	snapshot := cloneSnapshot(s.snapshot)
	handler := s.stateHandler
	s.mu.Unlock()
	s.emitStateSnapshot(handler, snapshot)
}

func (s *Shell) markStopped() {
	s.mu.Lock()
	s.snapshot.ForwardWS.State = TransportStateStopped
	if s.snapshot.ReverseWS.Configured && s.snapshot.ReverseWS.Enabled {
		s.snapshot.ReverseWS.State = TransportStateStopped
	} else {
		s.snapshot.ReverseWS.State = TransportStateIdle
	}
	if s.snapshot.Webhook.Configured && s.snapshot.Webhook.Enabled {
		s.snapshot.Webhook.State = TransportStateStopped
	} else {
		s.snapshot.Webhook.State = TransportStateIdle
	}
	if s.snapshot.HTTPAPI.Configured && s.snapshot.HTTPAPI.Enabled {
		s.snapshot.HTTPAPI.State = TransportStateStopped
	} else {
		s.snapshot.HTTPAPI.State = TransportStateIdle
	}
	s.snapshot.ReadyFrameSeen = false
	s.snapshot.ConnectedAt = nil
	s.refreshAggregateStateLocked()
	snapshot := cloneSnapshot(s.snapshot)
	handler := s.stateHandler
	s.mu.Unlock()
	s.emitStateSnapshot(handler, snapshot)
}

func (s *Shell) emitStateSnapshot(handler func(Snapshot), snapshot Snapshot) {
	if handler == nil {
		return
	}
	handler(snapshot)
}

func isAuthFailure(response *http.Response) bool {
	if response == nil {
		return false
	}

	return response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden
}

func summarizeError(err error) string {
	if err == nil {
		return ""
	}

	return strings.Join(strings.Fields(err.Error()), " ")
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

func dialURL(raw, accessToken string) string {
	if raw == "" || accessToken == "" {
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

func (s *Shell) forwardSupportedEvent(ctx context.Context, transport TransportKey, frame classifiedFrame) {
	if frame.Summary.Category != FrameCategoryEvent {
		return
	}

	s.invalidateIdentityCacheForFrame(frame.Frame)

	normalizedEvent, ok := normalizeSupportedEvent(frame.Frame, frame.Summary.ObservedAt)
	if !ok {
		s.logger.Debug(
			"adapter event ignored by runtime bridge",
			"component", "adapter",
			"adapter_state", s.Snapshot().State,
			"transport", string(transport),
			"frame_type", frame.Summary.Type,
		)
		return
	}
	if s.isDuplicateEvent(normalizedEvent.EventID, frame.Summary.ObservedAt) {
		s.logger.Info(
			"duplicate OneBot event dropped",
			"component", "adapter",
			"adapter_state", s.Snapshot().State,
			"transport", string(transport),
			"error_code", errorCodeWebhookDuplicateEvent,
			"event_id", normalizedEvent.EventID,
			"event_type", normalizedEvent.EventType,
		)
		return
	}

	handler := s.currentEventHandler()
	if handler == nil {
		return
	}

	select {
	case s.eventQueue <- normalizedEvent:
	case <-ctx.Done():
		return
	default:
		s.logger.Warn(
			"adapter event queue is full; dropping event",
			"component", "adapter",
			"adapter_state", s.Snapshot().State,
			"event_kind", normalizedEvent.Kind,
			"event_type", normalizedEvent.EventType,
		)
	}
}

func (s *Shell) currentEventHandler() func(context.Context, NormalizedEvent) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.eventHandler
}

func (s *Shell) currentReadyHandler() func(context.Context) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.readyHandler
}

func (s *Shell) dispatchEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-s.eventQueue:
			handler := s.currentEventHandler()
			if handler == nil {
				continue
			}
			handler(ctx, event)
		}
	}
}
