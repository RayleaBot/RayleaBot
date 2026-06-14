package config

import (
	"path/filepath"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/testutil"
)

func TestConfigFixtures(t *testing.T) {
	t.Parallel()

	schemaPath := testutil.RepoPath(t, "contracts", "config.user.schema.json")
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

			fixture := testutil.LoadConfigFixture(t, tc.fixturePath)
			configPath := testutil.WriteYAMLConfig(t, fixture.Input)

			cfg, _, err := Load(configPath, schemaPath)
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
