package shell

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/coder/websocket"

	adaptercache "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/cache"
	adapteroutbound "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/outbound"
	adapterbackoff "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/shell/backoff"
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
	s.pendingResponses = make(map[string]chan adapteroutbound.APIResponse)
	s.recentEventIDs = make(map[string]time.Time)
	s.identityCache = adaptercache.NewIdentityCache(defaultIdentityCacheTTL)
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

func nextBackoff(previousCfg config.AdapterConfig, nextCfg config.AdapterConfig, current *adapterbackoff.Backoff) *adapterbackoff.Backoff {
	if reconnectSettingsEqual(previousCfg, nextCfg) && current != nil {
		return current
	}

	var randFloat func() float64
	if current != nil {
		randFloat = current.RandFloat()
	}

	return adapterbackoff.New(
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
