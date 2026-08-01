package rayleabot

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const maxProtocolFrameBytes = 8 * 1024 * 1024

type runtimeState struct {
	client          *runtimeClient
	handler         Handler
	logger          *slog.Logger
	subscriptions   []string
	semaphore       chan struct{}
	handlers        sync.WaitGroup
	shutdownGrace   time.Duration
	cancel          context.CancelFunc
	botMu           sync.RWMutex
	bot             Bot
	capabilities    []string
	superAdmins     []string
	commandPrefixes []string
}

type EventContext struct {
	Event           Event
	RequestID       string
	PluginID        string
	Bot             Bot
	Capabilities    []string
	SuperAdmins     []string
	CommandPrefixes []string

	client   *runtimeClient
	terminal atomic.Bool
}

func Run(ctx context.Context, options Options, handler Handler) error {
	if handler == nil {
		return errors.New("rayleabot: handler is required")
	}
	pluginID := strings.TrimSpace(options.PluginID)
	if pluginID == "" {
		return errors.New("rayleabot: options.PluginID is required")
	}
	in := options.Stdin
	if in == nil {
		in = os.Stdin
	}
	out := options.Stdout
	if out == nil {
		out = os.Stdout
	}
	stderr := options.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}
	logger := options.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}
	actionTimeout := options.ActionTimeout
	if actionTimeout <= 0 {
		actionTimeout = 30 * time.Second
	}
	shutdownGrace := options.ShutdownGrace
	if shutdownGrace <= 0 {
		shutdownGrace = 5 * time.Second
	}
	maxHandlers := options.MaxConcurrentHandlers
	if maxHandlers <= 0 {
		maxHandlers = 1
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	state := &runtimeState{
		client:        newRuntimeClient(pluginID, out, actionTimeout),
		handler:       handler,
		logger:        logger,
		subscriptions: append([]string(nil), options.Subscriptions...),
		semaphore:     make(chan struct{}, maxHandlers),
		shutdownGrace: shutdownGrace,
		cancel:        cancel,
	}
	return state.run(runCtx, in)
}

func (state *runtimeState) run(ctx context.Context, in io.Reader) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 64*1024), maxProtocolFrameBytes)
	initialized := false
	for scanner.Scan() {
		var frame protocolFrame
		if err := json.Unmarshal(scanner.Bytes(), &frame); err != nil {
			state.client.rejectPending(err)
			return fmt.Errorf("rayleabot: decode protocol frame: %w", err)
		}
		if frame.ProtocolVersion != ProtocolVersion {
			return fmt.Errorf("rayleabot: unsupported protocol version %q", frame.ProtocolVersion)
		}
		if state.client.routeResponse(frame) {
			continue
		}
		switch frame.Type {
		case "init":
			if initialized {
				return protocolError("received duplicate init")
			}
			if frame.PluginID != "" && frame.PluginID != state.client.pluginID {
				return fmt.Errorf("rayleabot: init plugin id %q does not match %q", frame.PluginID, state.client.pluginID)
			}
			state.captureInit(frame)
			if err := state.client.writer.write(protocolFrame{
				ProtocolVersion: ProtocolVersion,
				Type:            "init_ack",
				Timestamp:       time.Now().Unix(),
				PluginID:        state.client.pluginID,
				RequestID:       frame.RequestID,
				Status:          "ready",
				Subscriptions:   state.subscriptions,
			}); err != nil {
				return err
			}
			initialized = true
		case "event":
			if !initialized {
				return protocolError("received event before init")
			}
			state.startEvent(ctx, frame)
		case "ping":
			if err := state.client.writer.write(protocolFrame{
				ProtocolVersion: ProtocolVersion,
				Type:            "pong",
				Timestamp:       time.Now().Unix(),
				PluginID:        state.client.pluginID,
				RequestID:       frame.RequestID,
			}); err != nil {
				return err
			}
		case "shutdown":
			state.cancel()
			state.client.rejectPending(errors.New("received shutdown"))
			return state.waitHandlers()
		default:
			return fmt.Errorf("rayleabot: unsupported frame type %q", frame.Type)
		}
	}
	state.cancel()
	if err := scanner.Err(); err != nil {
		state.client.rejectPending(err)
		return fmt.Errorf("rayleabot: read stdin: %w", err)
	}
	state.client.rejectPending(io.EOF)
	if initialized {
		return state.waitHandlers()
	}
	return io.EOF
}

func (state *runtimeState) captureInit(frame protocolFrame) {
	state.botMu.Lock()
	defer state.botMu.Unlock()
	state.bot = frame.Bot
	state.capabilities = append([]string(nil), frame.Capabilities...)
	state.superAdmins = append([]string(nil), frame.Permissions.SuperAdmins...)
	state.commandPrefixes = append([]string(nil), frame.CommandPrefixes...)
	if len(state.commandPrefixes) == 0 {
		state.commandPrefixes = []string{"/"}
	}
}

func (state *runtimeState) startEvent(ctx context.Context, frame protocolFrame) {
	state.handlers.Add(1)
	go func() {
		defer state.handlers.Done()
		select {
		case state.semaphore <- struct{}{}:
			defer func() { <-state.semaphore }()
		case <-ctx.Done():
			return
		}
		var event Event
		if err := json.Unmarshal(frame.Event, &event); err != nil {
			_ = state.sendError(frame.RequestID, "plugin.protocol_violation", "invalid event payload")
			return
		}
		event.Raw = append(json.RawMessage(nil), frame.Event...)
		state.updateBotIdentity(event)
		eventContext := state.newEventContext(frame.RequestID, event)
		defer func() {
			if recovered := recover(); recovered != nil {
				state.logger.Error("plugin event handler panic", "request_id", frame.RequestID, "panic", redact(fmt.Sprint(recovered)), "stack", string(debug.Stack()))
				if !eventContext.terminal.Load() {
					_ = eventContext.Fail("plugin.internal_error", "plugin event handler panicked")
				}
			}
		}()
		err := state.handler.Handle(ctx, eventContext)
		if err != nil {
			state.logger.Error("plugin event handler failed", "request_id", frame.RequestID, "err", redact(err.Error()))
			if !eventContext.terminal.Load() {
				_ = eventContext.Fail("plugin.internal_error", err.Error())
			}
			return
		}
		if !eventContext.terminal.Load() {
			_ = eventContext.Result(map[string]any{})
		}
	}()
}

func (state *runtimeState) updateBotIdentity(event Event) {
	if event.EventType != "bot.identity.changed" {
		return
	}
	botID := ""
	if event.Target.Type == "bot" {
		botID = event.Target.ID
	}
	if botID == "" {
		if onebot, ok := event.Payload["onebot"].(map[string]any); ok {
			botID, _ = onebot["self_id"].(string)
		}
	}
	if botID == "" {
		return
	}
	state.botMu.Lock()
	state.bot.ID = botID
	state.botMu.Unlock()
}

func (state *runtimeState) newEventContext(requestID string, event Event) *EventContext {
	state.botMu.RLock()
	defer state.botMu.RUnlock()
	return &EventContext{
		Event:           event,
		RequestID:       requestID,
		PluginID:        state.client.pluginID,
		Bot:             state.bot,
		Capabilities:    append([]string(nil), state.capabilities...),
		SuperAdmins:     append([]string(nil), state.superAdmins...),
		CommandPrefixes: append([]string(nil), state.commandPrefixes...),
		client:          state.client,
	}
}

func (state *runtimeState) sendError(requestID, code, message string) error {
	return state.client.writer.write(protocolFrame{
		ProtocolVersion: ProtocolVersion,
		Type:            "error",
		Timestamp:       time.Now().Unix(),
		PluginID:        state.client.pluginID,
		RequestID:       requestID,
		Code:            code,
		Message:         redact(message),
	})
}

func (state *runtimeState) waitHandlers() error {
	done := make(chan struct{})
	go func() {
		state.handlers.Wait()
		close(done)
	}()
	timer := time.NewTimer(state.shutdownGrace)
	defer timer.Stop()
	select {
	case <-done:
		return nil
	case <-timer.C:
		return errors.New("rayleabot: event handlers did not stop before shutdown grace expired")
	}
}

func (event *EventContext) Result(data any) error {
	if !event.terminal.CompareAndSwap(false, true) {
		return errors.New("rayleabot: terminal response already sent")
	}
	raw, err := json.Marshal(data)
	if err != nil {
		event.terminal.Store(false)
		return fmt.Errorf("rayleabot: marshal result: %w", err)
	}
	return event.client.writer.write(protocolFrame{
		ProtocolVersion: ProtocolVersion,
		Type:            "result",
		Timestamp:       time.Now().Unix(),
		PluginID:        event.PluginID,
		RequestID:       event.RequestID,
		Status:          "success",
		Data:            raw,
	})
}

func (event *EventContext) Fail(code, message string) error {
	if !event.terminal.CompareAndSwap(false, true) {
		return errors.New("rayleabot: terminal response already sent")
	}
	return event.client.writer.write(protocolFrame{
		ProtocolVersion: ProtocolVersion,
		Type:            "error",
		Timestamp:       time.Now().Unix(),
		PluginID:        event.PluginID,
		RequestID:       event.RequestID,
		Code:            code,
		Message:         redact(message),
	})
}

func (event *EventContext) Send(targetType, targetID string, segments ...Segment) error {
	if !event.terminal.CompareAndSwap(false, true) {
		return errors.New("rayleabot: terminal response already sent")
	}
	data, err := json.Marshal(map[string]any{
		"target_type": targetType,
		"target_id":   targetID,
		"message":     map[string]any{"segments": segments},
	})
	if err != nil {
		event.terminal.Store(false)
		return err
	}
	return event.client.writer.write(protocolFrame{
		ProtocolVersion: ProtocolVersion,
		Type:            "action",
		Timestamp:       time.Now().Unix(),
		PluginID:        event.PluginID,
		RequestID:       event.RequestID,
		Action:          "message.send",
		Data:            data,
	})
}

func (event *EventContext) SendText(text string) error {
	targetType := event.Event.Target.Type
	if targetType == "" {
		targetType = "group"
	}
	return event.Send(targetType, event.Event.Target.ID, Text(text))
}

func (event *EventContext) Reply(replyToEventID string, fallback bool, segments ...Segment) error {
	if !event.terminal.CompareAndSwap(false, true) {
		return errors.New("rayleabot: terminal response already sent")
	}
	payload := map[string]any{
		"reply_to_event_id": replyToEventID,
		"message":           map[string]any{"segments": segments},
	}
	if fallback {
		payload["fallback_to_send_if_missing"] = true
	}
	data, err := json.Marshal(payload)
	if err != nil {
		event.terminal.Store(false)
		return err
	}
	return event.client.writer.write(protocolFrame{
		ProtocolVersion: ProtocolVersion,
		Type:            "action",
		Timestamp:       time.Now().Unix(),
		PluginID:        event.PluginID,
		RequestID:       event.RequestID,
		Action:          "message.reply",
		Data:            data,
	})
}

func (event *EventContext) Actions() *Actions {
	return &Actions{event: event}
}
