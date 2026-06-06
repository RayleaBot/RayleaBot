package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/deps"
)

type configFixture struct {
	Input  json.RawMessage `json:"input"`
	Expect struct {
		Valid bool `json:"valid"`
	} `json:"expect"`
}

func TestConfigFixtures(t *testing.T) {
	t.Parallel()

	schemaPath := filepath.Join("..", "contracts", "config.user.schema.json")
	testCases := []struct {
		name                string
		fixturePath         string
		expectValid         bool
		expectExposureMode  string
		expectValidationErr bool
	}{
		{
			name:               "ok fixture",
			fixturePath:        filepath.Join("..", "fixtures", "config", "ok.minimal.json"),
			expectValid:        true,
			expectExposureMode: "localhost_only",
		},
		{
			name:                "invalid fixture",
			fixturePath:         filepath.Join("..", "fixtures", "config", "invalid.onebot-url.json"),
			expectValid:         false,
			expectValidationErr: true,
		},
		{
			name:               "edge fixture",
			fixturePath:        filepath.Join("..", "fixtures", "config", "edge.public-via-reverse-proxy.json"),
			expectValid:        true,
			expectExposureMode: "public_via_reverse_proxy",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fixture := loadConfigFixture(t, tc.fixturePath)
			configPath := writeYAMLConfig(t, fixture.Input)

			cfg, _, err := config.Load(configPath, schemaPath)
			if tc.expectValidationErr {
				if err == nil {
					t.Fatalf("expected config.Load to fail for %s", tc.fixturePath)
				}
				return
			}

			if err != nil {
				t.Fatalf("config.Load(%s) failed: %v", tc.fixturePath, err)
			}

			if cfg.Web.ExposureMode != tc.expectExposureMode {
				t.Fatalf("unexpected exposure mode: got %q want %q", cfg.Web.ExposureMode, tc.expectExposureMode)
			}
		})
	}
}

func loadConfigFixture(t *testing.T, path string) configFixture {
	t.Helper()

	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}

	var fixture configFixture
	if err := json.Unmarshal(bytes, &fixture); err != nil {
		t.Fatalf("unmarshal fixture %s: %v", path, err)
	}

	return fixture
}

func writeYAMLConfig(t *testing.T, raw json.RawMessage) string {
	t.Helper()

	var input any
	if err := json.Unmarshal(raw, &input); err != nil {
		t.Fatalf("unmarshal fixture input: %v", err)
	}

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

func newPreparedTestRuntimeRoot(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	writeTestDepsManifest(t, root)
	writeTestRuntimeEntry(t, root, "chromium-test", "147.0.7727.24", "chrome-win64", "chrome.exe")
	writeTestRuntimeEntry(t, root, "python-test", "3.12.13", "python", "python.exe")
	writeTestRuntimeEntry(t, root, "python-test", "3.12.13", "python", "Scripts", "pip.exe")
	writeTestRuntimeEntry(t, root, "node-test", "24.14.0", "node-v24.14.0-win-x64", "node.exe")
	writeTestRuntimeEntry(t, root, "node-test", "24.14.0", "node-v24.14.0-win-x64", "npm.cmd")
	writeTestTemplate(t, root, "help.menu", 640)
	writeTestTemplate(t, root, "status.panel", 540)
	return root
}

func writeTestDepsManifest(t *testing.T, root string) {
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

func writeTestRuntimeEntry(t *testing.T, root, id, version string, segments ...string) {
	t.Helper()

	target := filepath.Join(append([]string{root, ".deps", "store", id, version}, segments...)...)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir runtime entry root: %v", err)
	}
	if err := os.WriteFile(target, []byte("ok"), 0o755); err != nil {
		t.Fatalf("write runtime entry: %v", err)
	}
}

func writeTestTemplate(t *testing.T, root, id string, height int) {
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
