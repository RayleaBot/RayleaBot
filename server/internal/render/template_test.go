package render

import (
	"strings"
	"testing"
)

func TestSafeHTML(t *testing.T) {
	bundle, err := buildTemplateSourceBundle("test-safe-html", TemplateSource{
		ManifestJSON: map[string]any{
			"id":     "test-safe-html",
			"width":  100,
			"height": 100,
		},
		HTML:       `<div>{{ safeHTML .html }}</div>`,
		Stylesheet: ``,
	})
	if err != nil {
		t.Fatalf("build bundle: %v", err)
	}
	compiled, issues, err := compileTemplateBundle(bundle)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if len(issues) > 0 {
		t.Fatalf("validation issues: %v", issues)
	}

	html, err := compiled.renderHTML("default", map[string]any{"html": "<span class='topic'>#话题#</span>"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, "<span class='topic'>") {
		t.Errorf("expected raw HTML in output, got: %s", html)
	}
	if strings.Contains(html, "&lt;") {
		t.Errorf("expected unescaped HTML, got escaped output: %s", html)
	}
}

func TestToJSON(t *testing.T) {
	bundle, err := buildTemplateSourceBundle("test-to-json", TemplateSource{
		ManifestJSON: map[string]any{
			"id":     "test-to-json",
			"width":  100,
			"height": 100,
		},
		HTML:       `<script>var x = {{ toJSON .data }};</script>`,
		Stylesheet: ``,
	})
	if err != nil {
		t.Fatalf("build bundle: %v", err)
	}
	compiled, issues, err := compileTemplateBundle(bundle)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if len(issues) > 0 {
		t.Fatalf("validation issues: %v", issues)
	}

	html, err := compiled.renderHTML("default", map[string]any{"data": map[string]any{"key": "value"}})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, `{"key":"value"}`) {
		t.Errorf("expected JSON in output, got: %s", html)
	}
}
