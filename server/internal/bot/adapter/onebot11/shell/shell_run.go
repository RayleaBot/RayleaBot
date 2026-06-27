package shell

import (
	"context"
	"errors"

	"github.com/coder/websocket"
)

func (s *Shell) run(ctx context.Context) {
	defer func() {
		s.clearConn(nil)
		s.clearReverseConn(nil)
		s.markStopped()
		s.logger.Info(
			"OneBot 适配器已停止",
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
			"OneBot 主动 WebSocket 未启用或未配置，适配器保持空闲",
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
			"OneBot 主动 WebSocket 连接断开，将在 "+delay.String()+" 后重连："+sanitizeWSURL(s.forwardWSURL()),
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
		"OneBot 主动 WebSocket 正在连接："+sanitizeWSURL(s.forwardWSURL()),
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
				"OneBot 主动 WebSocket 鉴权失败："+sanitizeWSURL(s.forwardWSURL()),
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
		"OneBot 主动 WebSocket 已连接："+sanitizeWSURL(s.forwardWSURL()),
		"component", "adapter",
		"adapter_state", StateConnected,
		"transport", string(TransportForwardWS),
		"ws_url", sanitizeWSURL(s.forwardWSURL()),
	)
	go s.refreshRuntimeInfo(ctx, TransportForwardWS)
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
			"OneBot 主动 WebSocket 心跳超时，准备重连："+sanitizeWSURL(s.forwardWSURL()),
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
