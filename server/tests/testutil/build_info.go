package testutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// WriteBuildInfo creates an explicit installed-release version for integration
// tests using the contract's valid build metadata fixture.
func WriteBuildInfo(t testing.TB, root, version string) {
	t.Helper()
	payload, err := os.ReadFile(RepoPath(t, "fixtures", "release-manifest", "ok.build-info-minimal.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Input map[string]any `json:"input"`
	}
	if err := json.Unmarshal(payload, &fixture); err != nil {
		t.Fatal(err)
	}
	fixture.Input["version"] = version
	payload, err = json.Marshal(fixture.Input)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "build_info.json"), payload, 0o600); err != nil {
		t.Fatal(err)
	}
}
