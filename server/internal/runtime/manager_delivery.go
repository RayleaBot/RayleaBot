package runtime

import (
	"context"
	"time"
)

func (m *Manager) DeliverEvent(ctx context.Context, event Event) (Delivery, error) {
	if event.EventID == "" || event.SourceProtocol == "" || event.SourceAdapter == "" || event.EventType == "" || event.Timestamp <= 0 {
		return Delivery{}, errorf(codePlatformInvalidRequest, "event payload is missing required fields", nil)
	}

	m.deliverMu.Lock()
	defer m.deliverMu.Unlock()

	m.mu.RLock()
	handle := m.proc
	snapshot := m.snap
	m.mu.RUnlock()

	if handle == nil || snapshot.State == StateStopped {
		return Delivery{}, errorf(codePlatformInvalidRequest, "plugin runtime is not running", nil)
	}
	if snapshot.State == StateStopping {
		return Delivery{}, errorf(codePluginStopping, "plugin runtime is stopping", nil)
	}
	if snapshot.State != StateRunning {
		return Delivery{}, errorf(codePlatformInvalidRequest, "plugin runtime is not ready for event delivery", nil)
	}

	requestID := m.deps.requestID()
	frame := buildEventFrame(event, handle.spec.PluginID, requestID, m.deps.now().Unix())
	if err := writeJSONLine(handle.stdin, frame); err != nil {
		return Delivery{}, errorf(codePluginInternalError, "write event frame", err)
	}

	return m.awaitEventResponse(ctx, handle, requestID)
}

func (m *Manager) awaitEventResponse(ctx context.Context, handle *processHandle, requestID string) (Delivery, error) {
	timeout := handle.spec.EventTimeout
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining > 0 && remaining < timeout {
			timeout = remaining
		}
	}
	deadline := time.Now().Add(timeout)
	seenLocalRequestIDs := make(map[string]struct{})

	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			runtimeErr := errorf(codePluginEventTimeout, "plugin event response timed out", nil)
			m.cleanupFailedDelivery(handle, runtimeErr.Code, runtimeErr.Message, runtimeErr.Err)
			return Delivery{}, runtimeErr
		}

		readCh := make(chan []byte, 1)
		readErrCh := make(chan error, 1)
		go func() {
			line, err := handle.stdout.ReadBytes('\n')
			if err != nil {
				readErrCh <- err
				return
			}
			readCh <- line
		}()

		timer := time.NewTimer(remaining)
		select {
		case line := <-readCh:
			timer.Stop()
			delivery, done, err := m.processEventFrame(ctx, handle, requestID, seenLocalRequestIDs, line)
			if err != nil {
				if done {
					return delivery, err
				}
				return Delivery{}, err
			}
			if done {
				return delivery, nil
			}
		case readErr := <-readErrCh:
			timer.Stop()
			return Delivery{}, classifyProtocolReadError(handle, readErr, "plugin exited during event delivery", "read plugin event response")
		case <-handle.done:
			timer.Stop()
			waitErr, _ := handle.exitResult()
			if waitErr == nil {
				return Delivery{}, errorf(codePluginInternalError, "plugin exited during event delivery", nil)
			}
			return Delivery{}, errorf(codePluginInternalError, "plugin exited during event delivery", waitErr)
		case <-timer.C:
			runtimeErr := errorf(codePluginEventTimeout, "plugin event response timed out", nil)
			m.cleanupFailedDelivery(handle, runtimeErr.Code, runtimeErr.Message, runtimeErr.Err)
			return Delivery{}, runtimeErr
		case <-ctx.Done():
			timer.Stop()
			runtimeErr := errorf(codePluginEventTimeout, "plugin event response timed out", ctx.Err())
			m.cleanupFailedDelivery(handle, runtimeErr.Code, runtimeErr.Message, runtimeErr.Err)
			return Delivery{}, runtimeErr
		}
	}
}
