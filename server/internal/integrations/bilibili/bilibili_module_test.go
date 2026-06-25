package bilibili

import "testing"

func TestRuntimeEventProjectsBilibiliEvent(t *testing.T) {
	t.Parallel()

	event := RuntimeEvent(BilibiliEvent{
		UID:       "123456",
		ID:        "90001",
		EventType: "bilibili.dynamic.published",
		Kind:      "dynamic",
		Service:   "video",
		Title:     "新视频",
	}, 1780906200)

	if event.EventID != "bilibili.dynamic.published:123456:90001" {
		t.Fatalf("event id = %q", event.EventID)
	}
	if event.SourceProtocol != "bilibili" || event.SourceAdapter != "bilibili.source" {
		t.Fatalf("unexpected source identity: %#v", event)
	}
	if event.EventType != "bilibili.dynamic.published" || event.Timestamp != 1780906200 {
		t.Fatalf("unexpected event type or timestamp: %#v", event)
	}
	payload, ok := event.PayloadFields["bilibili"].(map[string]any)
	if !ok {
		t.Fatalf("missing bilibili payload: %#v", event.PayloadFields)
	}
	if payload["uid"] != "123456" || payload["id"] != "90001" || payload["service"] != "video" || payload["title"] != "新视频" {
		t.Fatalf("unexpected bilibili payload: %#v", payload)
	}
}
