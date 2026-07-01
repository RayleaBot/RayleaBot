package manager

import (
	"context"
	"time"

	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
)

func (m *Manager) DeliverEvent(ctx context.Context, event runtimeprotocol.Event) (Delivery, error) {
	if event.EventID == "" || event.SourceProtocol == "" || event.SourceAdapter == "" || event.EventType == "" || event.Timestamp <= 0 {
		return Delivery{}, errorf(codePlatformInvalidRequest, "event payload is missing required fields", nil)
	}

	m.mu.RLock()
	handle := m.proc
	m.mu.RUnlock()
	if handle == nil {
		return Delivery{}, errorf(codePlatformInvalidRequest, "plugin runtime is not running", nil)
	}

	requestID := m.deps.requestID()
	session, runtimeErr := m.registerEventSession(ctx, handle, requestID, event)
	if runtimeErr != nil {
		return Delivery{}, runtimeErr
	}

	frame := runtimeprotocol.BuildEventFrame(event, handle.Spec.PluginID, requestID, m.deps.now().Unix())
	if err := handle.WriteJSONLine(frame); err != nil {
		m.removeEventSession(handle, requestID)
		return Delivery{}, m.failRuntime(handle, codePluginInternalError, "write event frame", err)
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
	case <-session.done:
		if session.err != nil {
			return session.delivery, session.err
		}
		return session.delivery, nil
	case <-timer.C:
		return m.timeoutEvent(handle, session, codePluginEventTimeout, "plugin event response timed out", nil)
	case <-ctx.Done():
		return m.timeoutEvent(handle, session, codePluginEventTimeout, "plugin event response timed out", ctx.Err())
	}
}
