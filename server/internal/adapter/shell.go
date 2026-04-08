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
	errorCodeAuthFailed         = "adapter.auth_failed"
	errorCodeConnectionFail     = "adapter.connection_failed"
	errorCodeConnectionLost     = "adapter.connection_lost"
	defaultConnectedReadTimeout = 2 * time.Minute
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
	cancel           context.CancelFunc
	done             chan struct{}
	started          bool
	eventHandler     func(context.Context, NormalizedEvent)
	readyHandler     func(context.Context)
	eventQueue       chan NormalizedEvent
	nextEcho         uint64
	pendingResponses map[string]chan apiResponse
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
		cfg:    cfg,
		logger: logger,
		deps:   deps,
		snapshot: Snapshot{
			State: StateIdle,
		},
		eventQueue:       make(chan NormalizedEvent, 16),
		pendingResponses: make(map[string]chan apiResponse),
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
	s.mu.Unlock()

	s.logger.Info(
		"adapter shell starting",
		"component", "adapter",
		"adapter_state", StateIdle,
		"ws_url", sanitizeWSURL(s.cfg.WSURL),
	)
	if s.cfg.AccessToken == "" {
		s.logger.Warn(
			"adapter access token is empty",
			"component", "adapter",
			"adapter_state", StateIdle,
			"ws_url", sanitizeWSURL(s.cfg.WSURL),
		)
	}

	go s.dispatchEvents(runCtx)
	go s.run(runCtx)
}

func (s *Shell) Stop(ctx context.Context) error {
	s.mu.Lock()
	cancel := s.cancel
	done := s.done
	conn := s.conn
	started := s.started
	s.mu.Unlock()

	if !started {
		s.markStopped()
		return nil
	}

	s.logger.Info(
		"adapter shell stopping",
		"component", "adapter",
		"adapter_state", s.Snapshot().State,
	)

	if cancel != nil {
		cancel()
	}
	if conn != nil {
		_ = conn.Close(websocket.StatusNormalClosure, "")
	}

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
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

func (s *Shell) run(ctx context.Context) {
	defer func() {
		s.clearConn(nil)
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

	if strings.TrimSpace(s.cfg.WSURL) == "" {
		s.logger.Info(
			"adapter connection is not configured",
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

func (s *Shell) runAttempt(ctx context.Context) (bool, bool) {
	s.markConnecting()
	s.logger.Info(
		"adapter connecting",
		"component", "adapter",
		"adapter_state", StateConnecting,
		"ws_url", sanitizeWSURL(s.cfg.WSURL),
	)

	conn, response, err := s.dial(ctx)
	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}
	if err != nil {
		if isAuthFailure(response) {
			s.markAuthFailed(err)
			s.logger.Error(
				"adapter authentication failed",
				"component", "adapter",
				"adapter_state", StateAuthFailed,
				"ws_url", sanitizeWSURL(s.cfg.WSURL),
				"error_code", errorCodeAuthFailed,
				"err", summarizeError(err),
			)

			<-ctx.Done()
			return false, true
		}

		if ctx.Err() != nil {
			return false, true
		}

		s.markReconnecting(errorCodeConnectionFail, err)
		return false, false
	}

	s.setConn(conn)
	defer func() {
		_ = conn.Close(websocket.StatusNormalClosure, "")
		s.clearConn(conn)
	}()

	ready, err := s.waitForReadyFrame(ctx, conn)
	if err != nil {
		if ctx.Err() != nil {
			return false, true
		}

		s.markReconnecting(errorCodeConnectionFail, err)
		return false, false
	}

	s.markConnected(ready.ObservedAt)
	s.logger.Info(
		"adapter connected",
		"component", "adapter",
		"adapter_state", StateConnected,
		"ws_url", sanitizeWSURL(s.cfg.WSURL),
	)
	if handler := s.currentReadyHandler(); handler != nil {
		go handler(ctx)
	}

	err = s.readLoop(ctx, conn)
	if err == nil {
		return true, true
	}
	if ctx.Err() != nil {
		return true, true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		s.logger.Warn(
			"adapter heartbeat timeout",
			"component", "adapter",
			"adapter_state", StateConnected,
			"error_code", errorCodeConnectionLost,
			"ws_url", sanitizeWSURL(s.cfg.WSURL),
		)
	}

	s.markReconnecting(errorCodeConnectionLost, err)
	return true, false
}

func (s *Shell) waitForReadyFrame(ctx context.Context, conn *websocket.Conn) (FrameSummary, error) {
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

		if err := s.recordAndValidateFrame(frame); err != nil {
			return FrameSummary{}, err
		}
		if isReadySummary(frame.Summary) {
			return frame.Summary, nil
		}

		waitingForFirstFrame = false
	}
}

func (s *Shell) readLoop(ctx context.Context, conn *websocket.Conn) error {
	for {
		readCtx, cancel := s.readContext(ctx)
		frame, err := s.readFrame(readCtx, conn)
		cancel()
		if err != nil {
			return err
		}

		if err := s.recordAndValidateFrame(frame); err != nil {
			return err
		}

		s.routeAPIResponse(frame)
		s.forwardSupportedEvent(ctx, frame)
	}
}

func (s *Shell) readContext(ctx context.Context) (context.Context, context.CancelFunc) {
	snapshot := s.Snapshot()
	timeout := s.provisionalReadTimeout(snapshot)
	return context.WithTimeout(ctx, timeout)
}

func (s *Shell) recordAndValidateFrame(frame classifiedFrame) error {
	snapshot := s.recordFrame(frame)

	switch {
	case frame.Summary.Category == FrameCategoryInvalid:
		s.logger.Warn(
			"adapter invalid frame received",
			"component", "adapter",
			"adapter_state", s.Snapshot().State,
			"frame_type", frame.Summary.Type,
			"invalid_frame_count", snapshot.InvalidReceivedFrames,
			"reason", frame.InvalidSummary,
			"ws_url", sanitizeWSURL(s.cfg.WSURL),
		)
		return fmt.Errorf("invalid frame: %s", frame.InvalidSummary)
	case isLifecycleDisable(frame.Frame):
		s.logger.Warn(
			"adapter lifecycle disable observed",
			"component", "adapter",
			"adapter_state", s.Snapshot().State,
			"frame_type", frame.Summary.Type,
			"ws_url", sanitizeWSURL(s.cfg.WSURL),
		)
	}

	return nil
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
	if s.cfg.AccessToken != "" {
		headers.Set("Authorization", "Bearer "+s.cfg.AccessToken)
	}

	return s.deps.dial(dialCtx, dialURL(s.cfg.WSURL, s.cfg.AccessToken), &websocket.DialOptions{
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

func (s *Shell) markConnecting() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.snapshot.State = StateConnecting
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
}

func (s *Shell) markConnected(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.snapshot.State = StateConnected
	s.snapshot.LastErrorCode = ""
	s.snapshot.LastErrorMessage = ""
	s.snapshot.ReadyFrameSeen = true
	s.snapshot.ConnectedAt = cloneTime(&now)
}

func (s *Shell) markAuthFailed(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.snapshot.State = StateAuthFailed
	s.snapshot.LastErrorCode = errorCodeAuthFailed
	s.snapshot.LastErrorMessage = summarizeError(err)
	s.snapshot.ReadyFrameSeen = false
	s.snapshot.ConnectedAt = nil
}

func (s *Shell) markReconnecting(code string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.snapshot.State = StateReconnecting
	s.snapshot.LastErrorCode = code
	s.snapshot.LastErrorMessage = summarizeError(err)
	s.snapshot.ReadyFrameSeen = false
	s.snapshot.ConnectedAt = nil
}

func (s *Shell) markStopped() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.snapshot.State = StateStopped
	s.snapshot.ReadyFrameSeen = false
	s.snapshot.ConnectedAt = nil
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

func maxInt(value, fallback int) int {
	if value <= 0 {
		return fallback
	}

	return value
}

func (s *Shell) forwardSupportedEvent(ctx context.Context, frame classifiedFrame) {
	if frame.Summary.Category != FrameCategoryEvent {
		return
	}

	normalizedEvent, ok := normalizeSupportedEvent(frame.Frame, frame.Summary.ObservedAt)
	if !ok {
		s.logger.Debug(
			"adapter event ignored by runtime bridge",
			"component", "adapter",
			"adapter_state", s.Snapshot().State,
			"frame_type", frame.Summary.Type,
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
