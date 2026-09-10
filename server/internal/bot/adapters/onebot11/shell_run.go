package onebot11

import (
	"context"
	"errors"
)

func (s *Shell) run(ctx context.Context) {
	defer func() {
		s.mu.Lock()
		s.stopping = true
		cancel := s.cancel
		reverseConn := s.reverseConn
		s.mu.Unlock()
		cancel()
		if reverseConn != nil {
			_ = reverseConn.CloseNow()
		}
		// Stop may time out, but this lifecycle stays active until every callback
		// exits, preventing a reload from starting a second event dispatcher.
		s.workers.Wait()
		s.clearConn(nil)
		s.markStopped()
		s.logger.Info(
			"消息平台连接已关闭",
			"component", "adapter",
			"adapter_state", StateStopped,
		)

		s.mu.Lock()
		if s.done != nil {
			close(s.done)
		}
		s.started = false
		s.cancel = nil
		s.runCtx = nil
		s.done = nil
		s.mu.Unlock()
	}()

	snapshot := s.Snapshot()
	if !snapshot.ForwardWS.Enabled || !snapshot.ForwardWS.Configured {
		s.logger.Debug(
			"未配置消息平台主动连接",
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
		failure := s.Snapshot()
		reason := failure.LastErrorMessage
		if reason == "" {
			reason = "连接会话意外结束"
		}
		s.logger.Warn(
			"消息平台连接断开，等待重连", "reason", reason,
			"component", "adapter",
			"adapter_state", StateReconnecting,
			"retry_in", delay.String(),
			"error_code", failure.LastErrorCode,
			"err", reason,
		)

		if err := s.deps.sleep(ctx, delay); err != nil {
			return
		}

		retryAttempt++
	}
}

func (s *Shell) runAttempt(ctx context.Context) (bool, bool) {
	s.markConnecting()
	s.logger.Debug(
		"正在连接消息平台",
		"component", "adapter",
		"adapter_state", StateConnecting,
		"transport", string(TransportForwardWS),
		"ws_url", sanitizeWSURL(s.forwardWSURL()),
	)

	conn, response, err := s.dial(ctx)
	if response != nil && response.Body != nil {
		defer func(release func() error) { _ = release() }(response.Body.Close)
	}
	if err != nil {
		if isAuthFailure(response) {
			s.markAuthFailed(err)
			errorSummary := summarizeError(err)
			s.logger.Error(
				"消息平台连接验证失败，已停止重连，请检查连接密钥后重启",
				"component", "adapter",
				"adapter_state", StateAuthFailed,
				"transport", string(TransportForwardWS),
				"ws_url", sanitizeWSURL(s.forwardWSURL()),
				"error_code", errorCodeForwardWSConnectFail,
				"err", errorSummary,
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
		_ = conn.CloseNow()
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
		"消息平台已连接",
		"component", "adapter",
		"adapter_state", StateConnected,
		"transport", string(TransportForwardWS),
		"ws_url", sanitizeWSURL(s.forwardWSURL()),
	)
	s.startWorker(func(ctx context.Context) { s.refreshRuntimeInfo(ctx, TransportForwardWS) })
	if handler := s.currentReadyHandler(); handler != nil {
		s.startWorker(handler)
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
			"消息平台连接无响应，正在重新连接。",
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
