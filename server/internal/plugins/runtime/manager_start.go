package runtime

import (
	"bufio"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/logpath"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/pluginwire"
)

func (m *Manager) Start(ctx context.Context, spec Spec, payload InitPayload) error {
	if err := m.acquireLifecycle(ctx); err != nil {
		return err
	}
	defer func() { <-m.lifecycleGate }()
	if len(payload.CommandPrefixes) == 0 {
		return errorf(codePlatformInvalidRequest, "init payload command_prefixes is required", nil)
	}
	if strings.TrimSpace(payload.Timezone) == "" || payload.Timezone == "Local" {
		return errorf(codePlatformInvalidRequest, "init payload timezone must be an IANA timezone", nil)
	}
	if _, err := time.LoadLocation(payload.Timezone); err != nil {
		return errorf(codePlatformInvalidRequest, "init payload timezone is invalid", err)
	}

	m.mu.Lock()
	if m.proc != nil || m.snap.State == StateStarting {
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
	m.pendingLocalActions = 0
	m.actionBurstStarted = time.Time{}
	m.actionBurstCount = 0
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
	started := false
	defer func() {
		if !started {
			_ = stdin.Close()
			if childInput, ok := cmd.Stdin.(*os.File); ok {
				_ = childInput.Close()
			}
		}
	}()

	// Own the output readers: exec.Cmd.Wait closes StdoutPipe/StderrPipe
	// before protocol consumers necessarily inspect the child's final frames.
	stdout, stdoutWriter, err := os.Pipe()
	if err != nil {
		m.markStopped(codePluginInternalError, "open plugin stdout", err)
		return errorf(codePluginInternalError, "open plugin stdout", err)
	}
	defer func() {
		_ = stdoutWriter.Close()
		if !started {
			_ = stdout.Close()
		}
	}()
	cmd.Stdout = stdoutWriter

	stderr, stderrWriter, err := os.Pipe()
	if err != nil {
		m.markStopped(codePluginInternalError, "open plugin stderr", err)
		return errorf(codePluginInternalError, "open plugin stderr", err)
	}
	defer func() {
		_ = stderrWriter.Close()
		if !started {
			_ = stderr.Close()
		}
	}()
	cmd.Stderr = stderrWriter

	if err := cmd.Start(); err != nil {
		m.markStopped(codePluginInternalError, "start plugin process", err)
		return errorf(codePluginInternalError, "start plugin process", err)
	}
	started = true
	// Only the child keeps write ends, so normal process exit produces EOF.
	_ = stdoutWriter.Close()
	_ = stderrWriter.Close()

	handle := NewHandle(cmd, stdin, bufio.NewReader(stdout), processSpec(spec))
	runtimeReading := false
	defer func() {
		if !runtimeReading {
			handle.closeStdout()
			close(handle.protocolDone)
		}
	}()
	handle.stdout = stdout
	handle.stderr = stderr
	handle.stderrDone = make(chan struct{})
	go func() {
		defer close(handle.stderrDone)
		m.captureStderr(spec.PluginID, stderr)
	}()
	go handle.Watch()

	m.mu.Lock()
	m.proc = handle
	m.snap.PID = cmd.Process.Pid
	m.mu.Unlock()

	entryPathDisplay := logpath.Display(spec.RepoRoot, spec.EntryPath)
	m.logger.Debug(
		"插件正在启动", "plugin_label", runtimePluginLabel(spec),
		"component", "runtime",
		"plugin_id", spec.PluginID,
		"plugin_name", spec.PluginName,
		"runtime_state", string(StateStarting),
		"entry_path", entryPathDisplay,
	)

	bots := wireBotIdentities(payload.Bots)
	initConfig := cloneDetails(payload.Config)
	if initConfig == nil {
		initConfig = map[string]any{}
	}
	initCtx, cancelInit := context.WithTimeout(ctx, durationOrFallback(spec.InitTimeout, 10*time.Second))
	defer cancelInit()
	stopInitPipe := context.AfterFunc(initCtx, func() { _ = handle.Stdin.Close() })
	defer stopInitPipe()
	if err := handle.WriteJSONLine(pluginwire.InitFrame{
		Timezone:             payload.Timezone,
		ProtocolVersion:      pluginwire.ProtocolVersion,
		Type:                 "init",
		PluginID:             spec.PluginID,
		RequestID:            requestID,
		Bots:                 bots,
		Config:               initConfig,
		EffectivePermissions: append([]string{}, payload.Permissions...),
		SuperAdmins:          append([]string{}, payload.SuperAdmins...),
		CommandPrefixes:      append([]string(nil), payload.CommandPrefixes...),
		Concurrency:          spec.EffectiveConcurrency,
	}); err != nil {
		if initCtx.Err() != nil {
			m.cleanupFailedStart(handle, codePluginInitTimeout, "plugin initialization timed out", initCtx.Err())
			return errorf(codePluginInitTimeout, "plugin initialization timed out", initCtx.Err())
		}
		m.cleanupFailedStart(handle, codePluginInternalError, "write init frame", err)
		return errorf(codePluginInternalError, "write init frame", err)
	}

	runtimeErr := m.awaitInitAck(initCtx, handle, requestID)
	if runtimeErr != nil {
		m.cleanupFailedStart(handle, runtimeErr.Code, runtimeErr.Message, runtimeErr.Err)
		return runtimeErr
	}

	m.mu.Lock()
	if m.proc == handle {
		m.snap.State = StateRunning
		m.snap.LastErrorCode = ""
		m.snap.LastErrorMessage = ""
	}
	m.mu.Unlock()

	m.logger.Info(
		"插件已启动", "plugin_label", runtimePluginLabel(spec),
		"component", "runtime",
		"plugin_id", spec.PluginID,
		"plugin_name", spec.PluginName,
		"runtime_state", string(StateRunning),
		"entry_path", entryPathDisplay,
	)

	runtimeReading = true
	go m.readRuntimeFrames(handle)
	go m.watchRunningProcess(handle)

	return nil
}

func processSpec(spec Spec) ProcessSpec {
	return ProcessSpec{
		PluginID:             spec.PluginID,
		InitTimeout:          spec.InitTimeout,
		EventTimeout:         spec.EventTimeout,
		ShutdownGrace:        spec.ShutdownGrace,
		EffectiveConcurrency: spec.EffectiveConcurrency,
		IPCPendingActionsMax: positiveInt(spec.IPCPendingActionsMax, 256),
		IPCActionBurstCount:  positiveInt(spec.IPCActionBurstCount, 100),
		IPCActionBurstWindow: durationOrFallback(spec.IPCActionBurstWindow, time.Second),
		IPCMessageMaxBytes:   positiveInt(spec.IPCMessageMaxBytes, 8*1024*1024),
	}
}

func durationOrFallback(value, fallback time.Duration) time.Duration {
	if value <= 0 {
		return fallback
	}
	return value
}

func (m *Manager) readRuntimeFrames(handle *Handle) {
	defer func() {
		handle.closeStdout()
		if handle.protocolDone != nil {
			close(handle.protocolDone)
		}
	}()
	for {
		line, err := readProtocolLine(handle.Stdout, handle.Spec.IPCMessageMaxBytes)
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
		rejection, runtimeErr := m.routeRuntimeFrame(handle, line)
		if runtimeErr == nil && rejection != nil {
			runtimeErr = m.writeLocalRejectionLocked(handle, *rejection)
		}
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

func (m *Manager) routeRuntimeFrame(handle *Handle, line []byte) (*localActionRejection, *plugins.Error) {
	envelope, err := parseEventEnvelope(line, handle.Spec.PluginID)
	if err != nil {
		return nil, normalizeRuntimeError(err, "parse runtime frame envelope")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.proc != handle {
		return nil, nil
	}

	if ping := m.pendingPings[envelope.RequestID]; ping != nil {
		if envelope.Type != "pong" {
			return nil, errorf(codePluginProtocolViolation, "plugin returned unexpected frame type in response to ping", nil)
		}
		m.completePingLocked(envelope.RequestID, ping, nil)
		return nil, nil
	}

	if session := m.pendingEvents[envelope.RequestID]; session != nil {
		return nil, m.routeTerminalFrameLocked(session, envelope, line)
	}

	if m.eventExpiredLocked(envelope.RequestID) && (envelope.Type == "result" || envelope.Type == "error" || envelope.Type == "pong") {
		return nil, nil
	}

	if envelope.Type == "action" {
		return m.routeLocalActionFrameLocked(handle, line)
	}

	return nil, errorf(codePluginProtocolViolation, "plugin returned an unexpected protocol message during runtime delivery", nil)
}

func (m *Manager) routeTerminalFrameLocked(session *eventSession, envelope pluginwire.FrameEnvelope, line []byte) *plugins.Error {
	if session.pendingLocalAction > 0 {
		return errorf(codePluginProtocolViolation, "plugin returned a terminal frame before all local actions completed", nil)
	}

	delivery, done, err := decodeTerminalDelivery(session.requestID, line, envelope.Type)
	if !done {
		return errorf(codePluginProtocolViolation, "plugin returned an unexpected non-terminal frame for the active event", nil)
	}
	if err != nil {
		var runtimeErr *plugins.Error
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

func asRuntimeError(err error, target **plugins.Error) bool {
	if err == nil {
		return false
	}
	var runtimeErr *plugins.Error
	if !errors.As(err, &runtimeErr) {
		return false
	}
	*target = runtimeErr
	return true
}

func normalizeRuntimeError(err error, message string) *plugins.Error {
	if err == nil {
		return nil
	}
	var runtimeErr *plugins.Error
	if errors.As(err, &runtimeErr) {
		return runtimeErr
	}
	var actionErr *plugins.Error
	if errors.As(err, &actionErr) {
		return errorf(actionErr.Code, actionErr.Message, actionErr.Err)
	}
	return errorf(codePluginInternalError, message, err)
}
