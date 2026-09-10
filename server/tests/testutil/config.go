package testutil

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/deps"
)

type ConfigFixture struct {
	Input  json.RawMessage `json:"input"`
	Expect struct {
		Valid bool `json:"valid"`
	} `json:"expect"`
}

func LoadConfigFixture(t testing.TB, path string) ConfigFixture {
	t.Helper()

	bytes, err := ReadRepoPath(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}

	var fixture ConfigFixture
	if err := json.Unmarshal(bytes, &fixture); err != nil {
		t.Fatalf("unmarshal fixture %s: %v", path, err)
	}

	return fixture
}

func WriteYAMLConfig(t testing.TB, raw json.RawMessage) string {
	t.Helper()

	var input any
	if err := json.Unmarshal(raw, &input); err != nil {
		t.Fatalf("unmarshal fixture input: %v", err)
	}

	return WriteYAMLConfigMap(t, input)
}

func WriteYAMLConfigMap(t testing.TB, input any) string {
	t.Helper()

	yamlBytes, err := yaml.Marshal(input)
	if err != nil {
		t.Fatalf("marshal yaml: %v", err)
	}

	configPath := filepath.Join(t.TempDir(), "user.yaml")
	if err := os.WriteFile(configPath, yamlBytes, 0o644); err != nil {
		t.Fatalf("write temp config: %v", err)
	}

	return configPath
}

func NewPreparedTestRuntimeRoot(t testing.TB) string {
	t.Helper()

	root := t.TempDir()
	WriteTestDepsManifest(t, root)
	WriteTestRuntimeEntry(t, root, "chromium-test", "152.0.7977.42", "chrome-win64", "chrome.exe")
	WriteTestRuntimeEntry(t, root, "ffmpeg-test", "9.0.1", "bin", "ffmpeg")
	WriteTestRuntimeEntry(t, root, "ffmpeg-test", "9.0.1", "bin", "ffprobe")
	WriteTestTemplate(t, root, "help.menu", 640)
	WriteTestTemplate(t, root, "status.panel", 540)
	return root
}

func WriteTestDepsManifest(t testing.TB, root string) {
	t.Helper()

	manifestPath := filepath.Join(root, ".deps", "manifest.json")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatalf("mkdir deps manifest root: %v", err)
	}
	platform := deps.CurrentPlatform()
	manifest := `{
  "manifest_version": 5,
  "resources": [
    {
      "id": "chromium-test",
      "kind": "chromium",
      "version": "152.0.7977.42",
      "platform": "` + platform + `",
      "sources": [{"url": "https://example.invalid/chromium.zip", "kind": "upstream"}],
      "sha256": "5093f03a401b5579da490d281aba80b687d92fe6fdfec47ee522920918d6e327",
      "archive_format": "zip",
      "entrypoints": {"browser": ["chrome-win64/chrome.exe"]}
    },
    {
      "id": "ffmpeg-test",
      "kind": "ffmpeg",
      "version": "9.0.1",
      "platform": "` + platform + `",
      "sources": [{"url": "https://example.invalid/ffmpeg.zip", "kind": "upstream"}],
      "sha256": "10b7a95b928e551fc78cac665999e1ae1f08fb738b255adb0a8d3b9c2824a9c0",
      "archive_format": "zip",
      "entrypoints": {"ffmpeg": ["bin/ffmpeg"], "ffprobe": ["bin/ffprobe"]}
    }
  ]
}`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatalf("write deps manifest: %v", err)
	}
}

func WriteTestRuntimeEntry(t testing.TB, root, id, version string, segments ...string) {
	t.Helper()

	target := filepath.Join(append([]string{root, ".deps", "store", id, version}, segments...)...)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir runtime entry root: %v", err)
	}
	if err := os.WriteFile(target, []byte("ok"), 0o755); err != nil {
		t.Fatalf("write runtime entry: %v", err)
	}
}

func WriteTestTemplate(t testing.TB, root, id string, height int) {
	t.Helper()

	templateRoot := filepath.Join(root, "templates", id)
	if err := os.MkdirAll(templateRoot, 0o755); err != nil {
		t.Fatalf("mkdir test template root: %v", err)
	}
	files := map[string]string{
		"template.json": `{"name":"测试模板","id":"` + id + `","version":"1","entry_html":"template.html","stylesheet":"styles.css","input_schema":"input.schema.json","width":960,"height":` + fmt.Sprint(height) + `}`,
		"template.html": `<html><body>{{ .title }}</body></html>`,
		"styles.css":    `body { color: #111; }`,
		"input.schema.json": `{
  "type": "object",
  "additionalProperties": true
}`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(templateRoot, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write test template %s: %v", name, err)
		}
	}
}
