package runtime

func (m *Manager) watchRunningProcess(handle *processHandle) {
	<-handle.done

	waitErr, _ := handle.exitResult()

	m.mu.RLock()
	if m.proc != handle || m.snap.State != StateRunning {
		m.mu.RUnlock()
		return
	}
	m.mu.RUnlock()

	if waitErr != nil {
		m.mu.Lock()
		m.snap.CrashCount++
		crashCount := m.snap.CrashCount
		m.snap.State = StateCrashed
		now := m.deps.now()
		m.snap.StoppedAt = &now
		m.snap.LastErrorCode = codePluginInternalError
		m.snap.LastErrorMessage = "plugin exited unexpectedly"
		pluginID := m.snap.PluginID
		m.proc = nil
		m.mu.Unlock()

		m.logger.Warn(
			"plugin runtime crashed",
			"component", "runtime",
			"plugin_id", handle.spec.PluginID,
			"runtime_state", string(StateCrashed),
			"crash_count", crashCount,
			"err", waitErr.Error(),
		)

		if m.opts.OnCrash != nil {
			m.opts.OnCrash(pluginID, crashCount, codePluginInternalError)
		}
		return
	}

	m.markStopped("", "", nil)
	m.logger.Info(
		"plugin runtime exited",
		"component", "runtime",
		"plugin_id", handle.spec.PluginID,
		"runtime_state", string(StateStopped),
	)
}

func (m *Manager) reconcileExitedProcess(handle *processHandle, waitErr error) {
	if waitErr != nil {
		m.markStopped(codePluginInternalError, "plugin exited unexpectedly", waitErr)
		return
	}

	m.markStopped("", "", nil)
}
