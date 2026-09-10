package qqofficial

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestIntentMaskFoldsConfiguredNames(t *testing.T) {
	t.Parallel()

	// The mask below is the one the live gateway accepted at Identify.
	got := IntentMask([]string{"guilds", "guild_members", "group_and_c2c", "public_guild_messages"})
	want := (1 << 0) | (1 << 1) | (1 << 25) | (1 << 30)
	if got != want {
		t.Fatalf("IntentMask = %d, want %d", got, want)
	}
	if IntentMask(nil) != 0 {
		t.Fatalf("IntentMask(nil) = %d, want 0", IntentMask(nil))
	}
	// Duplicates must not double-count, since the mask is a set of bits.
	if IntentMask([]string{"guilds", "guilds"}) != 1 {
		t.Fatal("repeated intents changed the mask")
	}
}

func TestOpeningFramesCarryThePlatformTokenScheme(t *testing.T) {
	t.Parallel()

	identify, err := identifyPayload("abc", 1<<25)
	if err != nil {
		t.Fatalf("identifyPayload: %v", err)
	}
	var frame struct {
		Op int `json:"op"`
		D  struct {
			Token   string `json:"token"`
			Intents int    `json:"intents"`
			Shard   []int  `json:"shard"`
		} `json:"d"`
	}
	if err := json.Unmarshal(identify, &frame); err != nil {
		t.Fatalf("decode identify: %v", err)
	}
	if frame.Op != opIdentify || frame.D.Intents != 1<<25 {
		t.Fatalf("identify op/intents = %d/%d", frame.Op, frame.D.Intents)
	}
	// The gateway requires the "QQBot " prefix, not a bare token.
	if frame.D.Token != "QQBot abc" {
		t.Fatalf("identify token = %q, want the QQBot scheme", frame.D.Token)
	}
	if len(frame.D.Shard) != 2 {
		t.Fatalf("identify shard = %v, want a two element shard", frame.D.Shard)
	}

	resume, err := resumePayload("abc", "session-1", 42)
	if err != nil {
		t.Fatalf("resumePayload: %v", err)
	}
	var resumeFrame struct {
		Op int `json:"op"`
		D  struct {
			Token     string `json:"token"`
			SessionID string `json:"session_id"`
			Seq       int64  `json:"seq"`
		} `json:"d"`
	}
	if err := json.Unmarshal(resume, &resumeFrame); err != nil {
		t.Fatal(err)
	}
	if resumeFrame.Op != opResume || resumeFrame.D.SessionID != "session-1" || resumeFrame.D.Seq != 42 {
		t.Fatalf("resume frame = %+v", resumeFrame.D)
	}
}

func TestHeartbeatPayloadOmitsSequenceBeforeTheFirstEvent(t *testing.T) {
	t.Parallel()

	// Before any event the gateway expects a null sequence, not zero.
	first, _ := heartbeatPayload(0)
	var frame map[string]any
	if err := json.Unmarshal(first, &frame); err != nil {
		t.Fatal(err)
	}
	if frame["d"] != nil {
		t.Fatalf("heartbeat d = %v, want null before the first event", frame["d"])
	}
	later, _ := heartbeatPayload(7)
	if err := json.Unmarshal(later, &frame); err != nil {
		t.Fatal(err)
	}
	if frame["d"] != float64(7) {
		t.Fatalf("heartbeat d = %v, want the last sequence", frame["d"])
	}
}

func TestSessionResumesOnlyAfterReadyAndNotAfterRejection(t *testing.T) {
	t.Parallel()

	var s session
	if _, _, resumable := s.snapshot(); resumable {
		t.Fatal("a fresh session claimed to be resumable")
	}

	s.startSession("session-1", "bot-1", "bot", "")
	s.observeSeq(9)
	id, seq, resumable := s.snapshot()
	if !resumable || id != "session-1" || seq != 9 {
		t.Fatalf("session = %q/%d/%v, want a resumable session at seq 9", id, seq, resumable)
	}

	// A sequence only ever moves forward from real frames; heartbeat acks
	// carry none and must not reset it.
	s.observeSeq(0)
	if _, seq, _ := s.snapshot(); seq != 9 {
		t.Fatalf("sequence = %d after a frame without one, want 9", seq)
	}

	// After the gateway rejects the session, resuming it again would loop.
	s.invalidate()
	if _, _, resumable := s.snapshot(); resumable {
		t.Fatal("session stayed resumable after the gateway rejected it")
	}
}

func TestHeartbeatIntervalPrefersTheServerAssignedValue(t *testing.T) {
	t.Parallel()

	// The live gateway assigned 41250ms; it is per connection, not a constant.
	if got := heartbeatInterval(helloData{HeartbeatInterval: 41250}); got.Milliseconds() != 41250 {
		t.Fatalf("interval = %v, want the server assigned 41250ms", got)
	}
	if got := heartbeatInterval(helloData{}); got <= 0 {
		t.Fatalf("interval = %v, want a fallback when the gateway states none", got)
	}
}

func TestHandleDispatchStampsBotIdentityAndSkipsLifecycle(t *testing.T) {
	t.Parallel()

	client := &Client{logger: discardLogger()}
	var delivered int
	client.SetEventHandler(func(context.Context, chatevent.NormalizedEvent) { delivered++ })

	client.handleDispatch(context.Background(), gatewayFrame{
		Op: opDispatch, T: dispatchReady,
		D: json.RawMessage(`{"session_id":"s1","user":{"id":"bot-1","username":"bot"}}`),
	}, botProfile{})
	if delivered != 0 {
		t.Fatal("READY was delivered as an event")
	}
	if id, _ := client.BotIdentity(); id != "bot-1" {
		t.Fatalf("bot identity = %q, want bot-1 after READY", id)
	}

	client.handleDispatch(context.Background(), gatewayFrame{
		Op: opDispatch, T: dispatchC2CMessageCreate, ID: "evt-1",
		D: json.RawMessage(c2cTextDispatch),
	}, botProfile{})
	if delivered != 1 {
		t.Fatalf("delivered %d message events, want 1", delivered)
	}
}

func TestStatusReportsLifecycleTransitions(t *testing.T) {
	t.Parallel()

	client := &Client{logger: discardLogger()}
	// Before Start the adapter is idle, not "connected but unknown".
	if got := client.Status(); got.State != StateIdle {
		t.Fatalf("initial state = %q, want idle", got.State)
	}

	client.status.set(StateConnecting, "")
	if got := client.Status(); got.State != StateConnecting || got.Summary == "" {
		t.Fatalf("connecting status = %+v, want a connecting state with a summary", got)
	}

	client.session.startSession("s1", "bot-1", "洛箐箐", "")
	client.status.set(StateConnected, "")
	got := client.Status()
	if got.State != StateConnected || got.BotID != "bot-1" || got.BotName != "洛箐箐" {
		t.Fatalf("connected status = %+v, want the gateway identity", got)
	}
	if !strings.Contains(got.Summary, "洛箐箐") {
		t.Fatalf("summary = %q, want it to name the connected bot", got.Summary)
	}

	// A rejected credential is distinct from a dropped connection: reconnecting
	// will not fix it, and the operator needs to know that.
	client.status.set(StateAuthFailed, "app access token rejected")
	if got := client.Status(); got.State != StateAuthFailed || !strings.Contains(got.Summary, "鉴权") {
		t.Fatalf("auth failure status = %+v, want a distinct auth failure", got)
	}
}
