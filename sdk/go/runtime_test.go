package rayleabot

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRuntimeStateClearsBotIdentities(t *testing.T) {
	state := &runtimeState{bots: []Bot{{SourceAdapter: "onebot", SourceProtocol: "onebot11", ID: "10001", Nickname: "RayleaBot"}}}
	if err := state.updateBotIdentities(Event{EventType: "bot.identities.changed", Payload: map[string]any{"bots": []Bot{}}}); err != nil {
		t.Fatal(err)
	}
	if len(state.bots) != 0 {
		t.Fatalf("identity snapshot not cleared: %#v", state.bots)
	}
}

func TestInvalidIdentitySnapshotPreservesPreviousState(t *testing.T) {
	t.Parallel()
	bot := Bot{SourceAdapter: "onebot", SourceProtocol: "onebot11", ID: "original"}
	for _, value := range []any{nil, "invalid", []Bot{bot, bot}, []Bot{{SourceAdapter: "onebot", SourceProtocol: "unknown", ID: "other"}}} {
		state := &runtimeState{bots: []Bot{bot}}
		if err := state.updateBotIdentities(Event{Payload: map[string]any{"bots": value}}); err == nil {
			t.Fatalf("invalid identity snapshot was accepted: %#v", value)
		}
		if len(state.bots) != 1 || state.bots[0] != bot {
			t.Fatal("invalid update changed identity state")
		}
	}
}

func TestEventContextSelectsIdentityByAdapter(t *testing.T) {
	bots := []Bot{{SourceAdapter: "onebot", SourceProtocol: "onebot11", ID: "shared"}, {SourceAdapter: "qq", SourceProtocol: "qqofficial", ID: "shared"}}
	state := &runtimeState{}
	state.captureInit(protocolFrame{Bots: &bots})
	for _, bot := range bots {
		event := state.newEventContext("fixture", Event{SourceAdapter: bot.SourceAdapter, SourceProtocol: bot.SourceProtocol})
		if event.Bot != bot {
			t.Fatalf("selected %#v, want %#v", event.Bot, bot)
		}
		event.Bots[0].ID = "mutated"
	}
	if state.bots[0].ID != "shared" {
		t.Fatal("shared mutable identities")
	}
	if state.newEventContext("task", Event{SourceProtocol: "scheduler"}).Bot.ID != "" {
		t.Fatal("ambiguous task identity was guessed")
	}
	if state.newEventContext("missing", Event{SourceAdapter: "qq", SourceProtocol: "onebot11"}).Bot.ID != "" {
		t.Fatal("protocol mismatch selected an identity")
	}
}

func TestConfigChangedReplacesAtomicSnapshot(t *testing.T) {
	state := &runtimeState{}
	state.config.Store(&configSnapshot{values: map[string]any{"mode": "old"}})
	if err := state.applyControlEvent(Event{
		EventType: "config.changed",
		Payload: map[string]any{
			"config":       map[string]any{"mode": "new", "nested": map[string]any{"enabled": true}},
			"changed_keys": []any{"mode", "nested"},
		},
	}); err != nil {
		t.Fatalf("applyControlEvent: %v", err)
	}

	first := state.newEventContext("event-1", Event{})
	if first.Config["mode"] != "new" {
		t.Fatalf("config snapshot = %#v", first.Config)
	}
	first.Config["mode"] = "mutated"
	first.Config["nested"].(map[string]any)["enabled"] = false
	second := state.newEventContext("event-2", Event{})
	if second.Config["mode"] != "new" || second.Config["nested"].(map[string]any)["enabled"] != true {
		t.Fatalf("config snapshot was not isolated: %#v", second.Config)
	}
}

func TestConfigChangedClearsSnapshotAndRejectsInvalidPayload(t *testing.T) {
	state := &runtimeState{}
	state.config.Store(&configSnapshot{values: map[string]any{"mode": "old"}})

	if err := state.applyControlEvent(Event{
		EventType: "config.changed",
		Payload:   map[string]any{"config": map[string]any{}, "changed_keys": []any{"mode"}},
	}); err != nil {
		t.Fatalf("apply empty config: %v", err)
	}
	if snapshot := state.newEventContext("event-empty", Event{}).Config; len(snapshot) != 0 {
		t.Fatalf("empty config did not clear the previous snapshot: %#v", snapshot)
	}

	tests := []Event{
		{EventType: "config.changed", Payload: map[string]any{"changed_keys": []any{"mode"}}},
		{EventType: "config.changed", Payload: map[string]any{"config": "invalid", "changed_keys": []any{"mode"}}},
	}
	for _, event := range tests {
		if err := state.applyControlEvent(event); err == nil {
			t.Fatalf("invalid config.changed payload was accepted: %#v", event.Payload)
		}
	}
}

func TestRunAppliesControlEventsInInputOrderBeforeBusinessHandlers(t *testing.T) {
	inputReader, inputWriter := io.Pipe()
	outputReader, outputWriter := io.Pipe()
	defer inputReader.Close()
	defer outputReader.Close()

	type observation struct {
		zone      string
		eventType string
		config    map[string]any
		bot       Bot
	}
	observations := make(chan observation, 4)
	handler := HandlerFunc(func(_ context.Context, event *EventContext) error {
		observations <- observation{
			zone:      event.Actions().TimeLocation().String(),
			eventType: event.Event.EventType,
			config:    event.Config,
			bot:       event.Bot,
		}
		return event.Result(map[string]any{})
	})

	runDone := make(chan error, 1)
	go func() {
		runDone <- Run(context.Background(), Options{Stdin: inputReader, Stdout: outputWriter}, handler)
	}()
	encoder := json.NewEncoder(inputWriter)
	decoder := json.NewDecoder(outputReader)
	writeFrame(t, encoder, protocolFrame{
		ProtocolVersion: ProtocolVersion, Type: "init", Timezone: "Asia/Shanghai", PluginID: "test-plugin", RequestID: "init",
		Bots: &[]Bot{{SourceAdapter: "onebot", SourceProtocol: "onebot11", ID: "old-bot"}}, Config: map[string]any{"mode": "initial"},
		EffectivePermissions: []string{}, SuperAdmins: []string{}, CommandPrefixes: []string{"/"}, Concurrency: 4,
	})
	var frame protocolFrame
	decodeFrame(t, decoder, &frame)

	writeRuntimeEvent(t, encoder, "config-a", Event{
		EventID: "config-a", EventType: "config.changed",
		Payload: map[string]any{"config": map[string]any{"mode": "A"}, "changed_keys": []string{"mode"}},
	})
	writeRuntimeEvent(t, encoder, "config-b", Event{
		EventID: "config-b", EventType: "config.changed",
		Payload: map[string]any{"config": map[string]any{"mode": "B"}, "changed_keys": []string{"mode"}},
	})
	writeRuntimeEvent(t, encoder, "bot-new", Event{
		EventID: "bot-new", EventType: "bot.identities.changed",
		Payload: map[string]any{"bots": []Bot{{SourceAdapter: "onebot", SourceProtocol: "onebot11", ID: "new-bot"}}},
	})
	writeRuntimeEvent(t, encoder, "message", Event{EventID: "message", EventType: "message.group"})

	for range 4 {
		decodeFrame(t, decoder, &frame)
		if frame.Type != "result" {
			t.Fatalf("unexpected terminal frame: %#v", frame)
		}
	}

	var message observation
	for range 4 {
		item := <-observations
		if item.eventType == "message.group" {
			message = item
		}
	}
	if message.config["mode"] != "B" || message.bot.ID != "new-bot" {
		t.Fatalf("message observed stale control state: %#v", message)
	}
	if message.zone != "Asia/Shanghai" {
		t.Fatalf("host timezone = %q", message.zone)
	}

	writeFrame(t, encoder, protocolFrame{Type: "shutdown", RequestID: "shutdown", Reason: "stop"})
	if err := <-runDone; err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestRunRejectsConfigChangedWithoutSnapshotAndContinues(t *testing.T) {
	inputReader, inputWriter := io.Pipe()
	outputReader, outputWriter := io.Pipe()
	defer inputReader.Close()
	defer outputReader.Close()

	handled := make(chan string, 1)
	handler := HandlerFunc(func(_ context.Context, event *EventContext) error {
		handled <- event.Event.EventID
		return event.Result(map[string]any{})
	})
	runDone := make(chan error, 1)
	go func() {
		runDone <- Run(context.Background(), Options{Stdin: inputReader, Stdout: outputWriter}, handler)
	}()
	encoder := json.NewEncoder(inputWriter)
	decoder := json.NewDecoder(outputReader)
	writeFrame(t, encoder, protocolFrame{Bots: &[]Bot{},
		ProtocolVersion: ProtocolVersion, Type: "init", Timezone: "Asia/Shanghai", PluginID: "test-plugin", RequestID: "init",
		Config: map[string]any{"mode": "initial"}, EffectivePermissions: []string{},
		SuperAdmins: []string{}, CommandPrefixes: []string{"/"}, Concurrency: 1,
	})
	var frame protocolFrame
	decodeFrame(t, decoder, &frame)

	writeRuntimeEvent(t, encoder, "invalid-config", Event{
		EventID: "invalid-config", EventType: "config.changed",
		Payload: map[string]any{"changed_keys": []string{"mode"}},
	})
	decodeFrame(t, decoder, &frame)
	if frame.Type != "error" || frame.Code != "plugin.protocol_violation" {
		t.Fatalf("invalid config.changed response = %#v", frame)
	}

	writeRuntimeEvent(t, encoder, "message-after-error", Event{EventID: "message-after-error", EventType: "message.group"})
	decodeFrame(t, decoder, &frame)
	if frame.Type != "result" || <-handled != "message-after-error" {
		t.Fatalf("runtime did not continue after protocol violation: %#v", frame)
	}

	writeFrame(t, encoder, protocolFrame{Type: "shutdown", RequestID: "shutdown", Reason: "stop"})
	if err := <-runDone; err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestRunCorrelatesConcurrentLocalActionsAndSerializesTerminalFrames(t *testing.T) {
	inputReader, inputWriter := io.Pipe()
	outputReader, outputWriter := io.Pipe()
	defer inputReader.Close()
	defer outputReader.Close()

	results := make(chan string, 2)
	handler := HandlerFunc(func(ctx context.Context, event *EventContext) error {
		result, err := event.Actions().KVGet(ctx, event.Event.EventID)
		if err != nil {
			return err
		}
		value, _ := result["value"].(string)
		results <- event.Event.EventID + ":" + value
		return event.Result(map[string]any{"value": value})
	})

	runDone := make(chan error, 1)
	go func() {
		runDone <- Run(context.Background(), Options{
			Stdin: inputReader, Stdout: outputWriter, ActionTimeout: time.Second,
		}, handler)
	}()

	encoder := json.NewEncoder(inputWriter)
	decoder := json.NewDecoder(outputReader)
	writeFrame(t, encoder, protocolFrame{Bots: &[]Bot{},
		ProtocolVersion: ProtocolVersion, Type: "init", Timezone: "Asia/Shanghai", PluginID: "test-plugin", RequestID: "init-1",
		Config: map[string]any{"enabled": true}, EffectivePermissions: []string{},
		SuperAdmins: []string{}, CommandPrefixes: []string{"/"}, Concurrency: 2,
	})
	var initAck protocolFrame
	decodeFrame(t, decoder, &initAck)
	if initAck.Type != "init_ack" || initAck.Status != "ready" {
		t.Fatalf("unexpected init response: %#v", initAck)
	}
	writeEvent(t, encoder, "event-1", "alpha")
	writeEvent(t, encoder, "event-2", "beta")

	var actions [2]protocolFrame
	decodeFrame(t, decoder, &actions[0])
	decodeFrame(t, decoder, &actions[1])
	if actions[0].Type != "action" || actions[1].Type != "action" || actions[0].RequestID == actions[1].RequestID {
		t.Fatalf("unexpected action frames: %#v %#v", actions[0], actions[1])
	}
	for index := len(actions) - 1; index >= 0; index-- {
		value := "for-" + actions[index].ParentRequestID
		data, _ := json.Marshal(map[string]any{"value": value})
		writeFrame(t, encoder, protocolFrame{Type: "result", RequestID: actions[index].RequestID, Data: data})
	}

	var terminal [2]protocolFrame
	decodeFrame(t, decoder, &terminal[0])
	decodeFrame(t, decoder, &terminal[1])
	for _, frame := range terminal {
		if frame.Type != "result" || (frame.RequestID != "event-1" && frame.RequestID != "event-2") {
			t.Fatalf("unexpected terminal frame: %#v", frame)
		}
	}

	seen := map[string]bool{}
	seen[<-results] = true
	seen[<-results] = true
	if !seen["alpha:for-event-1"] || !seen["beta:for-event-2"] {
		t.Fatalf("local actions were mis-correlated: %#v", seen)
	}
	writeFrame(t, encoder, protocolFrame{Type: "shutdown", RequestID: "shutdown-1", Reason: "stop"})
	if err := <-runDone; err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestRunEnforcesOneTerminalResponseAndIsolatesPanics(t *testing.T) {
	inputReader, inputWriter := io.Pipe()
	outputReader, outputWriter := io.Pipe()
	defer inputReader.Close()
	defer outputReader.Close()

	var secondError error
	var mu sync.Mutex
	secondDone := make(chan struct{})
	handler := HandlerFunc(func(_ context.Context, event *EventContext) error {
		if event.Event.EventID == "panic" {
			panic("token=fixture-secret")
		}
		if err := event.Result(map[string]any{"ok": true}); err != nil {
			return err
		}
		mu.Lock()
		secondError = event.Fail("plugin.internal_error", "should not be emitted")
		mu.Unlock()
		close(secondDone)
		return nil
	})
	runDone := make(chan error, 1)
	go func() {
		runDone <- Run(context.Background(), Options{Stdin: inputReader, Stdout: outputWriter}, handler)
	}()
	encoder := json.NewEncoder(inputWriter)
	decoder := json.NewDecoder(outputReader)
	writeFrame(t, encoder, protocolFrame{Bots: &[]Bot{},
		ProtocolVersion: ProtocolVersion, Type: "init", Timezone: "Asia/Shanghai", PluginID: "test-plugin", RequestID: "init",
		Config: map[string]any{}, EffectivePermissions: []string{}, SuperAdmins: []string{}, CommandPrefixes: []string{"/"}, Concurrency: 1,
	})
	var frame protocolFrame
	decodeFrame(t, decoder, &frame)
	writeEvent(t, encoder, "first", "first")
	decodeFrame(t, decoder, &frame)
	if frame.Type != "result" {
		t.Fatalf("first terminal type = %q", frame.Type)
	}
	<-secondDone
	mu.Lock()
	err := secondError
	mu.Unlock()
	if err == nil || !strings.Contains(err.Error(), "already sent") {
		t.Fatalf("second terminal response error = %v", err)
	}

	writeEvent(t, encoder, "panic-request", "panic")
	decodeFrame(t, decoder, &frame)
	if frame.Type != "error" || frame.Code != "plugin.internal_error" || strings.Contains(frame.Message, "fixture-secret") {
		t.Fatalf("panic response leaked details or used wrong code: %#v", frame)
	}
	writeFrame(t, encoder, protocolFrame{Type: "shutdown", RequestID: "shutdown", Reason: "stop"})
	if err := <-runDone; err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func writeEvent(t *testing.T, encoder *json.Encoder, requestID, eventID string) {
	t.Helper()
	writeRuntimeEvent(t, encoder, requestID, Event{EventID: eventID, EventType: "message.group", Target: Target{Type: "group", ID: "100"}})
}

func writeRuntimeEvent(t *testing.T, encoder *json.Encoder, requestID string, event Event) {
	t.Helper()
	if event.EventType == "bot.identities.changed" {
		event.SourceProtocol = "platform"
		event.SourceAdapter = "adapters.internal"
	}
	if strings.HasPrefix(event.EventType, "message.") {
		if event.SourceProtocol == "" {
			event.SourceProtocol = "onebot11"
		}
		if event.SourceAdapter == "" {
			event.SourceAdapter = "onebot"
		}
	}
	if event.SourceProtocol == "" {
		event.SourceProtocol = "system"
	}
	if event.SourceAdapter == "" {
		event.SourceAdapter = "system"
	}
	raw := map[string]any{"event_id": event.EventID, "source_protocol": event.SourceProtocol, "source_adapter": event.SourceAdapter, "event_type": event.EventType, "timestamp": event.Timestamp}
	if event.Actor.ID != "" {
		raw["actor"] = event.Actor
	}
	if event.Target.ID != "" {
		raw["target"] = event.Target
	}
	if event.Message.PlainText != "" || len(event.Message.Segments) > 0 {
		raw["message"] = event.Message
	}
	if event.Payload != nil {
		raw["payload"] = event.Payload
	}
	if event.Webhook != nil {
		raw["webhook"] = event.Webhook
	}
	payload, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	writeFrame(t, encoder, protocolFrame{Type: "event", RequestID: requestID, Event: payload})
}

func writeFrame(t *testing.T, encoder *json.Encoder, frame protocolFrame) {
	t.Helper()
	if frame.Type == "init" {
		if frame.Config == nil {
			frame.Config = map[string]any{}
		}
		if frame.EffectivePermissions == nil {
			frame.EffectivePermissions = []string{}
		}
		if frame.SuperAdmins == nil {
			frame.SuperAdmins = []string{}
		}
		if frame.CommandPrefixes == nil {
			frame.CommandPrefixes = []string{"/"}
		}
		if frame.Concurrency == 0 {
			frame.Concurrency = 1
		}
	}
	if frame.Type == "result" && frame.Status == "" {
		frame.Status = "success"
	}
	if err := encoder.Encode(frame); err != nil {
		t.Fatalf("encode frame: %v", err)
	}
}

func decodeFrame(t *testing.T, decoder *json.Decoder, frame *protocolFrame) {
	t.Helper()
	if err := decoder.Decode(frame); err != nil {
		t.Fatalf("decode frame: %v", err)
	}
}
