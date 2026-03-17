package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"rayleabot/server/internal/config"
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
