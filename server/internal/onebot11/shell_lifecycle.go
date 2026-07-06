package onebot11

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"net/http"
	"time"

	"github.com/coder/websocket"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

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
		"OneBot 适配器正在启动，主动 WebSocket 地址："+sanitizeWSURL(s.forwardWSURL()),
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
		"OneBot 适配器正在停止，当前状态："+string(s.Snapshot().State),
		"component", "adapter",
		"adapter_state", s.Snapshot().State,
	)

	if err := waitForClosed(ctx, done); err != nil {
		return err
	}
	return waitForClosed(ctx, reverseDone)
}

func (s *Shell) Reload(nextCfg config.OneBotConfig, nextAdapterCfg config.AdapterConfig) error {
	if s == nil {
		return nil
	}

	s.mu.RLock()
	started := s.started
	supervisorCtx := s.supervisorCtx
	previousCfg := s.cfg
	previousAdapterCfg := s.adapterCfg
	s.mu.RUnlock()

	if started {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.Stop(stopCtx); err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
	}

	s.applyConfig(nextCfg, nextAdapterCfg, previousCfg, previousAdapterCfg)
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

func (s *Shell) applyConfig(nextCfg config.OneBotConfig, nextAdapterCfg config.AdapterConfig, previousCfg config.OneBotConfig, previousAdapterCfg config.AdapterConfig) {
	s.mu.Lock()
	s.cfg = nextCfg
	s.adapterCfg = nextAdapterCfg
	s.deps.connectTimeout = nextConnectTimeout(previousAdapterCfg, nextAdapterCfg, s.deps.connectTimeout)
	s.deps.backoff = nextBackoff(previousAdapterCfg, nextAdapterCfg, s.deps.backoff)
	s.httpClient = &http.Client{
		Timeout: s.deps.connectTimeout,
	}
	s.snapshot = newTransportSnapshot(nextCfg)
	s.pendingResponses = make(map[string]chan APIResponse)
	s.recentEventIDs = make(map[string]time.Time)
	s.identityCache = NewIdentityCache(defaultIdentityCacheTTL)
	snapshot := cloneSnapshot(s.snapshot)
	handler := s.stateHandler
	s.mu.Unlock()

	s.emitStateSnapshot(handler, snapshot)
}

func nextConnectTimeout(previousCfg config.AdapterConfig, nextCfg config.AdapterConfig, current time.Duration) time.Duration {
	if nextCfg.ConnectTimeoutSeconds == previousCfg.ConnectTimeoutSeconds && current > 0 {
		return current
	}

	return time.Duration(maxInt(nextCfg.ConnectTimeoutSeconds, 1)) * time.Second
}

func nextBackoff(previousCfg config.AdapterConfig, nextCfg config.AdapterConfig, current *Backoff) *Backoff {
	if reconnectSettingsEqual(previousCfg, nextCfg) && current != nil {
		return current
	}

	var randFloat func() float64
	if current != nil {
		randFloat = current.RandFloat()
	}

	return NewBackoff(
		nextCfg.ReconnectInitialSeconds,
		nextCfg.ReconnectMultiplier,
		nextCfg.ReconnectMaxSeconds,
		nextCfg.ReconnectJitterRatio,
		randFloat,
	)
}

func reconnectSettingsEqual(left config.AdapterConfig, right config.AdapterConfig) bool {
	return left.ReconnectInitialSeconds == right.ReconnectInitialSeconds &&
		left.ReconnectMultiplier == right.ReconnectMultiplier &&
		left.ReconnectMaxSeconds == right.ReconnectMaxSeconds &&
		left.ReconnectJitterRatio == right.ReconnectJitterRatio
}

func (s *Shell) AttachReverseWS(conn *websocket.Conn) {
	if conn == nil {
		return
	}

	done := make(chan struct{})
	var previous *websocket.Conn

	s.mu.Lock()
	if s.stopping || !s.started {
		s.mu.Unlock()
		_ = conn.Close(websocket.StatusNormalClosure, "")
		return
	}
	if s.reverseConn != nil {
		previous = s.reverseConn
	}
	s.reverseConn = conn
	s.reverseDone = done
	s.mu.Unlock()

	if previous != nil {
		_ = previous.Close(websocket.StatusNormalClosure, "")
	}

	go s.handleReverseSession(conn, done)
}

func (s *Shell) handleReverseSession(conn *websocket.Conn, done chan struct{}) {
	ctx := context.Background()
	defer func() {
		defer close(done)
		_ = conn.Close(websocket.StatusNormalClosure, "")
		s.mu.Lock()
		current := s.reverseConn == conn
		if current {
			s.reverseConn = nil
			if s.reverseDone == done {
				s.reverseDone = nil
			}
		}
		if !current && !s.stopping {
			s.mu.Unlock()
			return
		}
		s.clearTransportRuntimeInfoLocked(TransportReverseWS)
		if s.stopping && s.snapshot.ReverseWS.Enabled && s.snapshot.ReverseWS.Configured {
			s.snapshot.ReverseWS.State = TransportStateStopped
		} else if s.snapshot.ReverseWS.Enabled && s.snapshot.ReverseWS.Configured {
			s.snapshot.ReverseWS.State = TransportStateListening
		} else {
			s.snapshot.ReverseWS.State = TransportStateIdle
		}
		s.refreshAggregateStateLocked()
		snapshot := cloneSnapshot(s.snapshot)
		handler := s.stateHandler
		s.mu.Unlock()
		s.emitStateSnapshot(handler, snapshot)
	}()

	ready, err := s.waitForReadyFrame(ctx, TransportReverseWS, conn)
	if err != nil {
		if ctx.Err() != nil || s.isStopping() {
			return
		}
		s.markTransportFailure(TransportReverseWS, TransportStateListening, errorCodeConnectionLost, err)
		return
	}

	s.mu.Lock()
	s.snapshot.ReverseWS.State = TransportStateConnected
	s.snapshot.ReverseWS.LastErrorCode = ""
	s.snapshot.ReverseWS.LastErrorMessage = ""
	s.snapshot.ReadyFrameSeen = true
	s.snapshot.ConnectedAt = cloneTime(&ready.ObservedAt)
	s.syncLastErrorLocked()
	s.refreshAggregateStateLocked()
	snapshot := cloneSnapshot(s.snapshot)
	handler := s.stateHandler
	s.mu.Unlock()
	s.emitStateSnapshot(handler, snapshot)
	go s.refreshRuntimeInfo(ctx, TransportReverseWS)

	if readyHandler := s.currentReadyHandler(); readyHandler != nil {
		go readyHandler(ctx)
	}

	if err := s.readLoop(ctx, TransportReverseWS, conn); err != nil && ctx.Err() == nil && !s.isStopping() {
		s.markTransportFailure(TransportReverseWS, TransportStateListening, errorCodeConnectionLost, err)
	}
}

func (s *Shell) isStopping() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stopping
}

type Backoff struct {
	initial time.Duration
	max     time.Duration

	multiplier float64
	jitter     float64
	randFloat  func() float64
}

func NewBackoff(initialSeconds int, multiplier float64, maxSeconds int, jitterRatio float64, randFloat func() float64) *Backoff {
	return NewWithDurations(
		time.Duration(initialSeconds)*time.Second,
		multiplier,
		time.Duration(maxSeconds)*time.Second,
		jitterRatio,
		randFloat,
	)
}

func NewWithDurations(initial time.Duration, multiplier float64, maxDelay time.Duration, jitterRatio float64, randFloat func() float64) *Backoff {
	if initial <= 0 {
		initial = time.Second
	}

	if maxDelay <= 0 {
		maxDelay = initial
	}
	if maxDelay < initial {
		maxDelay = initial
	}

	if multiplier < 1 {
		multiplier = 1
	}
	if jitterRatio < 0 {
		jitterRatio = 0
	}
	if jitterRatio > 1 {
		jitterRatio = 1
	}
	if randFloat == nil {
		rng := rand.New(rand.NewSource(time.Now().UnixNano()))
		randFloat = rng.Float64
	}

	return &Backoff{
		initial:    initial,
		max:        maxDelay,
		multiplier: multiplier,
		jitter:     jitterRatio,
		randFloat:  randFloat,
	}
}

func (b *Backoff) RandFloat() func() float64 {
	if b == nil {
		return nil
	}
	return b.randFloat
}

func (b *Backoff) Duration(attempt int) time.Duration {
	if b == nil {
		return time.Second
	}

	base := float64(b.initial)
	maxDelay := float64(b.max)

	for i := 0; i < attempt; i++ {
		base *= b.multiplier
		if base >= maxDelay {
			base = maxDelay
			break
		}
	}

	jittered := base
	if b.jitter > 0 {
		factor := 1 - b.jitter + (2 * b.jitter * b.randFloat())
		jittered = base * factor
	}

	if jittered < 0 {
		jittered = 0
	}
	if jittered > maxDelay {
		jittered = maxDelay
	}

	return time.Duration(math.Round(jittered))
}
