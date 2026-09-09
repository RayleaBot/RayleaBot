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
	_ "time/tzdata"
)

const maxProtocolFrameBytes = 8 * 1024 * 1024

type runtimeState struct {
	location        *time.Location
	client          *runtimeClient
	pluginID        string
	handler         Handler
	logger          *slog.Logger
	semaphore       chan struct{}
	handlers        sync.WaitGroup
	shutdownGrace   time.Duration
	cancel          context.CancelFunc
	botMu           sync.RWMutex
	bot             Bot
	permissions     []string
	superAdmins     []string
	commandPrefixes []string
	config          atomic.Pointer[configSnapshot]
}

type configSnapshot struct {
	values map[string]any
}

type EventContext struct {
	// Location is the effective host timezone captured for this process session.
	Location        *time.Location
	Event           Event
	RequestID       string
	PluginID        string
	Bot             Bot
	Config          map[string]any
	Permissions     []string
	SuperAdmins     []string
	CommandPrefixes []string

	client   *runtimeClient
	terminal atomic.Bool
	actionMu sync.Mutex
	actions  map[string]chan struct{}
}

func Run(ctx context.Context, options Options, handler Handler) error {
	if handler == nil {
		return errors.New("rayleabot: handler is required")
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
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	state := &runtimeState{
		client:        newRuntimeClient(out, actionTimeout),
		handler:       handler,
		logger:        logger,
		shutdownGrace: shutdownGrace,
		cancel:        cancel,
	}
	state.config.Store(&configSnapshot{values: map[string]any{}})
	defer state.client.rejectPending(context.Canceled)
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
		if state.client.routeResponse(frame) {
			continue
		}
		switch frame.Type {
		case "init":
			if initialized {
				return protocolError("received duplicate init")
			}
			if frame.ProtocolVersion != ProtocolVersion {
				return fmt.Errorf("rayleabot: unsupported protocol version %q", frame.ProtocolVersion)
			}
			if strings.TrimSpace(frame.PluginID) == "" {
				return protocolError("init plugin_id is required")
			}
			if strings.TrimSpace(frame.Timezone) == "" || frame.Timezone == "Local" {
				return protocolError("init timezone must be an IANA timezone")
			}
			location, err := time.LoadLocation(frame.Timezone)
			if err != nil {
				return protocolError("init timezone is invalid")
			}
			state.location = location
			state.captureInit(frame)
			if err := state.client.writer.write(protocolFrame{
				Type:      "init_ack",
				RequestID: frame.RequestID,
				Status:    "ready",
			}); err != nil {
				return err
			}
			initialized = true
		case "event":
			if !initialized {
				return protocolError("received event before init")
			}
			event, err := state.decodeEvent(frame)
			if err != nil {
				if writeErr := state.sendError(frame.RequestID, "plugin.protocol_violation", err.Error()); writeErr != nil {
					return writeErr
				}
				continue
			}
			if err := state.applyControlEvent(event); err != nil {
				if writeErr := state.sendError(frame.RequestID, "plugin.protocol_violation", err.Error()); writeErr != nil {
					return writeErr
				}
				continue
			}
			state.startEvent(ctx, frame.RequestID, event)
		case "ping":
			if err := state.client.writer.write(protocolFrame{
				Type:      "pong",
				RequestID: frame.RequestID,
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
	state.pluginID = strings.TrimSpace(frame.PluginID)
	state.permissions = append([]string(nil), frame.EffectivePermissions...)
	state.superAdmins = append([]string(nil), frame.SuperAdmins...)
	state.commandPrefixes = append([]string(nil), frame.CommandPrefixes...)
	if len(state.commandPrefixes) == 0 {
		state.commandPrefixes = []string{"/"}
	}
	concurrency := frame.Concurrency
	if concurrency < 1 {
		concurrency = 1
	}
	state.semaphore = make(chan struct{}, concurrency)
	state.config.Store(&configSnapshot{values: cloneConfig(frame.Config)})
}

func (state *runtimeState) decodeEvent(frame protocolFrame) (Event, error) {
	var event Event
	if err := json.Unmarshal(frame.Event, &event); err != nil {
		return Event{}, protocolError("invalid event payload")
	}
	event.Raw = append(json.RawMessage(nil), frame.Event...)
	return event, nil
}

func (state *runtimeState) applyControlEvent(event Event) error {
	switch event.EventType {
	case "config.changed":
		value, exists := event.Payload["config"]
		if !exists {
			return protocolError("config.changed payload.config is required")
		}
		config, ok := value.(map[string]any)
		if !ok {
			return protocolError("config.changed payload.config must be an object")
		}
		state.config.Store(&configSnapshot{values: cloneConfig(config)})
	case "bot.identity.changed":
		state.updateBotIdentity(event)
	}
	return nil
}

func (state *runtimeState) startEvent(ctx context.Context, requestID string, event Event) {
	state.handlers.Add(1)
	go func() {
		defer state.handlers.Done()
		select {
		case state.semaphore <- struct{}{}:
			defer func() { <-state.semaphore }()
		case <-ctx.Done():
			return
		}
		if ctx.Err() != nil {
			return
		}
		eventContext := state.newEventContext(requestID, event)
		defer func() {
			if recovered := recover(); recovered != nil {
				state.logger.Error("plugin event handler panic", "request_id", requestID, "panic", redact(fmt.Sprint(recovered)), "stack", string(debug.Stack()))
				if !eventContext.terminal.Load() {
					_ = eventContext.Fail("plugin.internal_error", "plugin event handler panicked")
				}
			}
		}()
		err := state.handler.Handle(ctx, eventContext)
		if err != nil {
			if ctx.Err() != nil {
				state.logger.Debug("plugin event canceled during shutdown", "request_id", requestID)
				return
			}
			state.logger.Error("plugin event handler failed", "request_id", requestID, "err", redact(err.Error()))
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
	state.botMu.Lock()
	if botID == "" {
		state.bot = Bot{}
	} else {
		state.bot.ID = botID
	}
	state.botMu.Unlock()
}

func (state *runtimeState) newEventContext(requestID string, event Event) *EventContext {
	state.botMu.RLock()
	defer state.botMu.RUnlock()
	return &EventContext{
		Location:        state.location,
		Event:           event,
		RequestID:       requestID,
		PluginID:        state.pluginID,
		Bot:             state.bot,
		Config:          cloneConfig(state.config.Load().values),
		Permissions:     append([]string(nil), state.permissions...),
		SuperAdmins:     append([]string(nil), state.superAdmins...),
		CommandPrefixes: append([]string(nil), state.commandPrefixes...),
		client:          state.client,
	}
}

func (state *runtimeState) sendError(requestID, code, message string) error {
	return state.client.writer.write(protocolFrame{
		Type:      "error",
		RequestID: requestID,
		Code:      code,
		Message:   redact(message),
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
	raw, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("rayleabot: marshal result: %w", err)
	}
	return event.writeTerminal(protocolFrame{
		Type:      "result",
		RequestID: event.RequestID,
		Status:    "success",
		Data:      raw,
	})
}

func (event *EventContext) Fail(code, message string) error {
	return event.writeTerminal(protocolFrame{
		Type:      "error",
		RequestID: event.RequestID,
		Code:      code,
		Message:   redact(message),
	})
}

func (event *EventContext) Send(targetType, targetID string, segments ...Segment) error {
	data, err := json.Marshal(map[string]any{
		"target_type": targetType,
		"target_id":   targetID,
		"message":     map[string]any{"segments": segments},
	})
	if err != nil {
		return err
	}
	return event.writeTerminal(protocolFrame{
		Type:      "action",
		RequestID: event.RequestID,
		Action:    "message.send",
		Data:      data,
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
	payload := map[string]any{
		"target_type":       event.Event.Target.Type,
		"target_id":         event.Event.Target.ID,
		"reply_to_event_id": replyToEventID,
		"message":           map[string]any{"segments": segments},
	}
	if fallback {
		payload["fallback_to_send_if_missing"] = true
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return event.writeTerminal(protocolFrame{
		Type:      "action",
		RequestID: event.RequestID,
		Action:    "message.send",
		Data:      data,
	})
}

func (event *EventContext) Actions() *Actions {
	return &Actions{event: event}
}

func cloneConfig(values map[string]any) map[string]any {
	result := make(map[string]any, len(values))
	for key, value := range values {
		result[key] = cloneConfigValue(value)
	}
	return result
}

func cloneConfigValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneConfig(typed)
	case []any:
		items := make([]any, len(typed))
		for index, item := range typed {
			items[index] = cloneConfigValue(item)
		}
		return items
	default:
		return value
	}
}
