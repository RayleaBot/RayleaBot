package runtime

import (
	"bufio"
	"context"
	"encoding/base64"
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
	Subscriptions    []string
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

type ActionSegment struct {
	Type string
	Data map[string]any
}

type Action struct {
	Kind                    string
	TargetType              string
	TargetID                string
	ReplyToEventID          string
	FallbackToSendIfMissing bool
	MessageSegments         []ActionSegment
	LogLevel                string
	LogMessage              string
	LogFields               map[string]any
	StorageOperation        string
	StorageRoot             string
	StoragePath             string
	StorageKey              string
	StoragePrefix           string
	StorageValue            any
	StorageContent          []byte
	HTTPMethod              string
	HTTPURL                 string
	HTTPHeaders             map[string]string
	HTTPTimeoutSeconds      int
	HTTPBody                []byte
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

type LocalActionExecutor func(context.Context, string, string, Action) (map[string]any, error)

type Options struct {
	Console                    *console.Stream
	RedactText                 func(string) string
	StderrRateLimitBytesPerSec int
	OnCrash                    CrashCallback
	ExecuteLocalAction         LocalActionExecutor
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
	ProtocolVersion string          `json:"protocol_version"`
	Type            string          `json:"type"`
	Timestamp       int64           `json:"timestamp"`
	PluginID        string          `json:"plugin_id"`
	RequestID       string          `json:"request_id"`
	Action          string          `json:"action"`
	Data            json.RawMessage `json:"data"`
}

type protocolOutboundMessageFrame struct {
	Segments []protocolSegmentFrame `json:"segments"`
}

type protocolActionMessageSendFrame struct {
	TargetType string                        `json:"target_type"`
	TargetID   string                        `json:"target_id"`
	Message    *protocolOutboundMessageFrame `json:"message"`
}

type protocolActionMessageReplyFrame struct {
	ReplyToEventID          *string                       `json:"reply_to_event_id"`
	Message                 *protocolOutboundMessageFrame `json:"message"`
	FallbackToSendIfMissing bool                          `json:"fallback_to_send_if_missing,omitempty"`
}

type protocolActionLoggerWriteFrame struct {
	Level   string         `json:"level"`
	Message string         `json:"message"`
	Fields  map[string]any `json:"fields,omitempty"`
}

type protocolActionStorageKVFrame struct {
	Operation string           `json:"operation"`
	Key       *string          `json:"key,omitempty"`
	Prefix    *string          `json:"prefix,omitempty"`
	Value     *json.RawMessage `json:"value,omitempty"`
}

type protocolActionStorageFileFrame struct {
	Operation     string  `json:"operation"`
	Root          string  `json:"root"`
	Path          *string `json:"path,omitempty"`
	Prefix        *string `json:"prefix,omitempty"`
	ContentText   *string `json:"content_text,omitempty"`
	ContentBase64 *string `json:"content_base64,omitempty"`
}

type protocolActionHTTPRequestFrame struct {
	Method         string            `json:"method"`
	URL            string            `json:"url"`
	Headers        map[string]string `json:"headers,omitempty"`
	TimeoutSeconds *int              `json:"timeout_seconds,omitempty"`
	BodyText       *string           `json:"body_text,omitempty"`
	BodyBase64     *string           `json:"body_base64,omitempty"`
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

func (m *Manager) awaitInitAck(ctx context.Context, handle *processHandle, requestID string) ([]string, *Error) {
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
			status, payload, err := m.parseInitResponse(line, handle.spec.PluginID, requestID)
			if err != nil {
				return nil, err
			}
			if status == initResponseReady {
				return payload, nil
			}
			summary := ""
			if len(payload) > 0 {
				summary = payload[0]
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
					return nil, errorf(codePluginInternalError, "plugin exited before init_ack", nil)
				}
				return nil, errorf(codePluginInternalError, "plugin exited before init_ack", waitErr)
			}
			return nil, errorf(codePluginProtocolViolation, "read plugin init response", readErr)
		case <-handle.done:
			waitErr, _ := handle.exitResult()
			if waitErr == nil {
				return nil, errorf(codePluginInternalError, "plugin exited before init_ack", nil)
			}
			return nil, errorf(codePluginInternalError, "plugin exited before init_ack", waitErr)
		case <-silenceTimer.C:
			return nil, errorf(codePluginInitTimeout, "plugin init_ack timed out", nil)
		case <-totalTimer.C:
			return nil, errorf(codePluginInitTimeout, "plugin init exceeded maximum total duration", nil)
		case <-ctx.Done():
			return nil, errorf(codePluginInitTimeout, "plugin init_ack timed out", ctx.Err())
		}
	}
}

func (m *Manager) parseInitResponse(line []byte, pluginID string, requestID string) (initResponseStatus, []string, *Error) {
	var envelope frameEnvelope
	if err := json.Unmarshal(line, &envelope); err != nil {
		return initResponseWait, nil, errorf(codePluginProtocolViolation, "plugin returned malformed protocol json", err)
	}

	if envelope.ProtocolVersion != "1" {
		return initResponseWait, nil, errorf(codePluginProtocolViolation, "plugin returned an unsupported protocol_version", nil)
	}
	if envelope.PluginID == "" || envelope.PluginID != pluginID {
		return initResponseWait, nil, errorf(codePluginProtocolViolation, "plugin returned a mismatched plugin_id", nil)
	}

	if envelope.RequestID == "" || envelope.RequestID != requestID {
		return initResponseWait, nil, errorf(codePluginProtocolViolation, "plugin returned a mismatched request_id", nil)
	}

	switch envelope.Type {
	case "init_progress":
		var progress initProgressFrame
		if err := json.Unmarshal(line, &progress); err != nil {
			return initResponseWait, nil, errorf(codePluginProtocolViolation, "plugin returned malformed init_progress", err)
		}

		summary := strings.TrimSpace(progress.Summary)
		if summary == "" {
			return initResponseWait, nil, errorf(codePluginProtocolViolation, "plugin init_progress is missing summary", nil)
		}
		return initResponseWait, []string{summary}, nil
	case "init_ack":
		var ack initAckFrame
		if err := json.Unmarshal(line, &ack); err != nil {
			return initResponseWait, nil, errorf(codePluginProtocolViolation, "plugin returned malformed init_ack", err)
		}
		if ack.Status == "ready" {
			return initResponseReady, append([]string(nil), ack.Subscriptions...), nil
		}
		if ack.Status == "error" {
			message := strings.TrimSpace(ack.ErrorMessage)
			if message == "" {
				message = "plugin reported init error"
			}
			return initResponseWait, nil, errorf(codePluginInternalError, message, nil)
		}
		return initResponseWait, nil, errorf(codePluginProtocolViolation, "plugin returned unsupported init_ack status", nil)
	case "error":
		var frame errorFrame
		if err := json.Unmarshal(line, &frame); err != nil {
			return initResponseWait, nil, errorf(codePluginProtocolViolation, "plugin returned malformed error frame", err)
		}
		if frame.Code == "" || frame.Message == "" {
			return initResponseWait, nil, errorf(codePluginProtocolViolation, "plugin error frame is missing code or message", nil)
		}
		return initResponseWait, nil, errorf(frame.Code, frame.Message, nil)
	default:
		return initResponseWait, nil, errorf(codePluginProtocolViolation, "plugin returned an unexpected protocol message during init", nil)
	}
}

func (m *Manager) awaitEventResponse(ctx context.Context, handle *processHandle, requestID string) (Delivery, error) {
	timeout := handle.spec.EventTimeout
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining > 0 && remaining < timeout {
			timeout = remaining
		}
	}
	deadline := time.Now().Add(timeout)
	seenLocalRequestIDs := make(map[string]struct{})

	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			runtimeErr := errorf(codePluginEventTimeout, "plugin event response timed out", nil)
			m.cleanupFailedDelivery(handle, runtimeErr.Code, runtimeErr.Message, runtimeErr.Err)
			return Delivery{}, runtimeErr
		}

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

		timer := time.NewTimer(remaining)
		select {
		case line := <-readCh:
			timer.Stop()
			delivery, done, err := m.processEventFrame(ctx, handle, requestID, seenLocalRequestIDs, line)
			if err != nil {
				if done {
					return delivery, err
				}
				return Delivery{}, err
			}
			if done {
				return delivery, nil
			}
		case readErr := <-readErrCh:
			timer.Stop()
			if errors.Is(readErr, io.EOF) {
				waitErr, _ := handle.exitResult()
				if waitErr == nil {
					return Delivery{}, errorf(codePluginInternalError, "plugin exited during event delivery", nil)
				}
				return Delivery{}, errorf(codePluginInternalError, "plugin exited during event delivery", waitErr)
			}
			return Delivery{}, errorf(codePluginProtocolViolation, "read plugin event response", readErr)
		case <-handle.done:
			timer.Stop()
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
			timer.Stop()
			runtimeErr := errorf(codePluginEventTimeout, "plugin event response timed out", ctx.Err())
			m.cleanupFailedDelivery(handle, runtimeErr.Code, runtimeErr.Message, runtimeErr.Err)
			return Delivery{}, runtimeErr
		}
	}
}

func (m *Manager) processEventFrame(ctx context.Context, handle *processHandle, eventRequestID string, seenLocalRequestIDs map[string]struct{}, line []byte) (Delivery, bool, error) {
	var envelope frameEnvelope
	if err := json.Unmarshal(line, &envelope); err != nil {
		return Delivery{}, false, errorf(codePluginProtocolViolation, "plugin returned malformed protocol json", err)
	}
	if envelope.ProtocolVersion != "1" {
		return Delivery{}, false, errorf(codePluginProtocolViolation, "plugin returned an unsupported protocol_version", nil)
	}
	if envelope.PluginID == "" || envelope.PluginID != handle.spec.PluginID {
		return Delivery{}, false, errorf(codePluginProtocolViolation, "plugin returned a mismatched plugin_id", nil)
	}
	if envelope.RequestID == "" {
		return Delivery{}, false, errorf(codePluginProtocolViolation, "plugin returned a mismatched request_id", nil)
	}

	if envelope.RequestID != eventRequestID {
		if err := m.handleLocalActionFrame(ctx, handle, envelope, seenLocalRequestIDs, line); err != nil {
			return Delivery{}, false, err
		}
		return Delivery{}, false, nil
	}

	switch envelope.Type {
	case "action":
		var frame actionFrame
		if err := json.Unmarshal(line, &frame); err != nil {
			return Delivery{}, false, errorf(codePluginProtocolViolation, "plugin returned malformed action frame", err)
		}

		switch frame.Action {
		case "message.send":
			action, err := parseMessageSendAction(frame.Data)
			if err != nil {
				return Delivery{}, false, err
			}
			return Delivery{
				RequestID: eventRequestID,
				Action:    action,
			}, true, nil

		case "message.reply":
			action, err := parseMessageReplyAction(frame.Data)
			if err != nil {
				return Delivery{}, false, err
			}
			return Delivery{
				RequestID: eventRequestID,
				Action:    action,
			}, true, nil

		case "logger.write", "storage.kv", "storage.file", "http.request":
			return Delivery{}, false, errorf(codePluginProtocolViolation, "plugin local action request_id must differ from the current event request_id", nil)

		default:
			return Delivery{}, false, errorf(codePluginProtocolViolation, "plugin returned unsupported action kind", nil)
		}
	case "result":
		var frame resultFrame
		if err := json.Unmarshal(line, &frame); err != nil {
			return Delivery{}, false, errorf(codePluginProtocolViolation, "plugin returned malformed result frame", err)
		}
		if frame.Status != "success" {
			return Delivery{}, false, errorf(codePluginProtocolViolation, "plugin result frame must use status=success", nil)
		}
		if frame.Data == nil {
			frame.Data = map[string]any{}
		}
		return Delivery{
			RequestID: eventRequestID,
			Result:    frame.Data,
		}, true, nil
	case "error":
		var frame errorFrame
		if err := json.Unmarshal(line, &frame); err != nil {
			return Delivery{}, false, errorf(codePluginProtocolViolation, "plugin returned malformed error frame", err)
		}
		if frame.Code == "" || frame.Message == "" {
			return Delivery{}, false, errorf(codePluginProtocolViolation, "plugin error frame is missing code or message", nil)
		}
		delivery := Delivery{
			RequestID:    eventRequestID,
			ErrorCode:    frame.Code,
			ErrorMessage: frame.Message,
		}
		return delivery, true, errorf(frame.Code, frame.Message, nil)
	default:
		return Delivery{}, false, errorf(codePluginProtocolViolation, "plugin returned an unexpected protocol message during event delivery", nil)
	}
}

func (m *Manager) handleLocalActionFrame(ctx context.Context, handle *processHandle, envelope frameEnvelope, seenLocalRequestIDs map[string]struct{}, line []byte) error {
	if envelope.Type != "action" {
		return errorf(codePluginProtocolViolation, "plugin returned an unexpected protocol message during local action handling", nil)
	}
	if _, exists := seenLocalRequestIDs[envelope.RequestID]; exists {
		return errorf(codePluginProtocolViolation, "plugin reused a local action request_id within one event delivery", nil)
	}

	var frame actionFrame
	if err := json.Unmarshal(line, &frame); err != nil {
		return errorf(codePluginProtocolViolation, "plugin returned malformed action frame", err)
	}

	var action *Action
	var err error
	switch frame.Action {
	case "logger.write":
		action, err = parseLoggerWriteAction(frame.Data)
	case "storage.kv":
		action, err = parseStorageKVAction(frame.Data)
	case "storage.file":
		action, err = parseStorageFileAction(frame.Data)
	case "http.request":
		action, err = parseHTTPRequestAction(frame.Data)
	case "message.send", "message.reply":
		return errorf(codePluginProtocolViolation, "terminal message actions must use the current event request_id", nil)
	default:
		return errorf(codePluginProtocolViolation, "plugin returned unsupported action kind", nil)
	}
	if err != nil {
		return err
	}

	seenLocalRequestIDs[envelope.RequestID] = struct{}{}
	return m.executeLocalAction(ctx, handle, envelope.RequestID, *action)
}

func (m *Manager) executeLocalAction(ctx context.Context, handle *processHandle, requestID string, action Action) error {
	if m.opts.ExecuteLocalAction == nil {
		return errorf(codePluginInternalError, "plugin local action executor is not available", nil)
	}

	result, err := m.opts.ExecuteLocalAction(ctx, handle.spec.PluginID, requestID, action)
	if err != nil {
		var runtimeErr *Error
		if errors.As(err, &runtimeErr) {
			return m.writeLocalError(handle, requestID, runtimeErr.Code, runtimeErr.Message)
		}
		return m.writeLocalError(handle, requestID, codePluginInternalError, "plugin local action failed")
	}

	if result == nil {
		result = map[string]any{}
	}
	return m.writeLocalResult(handle, requestID, result)
}

func (m *Manager) writeLocalResult(handle *processHandle, requestID string, data map[string]any) error {
	frame := map[string]any{
		"protocol_version": "1",
		"type":             "result",
		"timestamp":        m.deps.now().Unix(),
		"plugin_id":        handle.spec.PluginID,
		"request_id":       requestID,
		"status":           "success",
		"data":             data,
	}
	if err := writeJSONLine(handle.stdin, frame); err != nil {
		return errorf(codePluginInternalError, "write local action result frame", err)
	}
	return nil
}

func (m *Manager) writeLocalError(handle *processHandle, requestID string, code string, message string) error {
	frame := map[string]any{
		"protocol_version": "1",
		"type":             "error",
		"timestamp":        m.deps.now().Unix(),
		"plugin_id":        handle.spec.PluginID,
		"request_id":       requestID,
		"code":             code,
		"message":          message,
	}
	if err := writeJSONLine(handle.stdin, frame); err != nil {
		return errorf(codePluginInternalError, "write local action error frame", err)
	}
	return nil
}

func parseMessageSendAction(raw json.RawMessage) (*Action, error) {
	var frame protocolActionMessageSendFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		return nil, errorf(codePluginProtocolViolation, "plugin returned malformed message.send data", err)
	}

	targetType, targetID, err := validateActionTarget(frame.TargetType, frame.TargetID, "message.send")
	if err != nil {
		return nil, err
	}

	if frame.Message == nil {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required message.send fields", nil)
	}
	segments, err := parseOutboundActionSegments(frame.Message.Segments)
	if err != nil {
		return nil, err
	}
	return &Action{
		Kind:            "message.send",
		TargetType:      targetType,
		TargetID:        targetID,
		MessageSegments: segments,
	}, nil
}

func parseMessageReplyAction(raw json.RawMessage) (*Action, error) {
	var frame protocolActionMessageReplyFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		return nil, errorf(codePluginProtocolViolation, "plugin returned malformed message.reply data", err)
	}

	if frame.ReplyToEventID == nil || frame.Message == nil {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required message.reply fields", nil)
	}
	replyToEventID := strings.TrimSpace(*frame.ReplyToEventID)
	if replyToEventID == "" {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required message.reply fields", nil)
	}
	segments, err := parseOutboundActionSegments(frame.Message.Segments)
	if err != nil {
		return nil, err
	}
	return &Action{
		Kind:                    "message.reply",
		ReplyToEventID:          replyToEventID,
		FallbackToSendIfMissing: frame.FallbackToSendIfMissing,
		MessageSegments:         segments,
	}, nil
}

func parseLoggerWriteAction(raw json.RawMessage) (*Action, error) {
	var frame protocolActionLoggerWriteFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		return nil, errorf(codePluginProtocolViolation, "plugin returned malformed logger.write data", err)
	}

	level := strings.TrimSpace(frame.Level)
	switch level {
	case "debug", "info", "warn", "error":
	default:
		return nil, errorf(codePluginProtocolViolation, "plugin action frame has invalid logger.write level", nil)
	}

	message := strings.TrimSpace(frame.Message)
	if message == "" {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required logger.write fields", nil)
	}

	return &Action{
		Kind:       "logger.write",
		LogLevel:   level,
		LogMessage: message,
		LogFields:  cloneActionSegmentData(frame.Fields),
	}, nil
}

func parseStorageKVAction(raw json.RawMessage) (*Action, error) {
	var frame protocolActionStorageKVFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		return nil, errorf(codePluginProtocolViolation, "plugin returned malformed storage.kv data", err)
	}

	switch strings.TrimSpace(frame.Operation) {
	case "get":
		if frame.Key == nil {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required storage.kv fields", nil)
		}
		key := strings.TrimSpace(*frame.Key)
		if key == "" {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required storage.kv fields", nil)
		}
		return &Action{Kind: "storage.kv", StorageOperation: "get", StorageKey: key}, nil
	case "set":
		if frame.Key == nil || frame.Value == nil {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required storage.kv fields", nil)
		}
		key := strings.TrimSpace(*frame.Key)
		if key == "" {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required storage.kv fields", nil)
		}
		var value any
		if err := json.Unmarshal(*frame.Value, &value); err != nil {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame has invalid storage.kv value", err)
		}
		return &Action{Kind: "storage.kv", StorageOperation: "set", StorageKey: key, StorageValue: value}, nil
	case "delete":
		if frame.Key == nil {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required storage.kv fields", nil)
		}
		key := strings.TrimSpace(*frame.Key)
		if key == "" {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required storage.kv fields", nil)
		}
		return &Action{Kind: "storage.kv", StorageOperation: "delete", StorageKey: key}, nil
	case "list":
		if frame.Prefix == nil {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required storage.kv fields", nil)
		}
		prefix := *frame.Prefix
		return &Action{Kind: "storage.kv", StorageOperation: "list", StoragePrefix: prefix}, nil
	default:
		return nil, errorf(codePluginProtocolViolation, "plugin action frame uses unsupported storage.kv operation", nil)
	}
}

func parseStorageFileAction(raw json.RawMessage) (*Action, error) {
	var frame protocolActionStorageFileFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		return nil, errorf(codePluginProtocolViolation, "plugin returned malformed storage.file data", err)
	}

	if strings.TrimSpace(frame.Root) != "plugin_data" {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame uses unsupported storage.file root", nil)
	}

	switch strings.TrimSpace(frame.Operation) {
	case "read":
		if frame.Path == nil || *frame.Path == "" {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required storage.file fields", nil)
		}
		return &Action{Kind: "storage.file", StorageOperation: "read", StorageRoot: "plugin_data", StoragePath: *frame.Path}, nil
	case "write":
		if frame.Path == nil {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required storage.file fields", nil)
		}
		content, err := decodeExclusiveTextOrBase64(frame.ContentText, frame.ContentBase64, true)
		if err != nil {
			return nil, err
		}
		return &Action{
			Kind:             "storage.file",
			StorageOperation: "write",
			StorageRoot:      "plugin_data",
			StoragePath:      *frame.Path,
			StorageContent:   content,
		}, nil
	case "delete":
		if frame.Path == nil || *frame.Path == "" {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required storage.file fields", nil)
		}
		return &Action{Kind: "storage.file", StorageOperation: "delete", StorageRoot: "plugin_data", StoragePath: *frame.Path}, nil
	case "list":
		if frame.Prefix == nil {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required storage.file fields", nil)
		}
		return &Action{Kind: "storage.file", StorageOperation: "list", StorageRoot: "plugin_data", StoragePrefix: *frame.Prefix}, nil
	default:
		return nil, errorf(codePluginProtocolViolation, "plugin action frame uses unsupported storage.file operation", nil)
	}
}

func parseHTTPRequestAction(raw json.RawMessage) (*Action, error) {
	var frame protocolActionHTTPRequestFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		return nil, errorf(codePluginProtocolViolation, "plugin returned malformed http.request data", err)
	}

	method := strings.ToUpper(strings.TrimSpace(frame.Method))
	switch method {
	case "GET", "HEAD", "POST", "PUT", "PATCH", "DELETE":
	default:
		return nil, errorf(codePluginProtocolViolation, "plugin action frame uses unsupported http.request method", nil)
	}

	targetURL := strings.TrimSpace(frame.URL)
	if targetURL == "" {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required http.request fields", nil)
	}

	body, err := decodeExclusiveTextOrBase64(frame.BodyText, frame.BodyBase64, false)
	if err != nil {
		return nil, err
	}
	if (method == "GET" || method == "HEAD") && len(body) > 0 {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame uses unsupported http.request body for method", nil)
	}

	timeoutSeconds := 0
	if frame.TimeoutSeconds != nil {
		timeoutSeconds = *frame.TimeoutSeconds
		if timeoutSeconds <= 0 {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame has invalid http.request timeout_seconds", nil)
		}
	}

	return &Action{
		Kind:               "http.request",
		HTTPMethod:         method,
		HTTPURL:            targetURL,
		HTTPHeaders:        cloneHTTPActionHeaders(frame.Headers),
		HTTPTimeoutSeconds: timeoutSeconds,
		HTTPBody:           body,
	}, nil
}

func decodeExclusiveTextOrBase64(text *string, encoded *string, required bool) ([]byte, error) {
	if text != nil && encoded != nil {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame mixes text and base64 content fields", nil)
	}
	if text != nil {
		return []byte(*text), nil
	}
	if encoded != nil {
		content, err := base64.StdEncoding.DecodeString(*encoded)
		if err != nil {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame has invalid base64 content", err)
		}
		return content, nil
	}
	if !required {
		return nil, nil
	}
	return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required text or base64 content fields", nil)
}

func cloneHTTPActionHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return map[string]string{}
	}
	cloned := make(map[string]string, len(headers))
	for key, value := range headers {
		cloned[key] = value
	}
	return cloned
}

func validateActionTarget(rawType, rawID, actionKind string) (string, string, error) {
	targetType := strings.TrimSpace(rawType)
	targetID := strings.TrimSpace(rawID)
	if targetID == "" {
		return "", "", errorf(codePluginProtocolViolation, "plugin action frame is missing required "+actionKind+" fields", nil)
	}
	switch targetType {
	case "group", "private":
		return targetType, targetID, nil
	default:
		return "", "", errorf(codePluginProtocolViolation, "plugin action frame uses unsupported target_type", nil)
	}
}

func parseOutboundActionSegments(raw []protocolSegmentFrame) ([]ActionSegment, error) {
	if len(raw) == 0 {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required rich message segments", nil)
	}

	segments := make([]ActionSegment, 0, len(raw))
	for index, segment := range raw {
		actionSegment, err := parseOutboundActionSegment(segment, index)
		if err != nil {
			return nil, err
		}
		segments = append(segments, actionSegment)
	}
	return segments, nil
}

func parseOutboundActionSegment(segment protocolSegmentFrame, index int) (ActionSegment, error) {
	segmentType := strings.TrimSpace(segment.Type)
	data := cloneActionSegmentData(segment.Data)

	switch segmentType {
	case "text":
		text, ok := data["text"].(string)
		if !ok || strings.TrimSpace(text) == "" {
			return ActionSegment{}, errorf(codePluginProtocolViolation, "plugin action frame has invalid text segment", nil)
		}
		data["text"] = text
	case "image":
		file := outboundActionString(data, "file")
		url := outboundActionString(data, "url")
		if file == "" && url == "" {
			return ActionSegment{}, errorf(codePluginProtocolViolation, "plugin action frame has invalid image segment", nil)
		}
		if file != "" {
			data["file"] = file
		}
		if url != "" {
			data["url"] = url
		}
	case "at":
		userID := outboundActionString(data, "user_id")
		if userID == "" {
			return ActionSegment{}, errorf(codePluginProtocolViolation, "plugin action frame has invalid at segment", nil)
		}
		data["user_id"] = userID
	case "at_all":
		data = map[string]any{}
	case "face":
		faceID := outboundActionString(data, "face_id")
		if faceID == "" {
			return ActionSegment{}, errorf(codePluginProtocolViolation, "plugin action frame has invalid face segment", nil)
		}
		data["face_id"] = faceID
	case "reply":
		if index != 0 {
			return ActionSegment{}, errorf(codePluginProtocolViolation, "plugin action frame places reply segment outside the message head", nil)
		}
		messageID := outboundActionString(data, "message_id")
		if messageID == "" {
			return ActionSegment{}, errorf(codePluginProtocolViolation, "plugin action frame has invalid reply segment", nil)
		}
		data["message_id"] = messageID
	default:
		return ActionSegment{}, errorf(codePluginProtocolViolation, "plugin action frame uses unsupported message segment type", nil)
	}

	return ActionSegment{
		Type: segmentType,
		Data: data,
	}, nil
}

func cloneActionSegmentData(data map[string]any) map[string]any {
	if len(data) == 0 {
		return map[string]any{}
	}
	cloned := make(map[string]any, len(data))
	for key, value := range data {
		cloned[key] = value
	}
	return cloned
}

func outboundActionString(data map[string]any, key string) string {
	if len(data) == 0 {
		return ""
	}
	value, ok := data[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
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
	cloned.Subscriptions = append([]string(nil), snapshot.Subscriptions...)
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
