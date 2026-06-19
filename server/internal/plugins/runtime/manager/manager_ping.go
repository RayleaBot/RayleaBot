package manager

import (
	"context"
	"time"

	runtimeprocess "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/process"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
)

func (m *Manager) Ping(ctx context.Context) error {
	m.mu.RLock()
	handle := m.proc
	m.mu.RUnlock()
	if handle == nil {
		return errorf(codePlatformInvalidRequest, "plugin runtime is not running", nil)
	}

	requestID := m.deps.requestID()
	request, runtimeErr := m.registerPingRequest(handle, requestID)
	if runtimeErr != nil {
		return runtimeErr
	}

	if err := handle.WriteJSONLine(runtimeprotocol.PingFrame{
		ProtocolVersion: "1",
		Type:            "ping",
		Timestamp:       m.deps.now().Unix(),
		PluginID:        handle.Spec.PluginID,
		RequestID:       requestID,
	}); err != nil {
		m.mu.Lock()
		delete(m.pendingPings, requestID)
		m.mu.Unlock()
		return m.failRuntime(handle, codePluginInternalError, "write ping frame", err)
	}

	timeout := handle.Spec.EventTimeout
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining > 0 && remaining < timeout {
			timeout = remaining
		}
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case err := <-request.done:
		return err
	case <-timer.C:
		return m.failRuntime(handle, codePluginEventTimeout, "plugin pong response timed out", nil)
	case <-ctx.Done():
		return m.failRuntime(handle, codePluginEventTimeout, "plugin pong response timed out", ctx.Err())
	}
}

func (m *Manager) registerPingRequest(handle *runtimeprocess.Handle, requestID string) (*pingRequest, *Error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.proc != handle || handle == nil {
		return nil, errorf(codePlatformInvalidRequest, "plugin runtime is not running", nil)
	}
	if m.snap.State == StateStopping {
		return nil, errorf(codePluginStopping, "plugin runtime is stopping", nil)
	}
	if m.snap.State != StateRunning {
		return nil, errorf(codePlatformInvalidRequest, "plugin runtime is not ready for ping", nil)
	}

	request := &pingRequest{done: make(chan error, 1)}
	m.pendingPings[requestID] = request
	return request, nil
}

func (m *Manager) completePingLocked(requestID string, request *pingRequest, err error) {
	if request == nil || request.completed {
		return
	}
	request.completed = true
	request.err = err
	delete(m.pendingPings, requestID)
	request.done <- err
	close(request.done)
}
