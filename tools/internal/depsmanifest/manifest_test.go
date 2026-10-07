package depsmanifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseAndRead(t *testing.T) {
	source := []byte(`{"manifest_version":5,"large":9007199254740993,"resources":[]}`)
	path := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(path, source, 0600); err != nil {
		t.Fatal(err)
	}
	m, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if m["large"] != json.Number("9007199254740993") {
		t.Fatal("lost integer precision")
	}
	for _, source := range []string{"null", "[]", "{} {}", "{broken}"} {
		if _, err := Parse([]byte(source)); err == nil {
			t.Fatalf("accepted %q", source)
		}
	}
}
func TestResourceIdentityAndSourceRules(t *testing.T) {
	for _, tc := range []struct {
		name    string
		change  func(map[string]any)
		invalid bool
	}{
		{"valid", func(map[string]any) {}, false},
		{"duplicate id", func(m map[string]any) {
			r := m["resources"].([]any)
			r = append(r, map[string]any{"id": "ffmpeg", "kind": "chromium", "platform": "linux", "sources": []any{}})
			m["resources"] = r
		}, true},
		{"duplicate platform kind", func(m map[string]any) {
			m["resources"] = append(m["resources"].([]any), map[string]any{"id": "other", "kind": "ffmpeg", "platform": "linux", "sources": []any{}})
		}, true},
		{"distinct platform", func(m map[string]any) {
			m["resources"] = append(m["resources"].([]any), map[string]any{"id": "other", "kind": "ffmpeg", "platform": "windows", "sources": []any{}})
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := map[string]any{"resources": []any{map[string]any{"id": "ffmpeg", "platform": "linux", "kind": "ffmpeg", "sources": []any{map[string]any{"url": "https://example.com/archive"}}}}}
			tc.change(m)
			errors := SemanticErrors(m)
			if (len(errors) > 0) != tc.invalid {
				t.Fatalf("errors=%v", errors)
			}
		})
	}
	for _, raw := range []string{"http://example.com/archive", "https:///archive", "https://user:pass@example.com/archive", "https://example.com/archive#fragment", "https://[broken/archive"} {
		m := map[string]any{"resources": []any{map[string]any{"sources": []any{map[string]any{"url": raw}}}}}
		errors := SemanticErrors(m)
		if len(errors) == 0 || !strings.Contains(errors[0], "/resources/0/sources/0/url") {
			t.Fatalf("URL %q: %v", raw, errors)
		}
	}
	source := map[string]any{"url": "https://example.com/archive"}
	resource := map[string]any{"sources": []any{source}, "ffprobe_archive": map[string]any{"sources": []any{source}}}
	errors := SemanticErrors(map[string]any{"resources": []any{resource}})
	if len(errors) != 1 || !strings.Contains(errors[0], "/ffprobe_archive/sources/0/url") {
		t.Fatalf("duplicate companion source: %v", errors)
	}
	if errors := SemanticErrors(map[string]any{"resources": []any{nil, map[string]any{"sources": []any{false, map[string]any{"url": 1}}}}}); len(errors) != 0 {
		t.Fatalf("schema-shape checks must be left to schema: %v", errors)
	}
}
