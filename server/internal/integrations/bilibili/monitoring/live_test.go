package monitoring

import (
	"strings"
	"testing"
	"time"

	bilibiliLive "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/live"
)

func TestLiveTransitionEventPayloadShape(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 8, 8, 10, 0, 0, time.UTC)
	event := LiveTransitionEvent(LiveTransitionInput{
		Subject: Subject{
			UID:  "123456",
			Name: "测试主播",
		},
		Item: bilibiliLive.StatusItem{
			Title:         "直播标题",
			URL:           "https://live.bilibili.com/98765",
			CoverFromUser: "//i0.hdslb.com/bfs/live/cover.jpg",
		},
		RoomID:        "98765",
		Name:          "测试主播",
		Face:          "https://i0.hdslb.com/bfs/face/default.jpg",
		LiveStartedAt: 1780906200,
		LiveStatus:    1,
		Now:           now,
	})

	if event.EventType != EventLiveStarted || !strings.HasPrefix(event.ID, "live-123456-98765-started-") {
		t.Fatalf("unexpected live event identity: %#v", event)
	}
	payload := Payload(event)
	if payload["kind"] != "live" || payload["uid"] != "123456" || payload["room_id"] != "98765" || payload["service"] != "live" {
		t.Fatalf("unexpected payload identity: %#v", payload)
	}
	if payload["title"] != "直播标题" || payload["url"] != "https://live.bilibili.com/98765" {
		t.Fatalf("unexpected payload content: %#v", payload)
	}
	if payload["live_status"] != 1 || payload["live_event"] != "started" || payload["status_label"] != "直播中" {
		t.Fatalf("unexpected live payload fields: %#v", payload)
	}
	images, ok := payload["images"].([]map[string]any)
	if !ok || len(images) != 1 || images[0]["url"] != "https://i0.hdslb.com/bfs/live/cover.jpg" {
		t.Fatalf("unexpected images payload: %#v", payload["images"])
	}
}
