package runtime

import (
	"bufio"
	"context"
	"errors"
	"os"
	"os/exec"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/logpath"
)

func (m *Manager) Start(ctx context.Context, spec Spec, payload InitPayload) error {
	if len(payload.CommandPrefixes) == 0 {
		return errorf(codePlatformInvalidRequest, "init payload command_prefixes is required", nil)
	}

	m.mu.Lock()
	if m.proc != nil {
		m.mu.Unlock()
		return errorf(codePluginInternalError, "plugin runtime is already active", nil)
	}

	startedAt := m.deps.now()
	requestID := m.deps.requestID()
	crashCount := m.snap.CrashCount
	m.snap = Snapshot{
		PluginID:      spec.PluginID,
		State:         StateStarting,
		InitRequestID: requestID,
		StartedAt:     &startedAt,
		CrashCount:    crashCount,
	}
	m.expiredEvents = make(map[string]time.Time)
	m.mu.Unlock()

	cmd := exec.Command(spec.Command, spec.Args...)
	cmd.Dir = spec.WorkDir
	cmd.Env = append([]string(nil), os.Environ()...)
	if len(spec.Env) > 0 {
		cmd.Env = append(cmd.Env, spec.Env...)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		m.markStopped(codePluginInternalError, "open plugin stdin", err)
		return errorf(codePluginInternalError, "open plugin stdin", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		m.markStopped(codePluginInternalError, "open plugin stdout", err)
		return errorf(codePluginInternalError, "open plugin stdout", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		m.markStopped(codePluginInternalError, "open plugin stderr", err)
		return errorf(codePluginInternalError, "open plugin stderr", err)
	}

	if err := cmd.Start(); err != nil {
		m.markStopped(codePluginInternalError, "start plugin process", err)
		return errorf(codePluginInternalError, "start plugin process", err)
	}

	go m.captureStderr(spec.PluginID, stderr)

	handle := NewHandle(cmd, stdin, bufio.NewReader(stdout), processSpec(spec))
	go handle.Watch()

	m.mu.Lock()
	m.proc = handle
	m.snap.PID = cmd.Process.Pid
	m.mu.Unlock()

	entryPathDisplay := logpath.Display(spec.RepoRoot, spec.EntryPath)
	m.logger.Info(
		"插件"+runtimePluginLabel(spec)+"运行时正在启动，入口文件："+entryPathDisplay,
		"component", "runtime",
		"plugin_id", spec.PluginID,
		"plugin_name", spec.PluginName,
		"runtime_state", string(StateStarting),
		"entry_path", entryPathDisplay,
	)

	var bot *BotFrame
	if payload.Bot.ID != "" {
		bot = &BotFrame{
			ID:       payload.Bot.ID,
			Nickname: payload.Bot.Nickname,
		}
	}
	var permissions *PermissionsFrame
	if len(payload.SuperAdmins) > 0 {
		permissions = &PermissionsFrame{
			SuperAdmins: append([]string(nil), payload.SuperAdmins...),
		}
	}

	if err := handle.WriteJSONLine(InitFrame{
		ProtocolVersion: "1",
		Type:            "init",
		Timestamp:       m.deps.now().Unix(),
		PluginID:        spec.PluginID,
		RequestID:       requestID,
		Bot:             bot,
		Capabilities:    append([]string(nil), payload.Capabilities...),
		Permissions:     permissions,
		CommandPrefixes: append([]string(nil), payload.CommandPrefixes...),
	}); err != nil {
		m.cleanupFailedStart(handle, codePluginInternalError, "write init frame", err)
		return errorf(codePluginInternalError, "write init frame", err)
	}

	subscriptions, runtimeErr := m.awaitInitAck(ctx, handle, requestID)
	if runtimeErr != nil {
		m.cleanupFailedStart(handle, runtimeErr.Code, runtimeErr.Message, runtimeErr.Err)
		return runtimeErr
	}

	m.mu.Lock()
	if m.proc == handle {
		m.snap.State = StateRunning
		m.snap.LastErrorCode = ""
		m.snap.LastErrorMessage = ""
		m.snap.Subscriptions = append([]string(nil), subscriptions...)
	}
	m.mu.Unlock()

	m.logger.Info(
		"插件"+runtimePluginLabel(spec)+"运行时已启动，入口文件："+entryPathDisplay,
		"component", "runtime",
		"plugin_id", spec.PluginID,
		"plugin_name", spec.PluginName,
		"runtime_state", string(StateRunning),
		"entry_path", entryPathDisplay,
	)

	go m.readRuntimeFrames(handle)
	go m.watchRunningProcess(handle)

	return nil
}

func processSpec(spec Spec) ProcessSpec {
	return ProcessSpec{
		PluginID:             spec.PluginID,
		InitTimeout:          spec.InitTimeout,
		InitMaxTotal:         spec.InitMaxTotal,
		EventTimeout:         spec.EventTimeout,
		ShutdownGrace:        spec.ShutdownGrace,
		EffectiveConcurrency: spec.EffectiveConcurrency,
	}
}

func (m *Manager) readRuntimeFrames(handle *Handle) {
	for {
		line, err := handle.Stdout.ReadBytes('\n')
		if err != nil {
			runtimeErr := classifyProtocolReadError(handle, err, "plugin exited during runtime delivery", "read plugin runtime response")
			if errorsAreExitLike(handle, err) {
				m.signalPendingRequests(handle, runtimeErr)
				return
			}
			_ = m.failRuntime(handle, runtimeErr.Code, runtimeErr.Message, runtimeErr.Err)
			return
		}

		m.protocolMu.Lock()
		runtimeErr := m.routeRuntimeFrame(handle, line)
		m.protocolMu.Unlock()
		if runtimeErr != nil {
			_ = m.failRuntime(handle, runtimeErr.Code, runtimeErr.Message, runtimeErr.Err)
			return
		}
	}
}

func errorsAreExitLike(handle *Handle, err error) bool {
	if isProcessPipeClosedError(err) {
		return true
	}
	if handle == nil {
		return false
	}
	_, exited := handle.ExitResult()
	return exited
}

func (m *Manager) routeRuntimeFrame(handle *Handle, line []byte) *Error {
	envelope, err := parseEventEnvelope(line, handle.Spec.PluginID)
	if err != nil {
		return normalizeRuntimeError(err, "parse runtime frame envelope")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.proc != handle {
		return nil
	}

	if ping := m.pendingPings[envelope.RequestID]; ping != nil {
		if envelope.Type != "pong" {
			return errorf(codePluginProtocolViolation, "plugin returned unexpected frame type in response to ping", nil)
		}
		m.completePingLocked(envelope.RequestID, ping, nil)
		return nil
	}

	if session := m.pendingEvents[envelope.RequestID]; session != nil {
		return m.routeTerminalFrameLocked(session, envelope, line)
	}

	if m.eventExpiredLocked(envelope.RequestID) && (envelope.Type == "result" || envelope.Type == "error") {
		return nil
	}

	if envelope.Type == "action" {
		return m.routeLocalActionFrameLocked(handle, line)
	}

	return errorf(codePluginProtocolViolation, "plugin returned an unexpected protocol message during runtime delivery", nil)
}

func (m *Manager) routeTerminalFrameLocked(session *eventSession, envelope FrameEnvelope, line []byte) *Error {
	if session.pendingLocalAction > 0 {
		return errorf(codePluginProtocolViolation, "plugin returned a terminal frame before all local actions completed", nil)
	}

	delivery, done, err := decodeTerminalDelivery(session.requestID, line, envelope.Type)
	if !done {
		return errorf(codePluginProtocolViolation, "plugin returned an unexpected non-terminal frame for the active event", nil)
	}
	if err != nil {
		var runtimeErr *Error
		if ok := asRuntimeError(err, &runtimeErr); ok {
			m.completeEventLocked(session, delivery, runtimeErr)
			return nil
		}
		m.completeEventLocked(session, delivery, errorf(codePluginInternalError, "terminal frame returned unexpected error", err))
		return nil
	}

	m.completeEventLocked(session, delivery, nil)
	return nil
}

func asRuntimeError(err error, target **Error) bool {
	if err == nil {
		return false
	}
	var runtimeErr *Error
	if !errors.As(err, &runtimeErr) {
		return false
	}
	*target = runtimeErr
	return true
}

func normalizeRuntimeError(err error, message string) *Error {
	if err == nil {
		return nil
	}
	var runtimeErr *Error
	if errors.As(err, &runtimeErr) {
		return runtimeErr
	}
	var actionErr *Error
	if errors.As(err, &actionErr) {
		return errorf(actionErr.Code, actionErr.Message, actionErr.Err)
	}
	return errorf(codePluginInternalError, message, err)
}
