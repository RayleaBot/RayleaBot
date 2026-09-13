package config_test

import (
	"path/filepath"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/tests/testutil"
)

func TestConfigFixtures(t *testing.T) {
	t.Parallel()

	schemaPath := testutil.RepoPath(t, "contracts", "config.user.schema.json")
	testCases := []struct {
		name                string
		fixturePath         string
		expectValid         bool
		expectValidationErr bool
	}{
		{
			name:        "ok fixture",
			fixturePath: filepath.Join("..", "fixtures", "config", "ok.minimal.json"),
			expectValid: true,
		},
		{
			name:                "invalid fixture",
			fixturePath:         filepath.Join("..", "fixtures", "config", "invalid.onebot-url.json"),
			expectValid:         false,
			expectValidationErr: true,
		},
		{
			name:        "edge fixture",
			fixturePath: filepath.Join("..", "fixtures", "config", "edge.lan-listener.json"),
			expectValid: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fixture := testutil.LoadConfigFixture(t, tc.fixturePath)
			configPath := testutil.WriteYAMLConfig(t, fixture.Input)

			_, _, err := config.Load(configPath, schemaPath)
			if tc.expectValidationErr {
				if err == nil {
					t.Fatalf("expected config.Load to fail for %s", tc.fixturePath)
				}
				return
			}

			if err != nil {
				t.Fatalf("config.Load(%s) failed: %v", tc.fixturePath, err)
			}

		})
	}
}
