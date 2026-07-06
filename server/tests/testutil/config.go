package testutil

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/RayleaBot/RayleaBot/server/internal/deps"
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
	WriteTestRuntimeEntry(t, root, "chromium-test", "147.0.7727.24", "chrome-win64", "chrome.exe")
	WriteTestRuntimeEntry(t, root, "python-test", "3.12.13", "python", "python.exe")
	WriteTestRuntimeEntry(t, root, "python-test", "3.12.13", "python", "Scripts", "pip.exe")
	WriteTestRuntimeEntry(t, root, "node-test", "24.14.0", "node-v24.14.0-win-x64", "node.exe")
	WriteTestRuntimeEntry(t, root, "node-test", "24.14.0", "node-v24.14.0-win-x64", "npm.cmd")
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
  "manifest_version": 3,
  "resources": [
    {
      "id": "chromium-test",
      "kind": "chromium",
      "version": "147.0.7727.24",
      "platform": "` + platform + `",
      "sources": [{"url": "https://example.invalid/chromium.zip", "kind": "upstream"}],
      "sha256": "22d9f6baf54f755ccf5843f8e6ad4ad6e0ba10d11092c574df9e8f97ce55369e",
      "archive_format": "zip",
      "entrypoints": {"browser": ["chrome-win64/chrome.exe"]}
    },
    {
      "id": "python-test",
      "kind": "python-runtime",
      "version": "3.12.13",
      "platform": "` + platform + `",
      "sources": [{"url": "https://example.invalid/python.tar.gz", "kind": "upstream"}],
      "sha256": "10b7a95b928e551fc78cac665999e1ae1f08fb738b255adb0a8d3b9c2824a9c0",
      "archive_format": "tar.gz",
      "entrypoints": {
        "python": ["python/python.exe"],
        "pip": ["python/Scripts/pip.exe"]
      }
    },
    {
      "id": "node-test",
      "kind": "nodejs-runtime",
      "version": "24.14.0",
      "platform": "` + platform + `",
      "sources": [{"url": "https://example.invalid/node.zip", "kind": "upstream"}],
      "sha256": "313fa40c0d7b18575821de8cb17483031fe07d95de5994f6f435f3b345f85c66",
      "archive_format": "zip",
      "entrypoints": {
        "node": ["node-v24.14.0-win-x64/node.exe"],
        "npm": ["node-v24.14.0-win-x64/npm.cmd"]
      }
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
		"template.json": `{"id":"` + id + `","version":"1","entry_html":"template.html","stylesheet":"styles.css","input_schema":"input.schema.json","width":960,"height":` + fmt.Sprint(height) + `}`,
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
