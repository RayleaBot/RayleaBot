package subscriptions

import "testing"

func TestSubjectsFromSettingsFiltersAndMergesBilibiliSubscriptions(t *testing.T) {
	t.Parallel()

	subjects := SubjectsFromSettings(map[string]any{
		"subscriptions": []any{
			map[string]any{
				"enabled":    true,
				"platform":   "bilibili",
				"uid":        "123456",
				"name":       "测试 UP",
				"avatar_url": "https://i0.hdslb.com/bfs/face/up.jpg",
				"services":   []any{"video"},
			},
			map[string]any{
				"enabled":  true,
				"platform": "bilibili",
				"uid":      float64(123456),
				"services": []any{"live"},
			},
			map[string]any{
				"enabled":  true,
				"platform": "other",
				"uid":      "999999",
				"services": []any{"live"},
			},
			map[string]any{
				"enabled":  false,
				"platform": "bilibili",
				"uid":      "777777",
				"services": []any{"live"},
			},
			map[string]any{
				"enabled":  true,
				"platform": "bilibili",
				"uid":      "bad-id",
				"services": []any{"live"},
			},
		},
	})

	if len(subjects) != 1 {
		t.Fatalf("subjects = %#v, want exactly one merged subject", subjects)
	}
	subject := subjects["123456"]
	if subject.UID != "123456" || subject.Name != "测试 UP" || subject.AvatarURL == "" {
		t.Fatalf("unexpected subject identity: %#v", subject)
	}
	if !subject.Services["video"] || !subject.Services["live"] {
		t.Fatalf("expected merged services, got %#v", subject.Services)
	}
}

func TestSubjectsFromSettingsDefaultsEmptyServicesToAllSupportedKinds(t *testing.T) {
	t.Parallel()

	subjects := SubjectsFromSettings(map[string]any{
		"subscriptions": []map[string]any{
			{
				"platform": "bilibili",
				"uid":      "123456",
			},
		},
	})

	for _, service := range []string{"live", "video", "image_text", "article", "repost"} {
		if !subjects["123456"].Services[service] {
			t.Fatalf("expected default service %q in %#v", service, subjects["123456"].Services)
		}
	}
}
