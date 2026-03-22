package runtime

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"rayleabot/server/internal/console"
)

type State string

const (
	StateStopped    State = "stopped"
	StateStarting   State = "starting"
	StateRunning    State = "running"
	StateStopping   State = "stopping"
	StateCrashed    State = "crashed"
	StateBackoff    State = "backoff"
	StateDeadLetter State = "dead_letter"
)

type Snapshot struct {
	PluginID         string
	State            State
	LastErrorCode    string
	LastErrorMessage string
	InitRequestID    string
	PID              int
	StartedAt        *time.Time
	StoppedAt        *time.Time
	CrashCount       int
	NextRetryAt      *time.Time
}

// CrashCallback is invoked by the runtime manager when a running plugin
// process exits unexpectedly. The lifecycle controller uses this to drive
// the backoff/restart cycle.
type CrashCallback func(pluginID string, crashCount int, lastErrorCode string)

type Event struct {
	EventID        string
	SourceProtocol string
	SourceAdapter  string
	EventType      string
	Timestamp      int64
	Actor          *EventActor
	Target         *EventTarget
	Message        *EventMessage
	PayloadFields  map[string]any
	MessageID      string
}

type EventActor struct {
	ID       string
	Nickname string
	Role     string
}

type EventTarget struct {
	Type string
	ID   string
	Name string
}

type EventMessage struct {
	PlainText string
	Segments  []EventSegment
}

type EventSegment struct {
	Type string
	Data map[string]any
}

type Action struct {
	Kind              string
	TargetType        string
	TargetID          string
	Text              string
	ReplyToMessageID  string
	File              string
}

type Delivery struct {
	RequestID    string
	Action       *Action
	Result       map[string]any
	ErrorCode    string
	ErrorMessage string
}

type managerDeps struct {
	now       func() time.Time
	requestID func() string
}

type Options struct {
	Console                    *console.Stream
	RedactText                 func(string) string
	StderrRateLimitBytesPerSec int
	OnCrash                    CrashCallback
}

type Manager struct {
	logger *slog.Logger
	deps   managerDeps
	opts   Options

	mu        sync.RWMutex
	deliverMu sync.Mutex
	proc      *processHandle
	snap      Snapshot
}

type processHandle struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	spec   Spec

	done    chan struct{}
	exitMu  sync.RWMutex
	exitErr error
}

type pingFrame struct {
	ProtocolVersion string `json:"protocol_version"`
	Type            string `json:"type"`
	Timestamp       int64  `json:"timestamp"`
	PluginID        string `json:"plugin_id"`
	RequestID       string `json:"request_id"`
}

type initFrame struct {
	ProtocolVersion string   `json:"protocol_version"`
	Type            string   `json:"type"`
	Timestamp       int64    `json:"timestamp"`
	PluginID        string   `json:"plugin_id"`
	RequestID       string   `json:"request_id"`
	Bot             botFrame `json:"bot"`
	Capabilities    []string `json:"capabilities,omitempty"`
}

type botFrame struct {
	ID       string `json:"id"`
	Nickname string `json:"nickname,omitempty"`
}

type shutdownFrame struct {
	ProtocolVersion string `json:"protocol_version"`
	Type            string `json:"type"`
	Timestamp       int64  `json:"timestamp"`
	PluginID        string `json:"plugin_id"`
	RequestID       string `json:"request_id"`
	Reason          string `json:"reason"`
}

type eventFrame struct {
	ProtocolVersion string             `json:"protocol_version"`
	Type            string             `json:"type"`
	Timestamp       int64              `json:"timestamp"`
	PluginID        string             `json:"plugin_id"`
	RequestID       string             `json:"request_id"`
	Event           protocolEventFrame `json:"event"`
}

type protocolEventFrame struct {
	EventID        string                `json:"event_id"`
	SourceProtocol string                `json:"source_protocol"`
	SourceAdapter  string                `json:"source_adapter"`
	EventType      string                `json:"event_type"`
	Timestamp      int64                 `json:"timestamp"`
	Actor          *protocolActorFrame   `json:"actor,omitempty"`
	Target         *protocolTargetFrame  `json:"target,omitempty"`
	Message        *protocolMessageFrame `json:"message,omitempty"`
	Payload        *protocolPayloadFrame `json:"payload,omitempty"`
}

type protocolActorFrame struct {
	ID       string `json:"id"`
	Nickname string `json:"nickname,omitempty"`
	Role     string `json:"role,omitempty"`
}

type protocolTargetFrame struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

type protocolMessageFrame struct {
	PlainText string                 `json:"plain_text,omitempty"`
	Segments  []protocolSegmentFrame `json:"segments,omitempty"`
}

type protocolSegmentFrame struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data,omitempty"`
}

type protocolPayloadFrame struct {
	MessageID  string   `json:"message_id,omitempty"`
	Command    string   `json:"command,omitempty"`
	Args       []string `json:"args,omitempty"`
	SubType    string   `json:"sub_type,omitempty"`
	OperatorID string   `json:"operator_id,omitempty"`
}

type actionFrame struct {
	ProtocolVersion string                  `json:"protocol_version"`
	Type            string                  `json:"type"`
	Timestamp       int64                   `json:"timestamp"`
	PluginID        string                  `json:"plugin_id"`
	RequestID       string                  `json:"request_id"`
	Action          string                  `json:"action"`
	Data            protocolActionDataFrame `json:"data"`
}

type protocolActionDataFrame struct {
	TargetType       string `json:"target_type,omitempty"`
	TargetID         string `json:"target_id,omitempty"`
	Text             string `json:"text,omitempty"`
	ReplyToMessageID string `json:"reply_to_message_id,omitempty"`
	File             string `json:"file,omitempty"`
}

type frameEnvelope struct {
	ProtocolVersion string `json:"protocol_version"`
	Type            string `json:"type"`
	Timestamp       int64  `json:"timestamp"`
	PluginID        string `json:"plugin_id"`
	RequestID       string `json:"request_id"`
}

type initProgressFrame struct {
	ProtocolVersion string `json:"protocol_version"`
	Type            string `json:"type"`
	Timestamp       int64  `json:"timestamp"`
	PluginID        string `json:"plugin_id"`
	RequestID       string `json:"request_id"`
	Summary         string `json:"summary"`
}

type initAckFrame struct {
	Type          string   `json:"type"`
	RequestID     string   `json:"request_id"`
	Status        string   `json:"status"`
	Subscriptions []string `json:"subscriptions,omitempty"`
	ErrorMessage  string   `json:"error_message,omitempty"`
}

type errorFrame struct {
	Type      string `json:"type"`
	RequestID string `json:"request_id"`
	Code      string `json:"code"`
	Message   string `json:"message"`
}

type resultFrame struct {
	Type      string         `json:"type"`
	RequestID string         `json:"request_id"`
	Status    string         `json:"status"`
	Data      map[string]any `json:"data"`
}

type initResponseStatus int

const (
	initResponseWait initResponseStatus = iota
	initResponseReady
)

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
		snap: Snapshot{
			State: StateStopped,
		},
	}
}

func newProcessHandle(cmd *exec.Cmd, stdin io.WriteCloser, stdout *bufio.Reader, spec Spec) *processHandle {
	return &processHandle{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		spec:   spec,
		done:   make(chan struct{}),
	}
}

func (h *processHandle) setExit(err error) {
	h.exitMu.Lock()
	defer h.exitMu.Unlock()

	h.exitErr = err
	close(h.done)
}

func (h *processHandle) exitResult() (error, bool) {
	select {
	case <-h.done:
		h.exitMu.RLock()
		defer h.exitMu.RUnlock()
		return h.exitErr, true
	default:
		return nil, false
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

	if err := writeJSONLine(stdin, initFrame{
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

	if err := m.awaitInitAck(ctx, handle, requestID); err != nil {
		m.cleanupFailedStart(handle, err.Code, err.Message, err.Err)
		return err
	}

	m.mu.Lock()
	if m.proc == handle {
		m.snap.State = StateRunning
		m.snap.LastErrorCode = ""
		m.snap.LastErrorMessage = ""
	}
	m.mu.Unlock()

	m.logger.Info(
		"plugin runtime started",
		"component", "runtime",
		"plugin_id", spec.PluginID,
		"runtime_state", string(StateRunning),
		"entry_path", spec.EntryPath,
	)

	go m.watchRunningProcess(handle)

	return nil
}

func (m *Manager) Stop(ctx context.Context) error {
	m.deliverMu.Lock()
	defer m.deliverMu.Unlock()

	m.mu.Lock()
	handle := m.proc
	if handle == nil {
		stoppedAt := m.deps.now()
		m.snap.State = StateStopped
		m.snap.StoppedAt = &stoppedAt
		m.mu.Unlock()
		return nil
	}
	if waitErr, exited := handle.exitResult(); exited {
		m.mu.Unlock()
		m.reconcileExitedProcess(handle, waitErr)
		return nil
	}
	m.snap.State = StateStopping
	m.mu.Unlock()

	m.logger.Info(
		"plugin runtime stopping",
		"component", "runtime",
		"plugin_id", handle.spec.PluginID,
		"runtime_state", string(StateStopping),
	)

	writeErr := writeJSONLine(handle.stdin, shutdownFrame{
		ProtocolVersion: "1",
		Type:            "shutdown",
		Timestamp:       m.deps.now().Unix(),
		PluginID:        handle.spec.PluginID,
		RequestID:       m.deps.requestID(),
		Reason:          "stop",
	})
	_ = handle.stdin.Close()

	if waitErr, exited := handle.exitResult(); exited {
		m.reconcileExitedProcess(handle, waitErr)
		return nil
	}

	stopCtx, cancel := context.WithTimeout(ctx, handle.spec.ShutdownGrace)
	defer cancel()

	select {
	case <-handle.done:
		waitErr, _ := handle.exitResult()
		if waitErr != nil {
			m.markStopped(codePluginInternalError, "plugin exited with error during shutdown", waitErr)
			return errorf(codePluginInternalError, "plugin exited with error during shutdown", waitErr)
		}
		if writeErr != nil {
			m.markStopped(codePluginInternalError, "write shutdown frame", writeErr)
			return errorf(codePluginInternalError, "write shutdown frame", writeErr)
		}
		m.markStopped("", "", nil)
		m.logger.Info(
			"plugin runtime stopped",
			"component", "runtime",
			"plugin_id", handle.spec.PluginID,
			"runtime_state", string(StateStopped),
		)
		return nil
	case <-stopCtx.Done():
		if handle.cmd.Process != nil {
			_ = handle.cmd.Process.Kill()
		}
		select {
		case <-handle.done:
		case <-time.After(500 * time.Millisecond):
		}
		m.markStopped(codePluginShutdownTimeout, "plugin shutdown timed out", stopCtx.Err())
		return errorf(codePluginShutdownTimeout, "plugin shutdown timed out", stopCtx.Err())
	}
}

// Ping sends a ping frame to the plugin and waits for a pong response.
// Returns an error if the runtime is not running, the plugin does not respond
// within the event timeout, or the plugin returns a protocol violation.
// A timeout causes the runtime to be stopped.
func (m *Manager) Ping(ctx context.Context) error {
	m.deliverMu.Lock()
	defer m.deliverMu.Unlock()

	m.mu.RLock()
	handle := m.proc
	snapshot := m.snap
	m.mu.RUnlock()

	if handle == nil || snapshot.State == StateStopped {
		return errorf(codePlatformInvalidRequest, "plugin runtime is not running", nil)
	}
	if snapshot.State == StateStopping {
		return errorf(codePluginStopping, "plugin runtime is stopping", nil)
	}
	if snapshot.State != StateRunning {
		return errorf(codePlatformInvalidRequest, "plugin runtime is not ready for ping", nil)
	}

	requestID := m.deps.requestID()
	if err := writeJSONLine(handle.stdin, pingFrame{
		ProtocolVersion: "1",
		Type:            "ping",
		Timestamp:       m.deps.now().Unix(),
		PluginID:        handle.spec.PluginID,
		RequestID:       requestID,
	}); err != nil {
		return errorf(codePluginInternalError, "write ping frame", err)
	}

	return m.awaitPong(ctx, handle, requestID)
}

func (m *Manager) awaitPong(ctx context.Context, handle *processHandle, requestID string) error {
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

	timeout := handle.spec.EventTimeout
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining > 0 && remaining < timeout {
			timeout = remaining
		}
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case line := <-readCh:
		if err := m.parsePongResponse(line, handle.spec.PluginID, requestID); err != nil {
			var runtimeErr *Error
			if errors.As(err, &runtimeErr) {
				m.cleanupFailedDelivery(handle, runtimeErr.Code, runtimeErr.Message, runtimeErr.Err)
			}
			return err
		}
		return nil
	case readErr := <-readErrCh:
		if errors.Is(readErr, io.EOF) {
			waitErr, _ := handle.exitResult()
			if waitErr == nil {
				return errorf(codePluginInternalError, "plugin exited during ping", nil)
			}
			return errorf(codePluginInternalError, "plugin exited during ping", waitErr)
		}
		return errorf(codePluginProtocolViolation, "read plugin pong response", readErr)
	case <-handle.done:
		waitErr, _ := handle.exitResult()
		if waitErr == nil {
			return errorf(codePluginInternalError, "plugin exited during ping", nil)
		}
		return errorf(codePluginInternalError, "plugin exited during ping", waitErr)
	case <-timer.C:
		runtimeErr := errorf(codePluginEventTimeout, "plugin pong response timed out", nil)
		m.cleanupFailedDelivery(handle, runtimeErr.Code, runtimeErr.Message, runtimeErr.Err)
		return runtimeErr
	case <-ctx.Done():
		runtimeErr := errorf(codePluginEventTimeout, "plugin pong response timed out", ctx.Err())
		m.cleanupFailedDelivery(handle, runtimeErr.Code, runtimeErr.Message, runtimeErr.Err)
		return runtimeErr
	}
}

func (m *Manager) parsePongResponse(line []byte, pluginID string, requestID string) error {
	var envelope frameEnvelope
	if err := json.Unmarshal(line, &envelope); err != nil {
		return errorf(codePluginProtocolViolation, "plugin returned malformed protocol json", err)
	}
	if envelope.ProtocolVersion != "1" {
		return errorf(codePluginProtocolViolation, "plugin returned an unsupported protocol_version", nil)
	}
	if envelope.PluginID == "" || envelope.PluginID != pluginID {
		return errorf(codePluginProtocolViolation, "plugin returned a mismatched plugin_id", nil)
	}
	if envelope.RequestID == "" || envelope.RequestID != requestID {
		return errorf(codePluginProtocolViolation, "plugin returned a mismatched request_id", nil)
	}
	if envelope.Type != "pong" {
		return errorf(codePluginProtocolViolation, "plugin returned unexpected frame type in response to ping", nil)
	}
	return nil
}

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
	frame := eventFrame{
		ProtocolVersion: "1",
		Type:            "event",
		Timestamp:       m.deps.now().Unix(),
		PluginID:        handle.spec.PluginID,
		RequestID:       requestID,
		Event: protocolEventFrame{
			EventID:        event.EventID,
			SourceProtocol: event.SourceProtocol,
			SourceAdapter:  event.SourceAdapter,
			EventType:      event.EventType,
			Timestamp:      event.Timestamp,
		},
	}

	if event.Actor != nil && event.Actor.ID != "" {
		frame.Event.Actor = &protocolActorFrame{
			ID:       event.Actor.ID,
			Nickname: event.Actor.Nickname,
			Role:     event.Actor.Role,
		}
	}
	if event.Target != nil && event.Target.Type != "" && event.Target.ID != "" {
		frame.Event.Target = &protocolTargetFrame{
			Type: event.Target.Type,
			ID:   event.Target.ID,
			Name: event.Target.Name,
		}
	}
	if event.Message != nil && event.Message.PlainText != "" {
		msgFrame := &protocolMessageFrame{PlainText: event.Message.PlainText}
		for _, seg := range event.Message.Segments {
			msgFrame.Segments = append(msgFrame.Segments, protocolSegmentFrame{
				Type: seg.Type,
				Data: seg.Data,
			})
		}
		frame.Event.Message = msgFrame
	}

	// Build payload frame from event fields.
	var payload protocolPayloadFrame
	hasPayload := false
	if event.MessageID != "" {
		payload.MessageID = event.MessageID
		hasPayload = true
	}
	if event.PayloadFields != nil {
		if v, ok := event.PayloadFields["sub_type"].(string); ok && v != "" {
			payload.SubType = v
			hasPayload = true
		}
		if v, ok := event.PayloadFields["operator_id"].(string); ok && v != "" {
			payload.OperatorID = v
			hasPayload = true
		}
		if v, ok := event.PayloadFields["command"].(string); ok && v != "" {
			payload.Command = v
			hasPayload = true
		}
		if v, ok := event.PayloadFields["args"].([]string); ok && len(v) > 0 {
			payload.Args = v
			hasPayload = true
		}
	}
	if hasPayload {
		frame.Event.Payload = &payload
	}

	if err := writeJSONLine(handle.stdin, frame); err != nil {
		return Delivery{}, errorf(codePluginInternalError, "write event frame", err)
	}

	return m.awaitEventResponse(ctx, handle, requestID)
}

func (m *Manager) awaitInitAck(ctx context.Context, handle *processHandle, requestID string) *Error {
	silenceTimer := time.NewTimer(handle.spec.InitTimeout)
	defer silenceTimer.Stop()

	totalTimer := time.NewTimer(handle.spec.InitMaxTotal)
	defer totalTimer.Stop()

	for {
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

		select {
		case line := <-readCh:
			status, summary, err := m.parseInitResponse(line, handle.spec.PluginID, requestID)
			if err != nil {
				return err
			}
			if status == initResponseReady {
				return nil
			}

			m.logger.Info(
				"plugin runtime init progress",
				"component", "runtime",
				"plugin_id", handle.spec.PluginID,
				"runtime_state", string(StateStarting),
				"summary", summary,
			)
			resetTimer(silenceTimer, handle.spec.InitTimeout)
		case readErr := <-readErrCh:
			if errors.Is(readErr, io.EOF) {
				waitErr, _ := handle.exitResult()
				if waitErr == nil {
					return errorf(codePluginInternalError, "plugin exited before init_ack", nil)
				}
				return errorf(codePluginInternalError, "plugin exited before init_ack", waitErr)
			}
			return errorf(codePluginProtocolViolation, "read plugin init response", readErr)
		case <-handle.done:
			waitErr, _ := handle.exitResult()
			if waitErr == nil {
				return errorf(codePluginInternalError, "plugin exited before init_ack", nil)
			}
			return errorf(codePluginInternalError, "plugin exited before init_ack", waitErr)
		case <-silenceTimer.C:
			return errorf(codePluginInitTimeout, "plugin init_ack timed out", nil)
		case <-totalTimer.C:
			return errorf(codePluginInitTimeout, "plugin init exceeded maximum total duration", nil)
		case <-ctx.Done():
			return errorf(codePluginInitTimeout, "plugin init_ack timed out", ctx.Err())
		}
	}
}

func (m *Manager) parseInitResponse(line []byte, pluginID string, requestID string) (initResponseStatus, string, *Error) {
	var envelope frameEnvelope
	if err := json.Unmarshal(line, &envelope); err != nil {
		return initResponseWait, "", errorf(codePluginProtocolViolation, "plugin returned malformed protocol json", err)
	}

	if envelope.ProtocolVersion != "1" {
		return initResponseWait, "", errorf(codePluginProtocolViolation, "plugin returned an unsupported protocol_version", nil)
	}
	if envelope.PluginID == "" || envelope.PluginID != pluginID {
		return initResponseWait, "", errorf(codePluginProtocolViolation, "plugin returned a mismatched plugin_id", nil)
	}

	if envelope.RequestID == "" || envelope.RequestID != requestID {
		return initResponseWait, "", errorf(codePluginProtocolViolation, "plugin returned a mismatched request_id", nil)
	}

	switch envelope.Type {
	case "init_progress":
		var progress initProgressFrame
		if err := json.Unmarshal(line, &progress); err != nil {
			return initResponseWait, "", errorf(codePluginProtocolViolation, "plugin returned malformed init_progress", err)
		}

		summary := strings.TrimSpace(progress.Summary)
		if summary == "" {
			return initResponseWait, "", errorf(codePluginProtocolViolation, "plugin init_progress is missing summary", nil)
		}
		return initResponseWait, summary, nil
	case "init_ack":
		var ack initAckFrame
		if err := json.Unmarshal(line, &ack); err != nil {
			return initResponseWait, "", errorf(codePluginProtocolViolation, "plugin returned malformed init_ack", err)
		}
		if ack.Status == "ready" {
			return initResponseReady, "", nil
		}
		if ack.Status == "error" {
			message := strings.TrimSpace(ack.ErrorMessage)
			if message == "" {
				message = "plugin reported init error"
			}
			return initResponseWait, "", errorf(codePluginInternalError, message, nil)
		}
		return initResponseWait, "", errorf(codePluginProtocolViolation, "plugin returned unsupported init_ack status", nil)
	case "error":
		var frame errorFrame
		if err := json.Unmarshal(line, &frame); err != nil {
			return initResponseWait, "", errorf(codePluginProtocolViolation, "plugin returned malformed error frame", err)
		}
		if frame.Code == "" || frame.Message == "" {
			return initResponseWait, "", errorf(codePluginProtocolViolation, "plugin error frame is missing code or message", nil)
		}
		return initResponseWait, "", errorf(frame.Code, frame.Message, nil)
	default:
		return initResponseWait, "", errorf(codePluginProtocolViolation, "plugin returned an unexpected protocol message during init", nil)
	}
}

func (m *Manager) awaitEventResponse(ctx context.Context, handle *processHandle, requestID string) (Delivery, error) {
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

	timeout := handle.spec.EventTimeout
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining > 0 && remaining < timeout {
			timeout = remaining
		}
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case line := <-readCh:
		return m.parseEventResponse(line, handle.spec.PluginID, requestID)
	case readErr := <-readErrCh:
		if errors.Is(readErr, io.EOF) {
			waitErr, _ := handle.exitResult()
			if waitErr == nil {
				return Delivery{}, errorf(codePluginInternalError, "plugin exited during event delivery", nil)
			}
			return Delivery{}, errorf(codePluginInternalError, "plugin exited during event delivery", waitErr)
		}
		return Delivery{}, errorf(codePluginProtocolViolation, "read plugin event response", readErr)
	case <-handle.done:
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
		runtimeErr := errorf(codePluginEventTimeout, "plugin event response timed out", ctx.Err())
		m.cleanupFailedDelivery(handle, runtimeErr.Code, runtimeErr.Message, runtimeErr.Err)
		return Delivery{}, runtimeErr
	}
}

func (m *Manager) parseEventResponse(line []byte, pluginID string, requestID string) (Delivery, error) {
	var envelope frameEnvelope
	if err := json.Unmarshal(line, &envelope); err != nil {
		return Delivery{}, errorf(codePluginProtocolViolation, "plugin returned malformed protocol json", err)
	}
	if envelope.ProtocolVersion != "1" {
		return Delivery{}, errorf(codePluginProtocolViolation, "plugin returned an unsupported protocol_version", nil)
	}
	if envelope.PluginID == "" || envelope.PluginID != pluginID {
		return Delivery{}, errorf(codePluginProtocolViolation, "plugin returned a mismatched plugin_id", nil)
	}
	if envelope.RequestID == "" || envelope.RequestID != requestID {
		return Delivery{}, errorf(codePluginProtocolViolation, "plugin returned a mismatched request_id", nil)
	}

	switch envelope.Type {
	case "action":
		var frame actionFrame
		if err := json.Unmarshal(line, &frame); err != nil {
			return Delivery{}, errorf(codePluginProtocolViolation, "plugin returned malformed action frame", err)
		}

		switch frame.Action {
		case "message.send":
			targetType := strings.TrimSpace(frame.Data.TargetType)
			targetID := strings.TrimSpace(frame.Data.TargetID)
			text := strings.TrimSpace(frame.Data.Text)
			if targetID == "" || text == "" {
				return Delivery{}, errorf(codePluginProtocolViolation, "plugin action frame is missing required message.send fields", nil)
			}
			switch targetType {
			case "group", "private":
			default:
				return Delivery{}, errorf(codePluginProtocolViolation, "plugin action frame uses unsupported target_type", nil)
			}
			return Delivery{
				RequestID: requestID,
				Action: &Action{
					Kind:       frame.Action,
					TargetType: targetType,
					TargetID:   targetID,
					Text:       text,
				},
			}, nil

		case "message.reply":
			replyToMessageID := strings.TrimSpace(frame.Data.ReplyToMessageID)
			text := strings.TrimSpace(frame.Data.Text)
			if replyToMessageID == "" || text == "" {
				return Delivery{}, errorf(codePluginProtocolViolation, "plugin action frame is missing required message.reply fields", nil)
			}
			return Delivery{
				RequestID: requestID,
				Action: &Action{
					Kind:             frame.Action,
					ReplyToMessageID: replyToMessageID,
					Text:             text,
				},
			}, nil

		case "message.send_image":
			targetType := strings.TrimSpace(frame.Data.TargetType)
			targetID := strings.TrimSpace(frame.Data.TargetID)
			file := strings.TrimSpace(frame.Data.File)
			if targetID == "" || file == "" {
				return Delivery{}, errorf(codePluginProtocolViolation, "plugin action frame is missing required message.send_image fields", nil)
			}
			switch targetType {
			case "group", "private":
			default:
				return Delivery{}, errorf(codePluginProtocolViolation, "plugin action frame uses unsupported target_type", nil)
			}
			return Delivery{
				RequestID: requestID,
				Action: &Action{
					Kind:       frame.Action,
					TargetType: targetType,
					TargetID:   targetID,
					File:       file,
				},
			}, nil

		default:
			return Delivery{}, errorf(codePluginProtocolViolation, "plugin returned unsupported action kind", nil)
		}
	case "result":
		var frame resultFrame
		if err := json.Unmarshal(line, &frame); err != nil {
			return Delivery{}, errorf(codePluginProtocolViolation, "plugin returned malformed result frame", err)
		}
		if frame.Status != "success" {
			return Delivery{}, errorf(codePluginProtocolViolation, "plugin result frame must use status=success", nil)
		}
		if frame.Data == nil {
			frame.Data = map[string]any{}
		}
		return Delivery{
			RequestID: requestID,
			Result:    frame.Data,
		}, nil
	case "error":
		var frame errorFrame
		if err := json.Unmarshal(line, &frame); err != nil {
			return Delivery{}, errorf(codePluginProtocolViolation, "plugin returned malformed error frame", err)
		}
		if frame.Code == "" || frame.Message == "" {
			return Delivery{}, errorf(codePluginProtocolViolation, "plugin error frame is missing code or message", nil)
		}
		delivery := Delivery{
			RequestID:    requestID,
			ErrorCode:    frame.Code,
			ErrorMessage: frame.Message,
		}
		return delivery, errorf(frame.Code, frame.Message, nil)
	default:
		return Delivery{}, errorf(codePluginProtocolViolation, "plugin returned an unexpected protocol message during event delivery", nil)
	}
}

func (m *Manager) cleanupFailedStart(handle *processHandle, code, message string, err error) {
	if handle != nil && handle.cmd != nil && handle.cmd.Process != nil {
		_ = handle.cmd.Process.Kill()
	}
	if handle != nil {
		select {
		case <-handle.done:
		case <-time.After(500 * time.Millisecond):
		}
	}
	m.markStopped(code, message, err)
}

func (m *Manager) cleanupFailedDelivery(handle *processHandle, code, message string, err error) {
	if handle != nil && handle.cmd != nil && handle.cmd.Process != nil {
		_ = handle.cmd.Process.Kill()
	}
	if handle != nil {
		select {
		case <-handle.done:
		case <-time.After(500 * time.Millisecond):
		}
	}
	m.markStopped(code, message, err)
}

func (m *Manager) markStopped(code, message string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	stoppedAt := m.deps.now()
	m.proc = nil
	m.snap.State = StateStopped
	m.snap.StoppedAt = &stoppedAt
	if code == "" {
		m.snap.LastErrorCode = ""
		m.snap.LastErrorMessage = ""
		return
	}

	m.snap.LastErrorCode = code
	if err != nil {
		m.snap.LastErrorMessage = fmt.Sprintf("%s: %v", message, err)
		return
	}
	m.snap.LastErrorMessage = message
}

func cloneSnapshot(snapshot Snapshot) Snapshot {
	cloned := snapshot
	if snapshot.StartedAt != nil {
		startedAt := *snapshot.StartedAt
		cloned.StartedAt = &startedAt
	}
	if snapshot.StoppedAt != nil {
		stoppedAt := *snapshot.StoppedAt
		cloned.StoppedAt = &stoppedAt
	}
	if snapshot.NextRetryAt != nil {
		nextRetryAt := *snapshot.NextRetryAt
		cloned.NextRetryAt = &nextRetryAt
	}
	return cloned
}

// ResetCrashCount resets the crash counter after a successful start.
func (m *Manager) ResetCrashCount() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.snap.CrashCount = 0
	m.snap.NextRetryAt = nil
}

// SetBackoffState transitions the runtime snapshot to backoff with a
// scheduled next retry time. The lifecycle controller calls this after
// a crash to indicate the backoff wait period.
func (m *Manager) SetBackoffState(nextRetry time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.snap.State = StateBackoff
	m.snap.NextRetryAt = &nextRetry
}

// SetDeadLetterState transitions the runtime snapshot to dead_letter,
// indicating that the maximum crash-backoff attempts have been exhausted.
func (m *Manager) SetDeadLetterState() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.snap.State = StateDeadLetter
	m.snap.NextRetryAt = nil
}

// SetOnCrash registers the crash callback after construction. This is
// used when the callback depends on objects that reference the manager
// itself (e.g. the lifecycle controller).
func (m *Manager) SetOnCrash(cb CrashCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.opts.OnCrash = cb
}

// SetStopped transitions the runtime snapshot to stopped without
// attempting to stop a process. Used when the runtime is in a
// non-running state (crashed, backoff, dead_letter) and needs to
// be reset.
func (m *Manager) SetStopped() {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.deps.now()
	m.snap.State = StateStopped
	m.snap.StoppedAt = &now
	m.snap.NextRetryAt = nil
}

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

func resetTimer(timer *time.Timer, duration time.Duration) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(duration)
}

func drainOutput(reader io.ReadCloser) {
	if reader == nil {
		return
	}
	defer reader.Close()

	_, _ = io.Copy(io.Discard, reader)
}

func writeJSONLine(writer io.Writer, value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}

	if _, err := writer.Write(append(encoded, '\n')); err != nil {
		return err
	}

	return nil
}
