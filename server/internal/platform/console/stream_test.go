package console

import (
	"slices"
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

func TestStreamHistoryPreservesLimitsAcrossEvictionAndGrowth(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name           string
		entries, bytes int
		input, want    []string
	}{
		{"entry limit", 3, 100, []string{"a", "b", "c", "d", "e", "f", "g"}, []string{"e", "f", "g"}},
		{"UTF8 byte limit", 8, 9, []string{"ab", "你好", "c", "d"}, []string{"你好", "c", "d"}},
		{"oversized entry", 3, 9, []string{"ab", "你好", "c", "longer-than-limit"}, []string{"longer-than-limit"}},
		{"after oversized entry", 3, 9, []string{"longer-than-limit", "x", "yz", "测试", "q"}, []string{"yz", "测试", "q"}},
		{"growth after byte eviction", 8, 16, []string{"aaa", "bbb", "ccc", "ddd", "123456789", "x", "y", "z"}, []string{"ddd", "123456789", "x", "y", "z"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stream := NewStream(tc.entries, tc.bytes)
			for _, text := range tc.input {
				stream.Append(Entry{PluginID: "fixture", Stream: "stdout", Text: text})
			}
			snapshot := stream.Snapshot("fixture")
			got := make([]string, len(snapshot))
			for index, entry := range snapshot {
				got[index] = entry.Text
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("history = %q, want %q", got, tc.want)
			}
			snapshot[0].Text = "changed by reader"
			if stream.Snapshot("fixture")[0].Text != tc.want[0] {
				t.Fatal("reader changed retained history")
			}
		})
	}
}
