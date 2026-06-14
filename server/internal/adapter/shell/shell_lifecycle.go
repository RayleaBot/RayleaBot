package shell

import (
	"context"
	"errors"
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
