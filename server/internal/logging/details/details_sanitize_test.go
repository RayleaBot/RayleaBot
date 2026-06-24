package details

import "testing"

func TestSanitizeMapDropsSensitiveLogDetailKeys(t *testing.T) {
	t.Parallel()

	sanitized := sanitizeMap(map[string]any{
		"access_token":  "token",
		"authorization": "Bearer token",
		"cookie":        "SESSDATA=fixture",
		"proxy_url":     "http://user:pass@example.test",
		"secret":        "secret",
		"nested": map[string]any{
			"api_token": "token",
			"safe":      "value",
		},
		"safe": "value",
	})

	for _, key := range []string{"access_token", "authorization", "cookie", "proxy_url", "secret"} {
		if _, ok := sanitized[key]; ok {
			t.Fatalf("sensitive key %q was not removed: %#v", key, sanitized)
		}
	}
	nested := sanitized["nested"].(map[string]any)
	if _, ok := nested["api_token"]; ok {
		t.Fatalf("nested sensitive key was not removed: %#v", nested)
	}
	if sanitized["safe"] != "value" || nested["safe"] != "value" {
		t.Fatalf("safe values were not preserved: %#v", sanitized)
	}
}
