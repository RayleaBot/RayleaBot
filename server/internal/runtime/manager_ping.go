package runtime

func (m *Manager) registerPingRequest(handle *processHandle, requestID string) (*pingRequest, *Error) {
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
