package manager

import (
	"fmt"

	runtimeprocess "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/process"
)

func (m *Manager) watchRunningProcess(handle *runtimeprocess.Handle) {
	<-handle.Done()

	waitErr, _ := handle.ExitResult()

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
			fmt.Sprintf("插件%s运行时异常退出，累计崩溃 %d 次", pluginIDLabel(handle.Spec.PluginID), crashCount),
			"component", "runtime",
			"plugin_id", handle.Spec.PluginID,
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
		"插件"+pluginIDLabel(handle.Spec.PluginID)+"运行时已退出",
		"component", "runtime",
		"plugin_id", handle.Spec.PluginID,
		"runtime_state", string(StateStopped),
	)
}

func (m *Manager) reconcileExitedProcess(handle *runtimeprocess.Handle, waitErr error) {
	if waitErr != nil {
		m.markStopped(codePluginInternalError, "plugin exited unexpectedly", waitErr)
		return
	}

	m.markStopped("", "", nil)
}
