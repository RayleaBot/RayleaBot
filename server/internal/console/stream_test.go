package console

import (
	"testing"
	"time"
)

func TestStreamAppendSnapshotAndSubscribe(t *testing.T) {
	t.Parallel()

	stream := NewStream(2, 32)
	subscription, unsubscribe := stream.Subscribe("weather", 1)
	defer unsubscribe()

	stream.Append(Entry{PluginID: "weather", Stream: "stdout", Text: "first", Timestamp: time.Now()})
	stream.Append(Entry{PluginID: "weather", Stream: "stdout", Text: "second", Timestamp: time.Now()})
	stream.Append(Entry{PluginID: "weather", Stream: "stdout", Text: "third", Timestamp: time.Now()})

	snapshot := stream.Snapshot("weather")
	if len(snapshot) != 2 {
		t.Fatalf("snapshot size = %d, want 2", len(snapshot))
	}
	if snapshot[0].Text != "second" || snapshot[1].Text != "third" {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}

	select {
	case entry := <-subscription:
		if entry.PluginID != "weather" {
			t.Fatalf("plugin_id = %q, want weather", entry.PluginID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for stream subscriber")
	}

	if count := stream.SubscriberCount("weather"); count != 1 {
		t.Fatalf("subscriber count = %d, want 1", count)
	}
}
