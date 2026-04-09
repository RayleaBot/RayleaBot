package runtime

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/console"
)

type Manager struct {
	logger *slog.Logger
	deps   managerDeps
	opts   Options

	mu            sync.RWMutex
	protocolMu    sync.Mutex
	proc          *processHandle
	snap          Snapshot
	pendingEvents map[string]*eventSession
	pendingPings  map[string]*pingRequest
}

func New(logger *slog.Logger, options Options) *Manager {
	return newManager(logger, managerDeps{
		now: time.Now,
		requestID: func() string {
			return fmt.Sprintf("req_%d", time.Now().UnixNano())
		},
	}, options)
}

func newManager(logger *slog.Logger, deps managerDeps, options Options) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	if deps.now == nil {
		deps.now = time.Now
	}
	if deps.requestID == nil {
		deps.requestID = func() string {
			return fmt.Sprintf("req_%d", time.Now().UnixNano())
		}
	}
	if options.Console == nil {
		options.Console = console.NewStream(1000, 2*1024*1024)
	}
	if options.RedactText == nil {
		options.RedactText = func(text string) string {
			return text
		}
	}

	return &Manager{
		logger: logger,
		deps:   deps,
		opts:   options,
		pendingEvents: make(map[string]*eventSession),
		pendingPings:  make(map[string]*pingRequest),
		snap: Snapshot{
			State: StateStopped,
		},
	}
}

func (m *Manager) Snapshot() Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneSnapshot(m.snap)
}

func (m *Manager) Start(ctx context.Context, spec Spec, payload InitPayload) error {
	if payload.Bot.ID == "" {
		return errorf(codePlatformInvalidRequest, "init payload bot.id is required", nil)
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

	handle := newProcessHandle(cmd, stdin, bufio.NewReader(stdout), spec)
	go func() {
		handle.setExit(cmd.Wait())
	}()

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

	if err := handle.writeJSONLine(initFrame{
		ProtocolVersion: "1",
		Type:            "init",
		Timestamp:       m.deps.now().Unix(),
		PluginID:        spec.PluginID,
		RequestID:       requestID,
		Bot: botFrame{
			ID:       payload.Bot.ID,
			Nickname: payload.Bot.Nickname,
		},
		Capabilities: append([]string(nil), payload.Capabilities...),
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
