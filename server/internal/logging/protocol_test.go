package logging

import "testing"

func TestNormalizeSummaryDerivesProtocolFromSource(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		source   string
		expected string
	}{
		{name: "adapter", source: "adapter", expected: ProtocolOneBot11},
		{name: "adapter.onebot11", source: "adapter.onebot11", expected: ProtocolOneBot11},
		{name: "bridge", source: "bridge", expected: ProtocolOneBot11},
		{name: "runtime", source: "runtime", expected: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			summary := NormalizeSummary(Summary{
				Source:  tc.source,
				Message: "test",
				Level:   "info",
			})
			if summary.Protocol != tc.expected {
				t.Fatalf("unexpected protocol: got %q want %q", summary.Protocol, tc.expected)
			}
		})
	}
}

func TestNormalizeSummaryCompactsRepeatedOneBotDetailFields(t *testing.T) {
	t.Parallel()

	summary := NormalizeSummary(Summary{
		Source:  "bridge",
		Message: "test",
		Level:   "info",
		Details: map[string]any{
			"event_timestamp": 1711015202,
			"time":            1711015202,
			"conversation_id": "2001",
			"group_id":        "2001",
			"message_id":      "1001",
			"real_id":         "1001",
			"message_seq":     "1001",
			"user_id":         "3001",
			"sender_id":       "3001",
			"sender_nickname": "Alice",
			"sender_role":     "admin",
		},
	})

	if _, ok := summary.Details["time"]; ok {
		t.Fatalf("time should be omitted: %#v", summary.Details)
	}
	if _, ok := summary.Details["group_id"]; ok {
		t.Fatalf("group_id should be omitted: %#v", summary.Details)
	}
	if _, ok := summary.Details["real_id"]; ok {
		t.Fatalf("real_id should be omitted: %#v", summary.Details)
	}
	if _, ok := summary.Details["message_seq"]; ok {
		t.Fatalf("message_seq should be omitted: %#v", summary.Details)
	}
	if _, ok := summary.Details["sender_id"]; ok {
		t.Fatalf("sender_id should be omitted: %#v", summary.Details)
	}
	if _, ok := summary.Details["sender_nickname"]; ok {
		t.Fatalf("sender_nickname should be omitted: %#v", summary.Details)
	}
	if _, ok := summary.Details["sender_role"]; ok {
		t.Fatalf("sender_role should be omitted: %#v", summary.Details)
	}
	if _, ok := summary.Details["user_id"]; ok {
		t.Fatalf("user_id should be omitted when sender.user_id is present: %#v", summary.Details)
	}

	sender, ok := summary.Details["sender"].(map[string]any)
	if !ok {
		t.Fatalf("expected sender map, got %#v", summary.Details["sender"])
	}
	if sender["user_id"] != "3001" || sender["nickname"] != "Alice" || sender["role"] != "admin" {
		t.Fatalf("unexpected sender details: %#v", sender)
	}
}

func TestNormalizeSummarySanitizesOneBotMessageAndDetails(t *testing.T) {
	t.Parallel()

	summary := NormalizeSummary(Summary{
		Source:  "bridge",
		Message: "1145141919: [终末地摸鱼群(553855023)][管理员]群星怒/没错，是魔法！(1358252269): 除了战猎这种抓不到加费就完全没法打的角色",
		Level:   "info",
		Details: map[string]any{
			"plain_text": "hello\u202eworld",
			"sender": map[string]any{
				"card": "群星怒\u2066~喵",
			},
		},
	})

	if summary.Message != "1145141919: [终末地摸鱼群(553855023)][管理员]群星怒/没错，是魔法！(1358252269): 除了战猎这种抓不到加费就完全没法打的角色" {
		t.Fatalf("unexpected sanitized summary message: %#v", summary.Message)
	}
	if got := summary.Details["plain_text"]; got != "helloworld" {
		t.Fatalf("unexpected sanitized plain_text detail: %#v", got)
	}
	sender := summary.Details["sender"].(map[string]any)
	if got := sender["card"]; got != "群星怒~喵" {
		t.Fatalf("unexpected sanitized sender card: %#v", got)
	}
}
