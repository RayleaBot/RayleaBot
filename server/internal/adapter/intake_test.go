package adapter

import (
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestClassifyFrameHeartbeat(t *testing.T) {
	t.Parallel()

	observedAt := time.Unix(1700000000, 0).UTC()
	frame := classifyFrame(
		websocket.MessageText,
		[]byte(`{"post_type":"meta_event","meta_event_type":"heartbeat","interval":5000}`),
		observedAt,
	)

	if frame.Summary.Category != FrameCategoryHeartbeat {
		t.Fatalf("unexpected category: got %s want %s", frame.Summary.Category, FrameCategoryHeartbeat)
	}
	if frame.Summary.Type != "meta.heartbeat" {
		t.Fatalf("unexpected type: got %q want %q", frame.Summary.Type, "meta.heartbeat")
	}
	if frame.Summary.HeartbeatInterval != 5*time.Second {
		t.Fatalf("unexpected heartbeat interval: got %s want %s", frame.Summary.HeartbeatInterval, 5*time.Second)
	}
	if !frame.Summary.ObservedAt.Equal(observedAt) {
		t.Fatalf("unexpected observedAt: got %s want %s", frame.Summary.ObservedAt, observedAt)
	}
}

func TestClassifyFrameBinaryHeartbeat(t *testing.T) {
	t.Parallel()

	observedAt := time.Unix(1700000000, 0).UTC()
	frame := classifyFrame(
		websocket.MessageBinary,
		[]byte(`{"post_type":"meta_event","meta_event_type":"heartbeat","interval":5000}`),
		observedAt,
	)

	if frame.Summary.Category != FrameCategoryHeartbeat {
		t.Fatalf("unexpected category: got %s want %s", frame.Summary.Category, FrameCategoryHeartbeat)
	}
	if frame.Summary.Type != "meta.heartbeat" {
		t.Fatalf("unexpected type: got %q want %q", frame.Summary.Type, "meta.heartbeat")
	}
	if frame.Summary.HeartbeatInterval != 5*time.Second {
		t.Fatalf("unexpected heartbeat interval: got %s want %s", frame.Summary.HeartbeatInterval, 5*time.Second)
	}
	if !frame.Summary.ObservedAt.Equal(observedAt) {
		t.Fatalf("unexpected observedAt: got %s want %s", frame.Summary.ObservedAt, observedAt)
	}
}

func TestClassifyFrameLifecycleReady(t *testing.T) {
	t.Parallel()

	frame := classifyFrame(
		websocket.MessageText,
		[]byte(`{"post_type":"meta_event","meta_event_type":"lifecycle","sub_type":"enable"}`),
		time.Unix(1700000000, 0).UTC(),
	)

	if frame.Summary.Category != FrameCategoryLifecycleReady {
		t.Fatalf("unexpected category: got %s want %s", frame.Summary.Category, FrameCategoryLifecycleReady)
	}
	if frame.Summary.Type != "meta.lifecycle.enable" {
		t.Fatalf("unexpected type: got %q want %q", frame.Summary.Type, "meta.lifecycle.enable")
	}
}

func TestClassifyFrameEvent(t *testing.T) {
	t.Parallel()

	frame := classifyFrame(
		websocket.MessageText,
		[]byte(`{"post_type":"message"}`),
		time.Unix(1700000000, 0).UTC(),
	)

	if frame.Summary.Category != FrameCategoryEvent {
		t.Fatalf("unexpected category: got %s want %s", frame.Summary.Category, FrameCategoryEvent)
	}
	if frame.Summary.Type != "message" {
		t.Fatalf("unexpected type: got %q want %q", frame.Summary.Type, "message")
	}
}

func TestClassifyFrameUnknown(t *testing.T) {
	t.Parallel()

	frame := classifyFrame(
		websocket.MessageText,
		[]byte(`{"status":"ok"}`),
		time.Unix(1700000000, 0).UTC(),
	)

	if frame.Summary.Category != FrameCategoryUnknown {
		t.Fatalf("unexpected category: got %s want %s", frame.Summary.Category, FrameCategoryUnknown)
	}
	if frame.Summary.Type != "unknown" {
		t.Fatalf("unexpected type: got %q want %q", frame.Summary.Type, "unknown")
	}
}

func TestClassifyFrameMalformedJSONIsInvalid(t *testing.T) {
	t.Parallel()

	frame := classifyFrame(
		websocket.MessageText,
		[]byte(`{"post_type":`),
		time.Unix(1700000000, 0).UTC(),
	)

	if frame.Summary.Category != FrameCategoryInvalid {
		t.Fatalf("unexpected category: got %s want %s", frame.Summary.Category, FrameCategoryInvalid)
	}
	if frame.Summary.Type != "invalid" {
		t.Fatalf("unexpected type: got %q want %q", frame.Summary.Type, "invalid")
	}
	if frame.InvalidSummary == "" {
		t.Fatal("expected invalid summary to be populated")
	}
}

func TestClassifyFrameUnsupportedMessageTypeIsInvalid(t *testing.T) {
	t.Parallel()

	frame := classifyFrame(
		websocket.MessageType(0),
		[]byte{0x01, 0x02},
		time.Unix(1700000000, 0).UTC(),
	)

	if frame.Summary.Category != FrameCategoryInvalid {
		t.Fatalf("unexpected category: got %s want %s", frame.Summary.Category, FrameCategoryInvalid)
	}
	if frame.InvalidSummary != "unexpected websocket message type" {
		t.Fatalf("unexpected invalid summary: got %q want %q", frame.InvalidSummary, "unexpected websocket message type")
	}
}

func TestApplyFrameSummaryUpdatesSnapshotObservability(t *testing.T) {
	t.Parallel()

	heartbeatAt := time.Unix(1700000000, 0).UTC()
	unknownAt := heartbeatAt.Add(5 * time.Second)
	snapshot := Snapshot{}

	applyFrameSummary(&snapshot, classifiedFrame{
		Summary: FrameSummary{
			Category:          FrameCategoryHeartbeat,
			Type:              "meta.heartbeat",
			ObservedAt:        heartbeatAt,
			HeartbeatInterval: 5 * time.Second,
		},
		Frame: oneBotFrame{SelfID: 10001},
	})
	applyFrameSummary(&snapshot, classifiedFrame{
		Summary: FrameSummary{
			Category:   FrameCategoryUnknown,
			Type:       "unknown",
			ObservedAt: unknownAt,
		},
	})
	applyFrameSummary(&snapshot, classifiedFrame{
		Summary: FrameSummary{
			Category:   FrameCategoryInvalid,
			Type:       "invalid",
			ObservedAt: unknownAt.Add(5 * time.Second),
		},
	})

	if snapshot.TotalReceivedFrames != 3 {
		t.Fatalf("unexpected total frame count: got %d want 3", snapshot.TotalReceivedFrames)
	}
	if snapshot.InvalidReceivedFrames != 1 {
		t.Fatalf("unexpected invalid frame count: got %d want 1", snapshot.InvalidReceivedFrames)
	}
	if !snapshot.HeartbeatSeen {
		t.Fatal("expected HeartbeatSeen to be true")
	}
	if snapshot.LastHeartbeatAt == nil || !snapshot.LastHeartbeatAt.Equal(heartbeatAt) {
		t.Fatalf("unexpected LastHeartbeatAt: got %v want %s", snapshot.LastHeartbeatAt, heartbeatAt)
	}
	if snapshot.LastFrameAt == nil || !snapshot.LastFrameAt.Equal(unknownAt) {
		t.Fatalf("unexpected LastFrameAt: got %v want %s", snapshot.LastFrameAt, unknownAt)
	}
	if snapshot.HeartbeatInterval != 5*time.Second {
		t.Fatalf("unexpected heartbeat interval: got %s want %s", snapshot.HeartbeatInterval, 5*time.Second)
	}
	if snapshot.BotID != "10001" {
		t.Fatalf("unexpected bot id: got %q want %q", snapshot.BotID, "10001")
	}
	if snapshot.LastFrameCategory != FrameCategoryInvalid {
		t.Fatalf("unexpected last frame category: got %s want %s", snapshot.LastFrameCategory, FrameCategoryInvalid)
	}
	if snapshot.LastFrameType != "invalid" {
		t.Fatalf("unexpected last frame type: got %q want %q", snapshot.LastFrameType, "invalid")
	}
}

func TestIsLifecycleDisable(t *testing.T) {
	t.Parallel()

	if !isLifecycleDisable(oneBotFrame{
		PostType:      "meta_event",
		MetaEventType: "lifecycle",
		SubType:       "disable",
	}) {
		t.Fatal("expected lifecycle disable to be recognized")
	}
}

func TestNormalizeSupportedEventFromTextMessage(t *testing.T) {
	t.Parallel()

	observedAt := time.Unix(1700000000, 0).UTC()
	event, ok := normalizeSupportedEvent(oneBotFrame{
		PostType:    "message",
		MessageType: "group",
		MessageID:   1001,
		Time:        observedAt.Unix(),
		SelfID:      10001,
		UserID:      3001,
		GroupID:     2001,
		RawMessage:  "hello adapter bridge",
	}, observedAt)
	if !ok {
		t.Fatal("expected supported text message event")
	}
	if event.Kind != EventKindMessage {
		t.Fatalf("unexpected kind: got %q want %q", event.Kind, EventKindMessage)
	}
	if event.BotID != "10001" {
		t.Fatalf("unexpected bot id: got %q want %q", event.BotID, "10001")
	}
	if event.EventType != "message.group" {
		t.Fatalf("unexpected event type: got %q want %q", event.EventType, "message.group")
	}
	if event.ConversationType != "group" || event.ConversationID != "2001" {
		t.Fatalf("unexpected conversation: %+v", event)
	}
	if event.SenderID != "3001" || event.PlainText != "hello adapter bridge" {
		t.Fatalf("unexpected normalized event: %+v", event)
	}
}

func TestNormalizeSupportedEventRejectsUnsupportedShape(t *testing.T) {
	t.Parallel()

	if _, ok := normalizeSupportedEvent(oneBotFrame{
		PostType:    "notice",
		MessageType: "group",
		SelfID:      10001,
		GroupID:     2001,
		UserID:      3001,
		RawMessage:  "should not pass",
	}, time.Unix(1700000000, 0).UTC()); ok {
		t.Fatal("notice event should not be normalized as supported bridge event")
	}

	if _, ok := normalizeSupportedEvent(oneBotFrame{
		PostType:    "message",
		MessageType: "group",
		SelfID:      10001,
		GroupID:     2001,
		UserID:      3001,
		RawMessage:  "   ",
	}, time.Unix(1700000000, 0).UTC()); ok {
		t.Fatal("empty text message should not be normalized as supported bridge event")
	}

	if _, ok := normalizeSupportedEvent(oneBotFrame{
		PostType:    "message",
		MessageType: "private",
		UserID:      3001,
		RawMessage:  "hello",
	}, time.Unix(1700000000, 0).UTC()); ok {
		t.Fatal("message without self_id should not be normalized as supported bridge event")
	}
}
