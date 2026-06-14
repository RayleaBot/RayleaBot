package dynamic

import "testing"

func TestEventFromItemBuildsVideoEvent(t *testing.T) {
	watched := map[string]Subject{
		"123456": {
			UID:       "123456",
			Name:      "测试 UP",
			AvatarURL: "https://i0.hdslb.com/bfs/face/default.jpg",
			Services:  map[string]bool{"video": true},
		},
	}

	event, ok := EventFromItem(map[string]any{
		"id_str": "90001",
		"type":   "DYNAMIC_TYPE_AV",
		"basic":  map[string]any{"jump_url": "//www.bilibili.com/video/BV1RayleaBot"},
		"modules": map[string]any{
			"module_author": map[string]any{
				"mid":    "123456",
				"name":   "测试 UP",
				"face":   "//i0.hdslb.com/bfs/face/up.jpg",
				"pub_ts": float64(1780906200),
			},
			"module_dynamic": map[string]any{
				"major": map[string]any{
					"type": "MAJOR_TYPE_ARCHIVE",
					"archive": map[string]any{
						"title": "新视频标题",
						"desc":  "视频简介",
						"cover": "//i0.hdslb.com/bfs/archive/cover.jpg",
					},
				},
			},
		},
	}, watched)
	if !ok {
		t.Fatal("expected dynamic event")
	}
	if event.EventType != EventDynamicPublished || event.Service != "video" || event.Title != "新视频标题" {
		t.Fatalf("unexpected event: %#v", event)
	}
	if event.Author.Avatar != "https://i0.hdslb.com/bfs/face/up.jpg" {
		t.Fatalf("unexpected author avatar: %#v", event.Author)
	}
	if len(event.Images) != 1 || event.Images[0].URL != "https://i0.hdslb.com/bfs/archive/cover.jpg" {
		t.Fatalf("unexpected images: %#v", event.Images)
	}
}

func TestLatestMonitorCandidatePrefersNewestNormalOverPinnedHistory(t *testing.T) {
	event, ok, err := LatestMonitorCandidate([]MonitorCandidate{
		{Event: BilibiliEvent{ID: "100", PubTS: 10, Title: "pinned"}, Pinned: true, Index: 0},
		{Event: BilibiliEvent{ID: "101", PubTS: 20, Title: "normal"}, Index: 1},
	})
	if err != nil {
		t.Fatalf("LatestMonitorCandidate failed: %v", err)
	}
	if !ok || event.Title != "normal" {
		t.Fatalf("unexpected latest event: ok=%v event=%#v", ok, event)
	}
}
