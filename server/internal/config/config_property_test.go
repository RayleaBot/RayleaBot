package config

import (
	"strings"
	"testing"

	"pgregory.net/rapid"
)

func TestNormalizeOneBotWSURLCanonicalizesShorthandForms(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		scheme := rapid.SampledFrom([]string{"ws", "wss"}).Draw(t, "scheme")
		slashes := rapid.SampledFrom([]string{"", "/", "//"}).Draw(t, "slashes")
		target := rapid.StringMatching(`[A-Za-z0-9._:-]{1,48}`).Draw(t, "target")

		normalized, ok := normalizeOneBotWSURL(scheme + ":" + slashes + target)
		if !ok {
			t.Fatalf("expected shorthand %q to normalize", scheme+":"+slashes+target)
		}

		expected := scheme + "://" + strings.TrimLeft(target, "/")
		if normalized != expected {
			t.Fatalf("normalized = %q, want %q", normalized, expected)
		}
	})
}

func TestCanonicalizeDocumentAssignsSchemaVersionAndNormalizesOneBotWSURL(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		target := rapid.StringMatching(`[A-Za-z0-9._:-]{1,48}`).Draw(t, "target")
		raw := map[string]any{
			"onebot": map[string]any{
				"ws_url": "ws:" + target,
			},
		}

		document, err := canonicalizeDocument(raw)
		if err != nil {
			t.Fatalf("canonicalizeDocument() error = %v", err)
		}

		if got := strings.TrimSpace(stringValue(document["schema_version"])); got != CurrentSchemaVersion() {
			t.Fatalf("schema_version = %q, want %q", got, CurrentSchemaVersion())
		}
		if got := stringValue(section(document, "onebot")["ws_url"]); got != "ws://"+target {
			t.Fatalf("onebot.ws_url = %q, want %q", got, "ws://"+target)
		}
	})
}
