package manager

import (
	"bufio"
	"context"
	"os"
	"os/exec"

	runtimeprocess "github.com/RayleaBot/RayleaBot/server/internal/runtime/process"
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

	handle := runtimeprocess.NewHandle(cmd, stdin, bufio.NewReader(stdout), processSpec(spec))
	go handle.Watch()

	m.mu.Lock()
	m.proc = handle
	m.snap.PID = cmd.Process.Pid
	m.mu.Unlock()

	m.logger.Info(
		"plugin runtime starting",
		"component", "runtime",
		"plugin_id", spec.PluginID,
		"runtime_state", string(StateStarting),
		"entry_path", spec.EntryPath,
	)

	var bot *botFrame
	if payload.Bot.ID != "" {
		bot = &botFrame{
			ID:       payload.Bot.ID,
			Nickname: payload.Bot.Nickname,
		}
	}
	var permissions *permissionsFrame
	if len(payload.SuperAdmins) > 0 {
		permissions = &permissionsFrame{
			SuperAdmins: append([]string(nil), payload.SuperAdmins...),
		}
	}

	if err := handle.WriteJSONLine(initFrame{
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
		"plugin runtime started",
		"component", "runtime",
		"plugin_id", spec.PluginID,
		"runtime_state", string(StateRunning),
		"entry_path", spec.EntryPath,
	)

	go m.readRuntimeFrames(handle)
	go m.watchRunningProcess(handle)

	return nil
}

func processSpec(spec Spec) runtimeprocess.Spec {
	return runtimeprocess.Spec{
		PluginID:             spec.PluginID,
		InitTimeout:          spec.InitTimeout,
		InitMaxTotal:         spec.InitMaxTotal,
		EventTimeout:         spec.EventTimeout,
		ShutdownGrace:        spec.ShutdownGrace,
		EffectiveConcurrency: spec.EffectiveConcurrency,
	}
}
